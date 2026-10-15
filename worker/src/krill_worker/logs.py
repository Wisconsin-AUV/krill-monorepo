import logging
import threading
from collections import deque
from itertools import islice

from google.protobuf.timestamp_pb2 import Timestamp
from krill.v1 import log_pb2

# Records waiting for the API. The oldest are dropped while it's unreachable.
CAPACITY = 1000
# Records sent with one heartbeat, which keeps each request small.
BATCH = 100
# Longer messages and tracebacks are cut so a runaway log line can't bloat a
# heartbeat.
MAX_TEXT = 8 << 10


def _level(levelno: int) -> log_pb2.LogLevel:
    if levelno < logging.INFO:
        return log_pb2.LOG_LEVEL_DEBUG
    if levelno < logging.WARNING:
        return log_pb2.LOG_LEVEL_INFO
    if levelno < logging.ERROR:
        return log_pb2.LOG_LEVEL_WARN
    return log_pb2.LOG_LEVEL_ERROR


def _cut(text: str) -> str:
    return text if len(text) <= MAX_TEXT else text[:MAX_TEXT] + "…"


class LogBuffer(logging.Handler):
    """Keeps log records until a heartbeat delivers them to the API."""

    def __init__(self, capacity: int = CAPACITY) -> None:
        super().__init__()
        self._records: deque[log_pb2.LogRecord] = deque(maxlen=capacity)
        self._seq = 0
        self._guard = threading.Lock()
        self._formatter = logging.Formatter()

    def emit(self, record: logging.LogRecord) -> None:
        try:
            at = Timestamp()
            at.FromNanoseconds(int(record.created * 1e9))
            attrs = [log_pb2.LogAttr(key="logger", value=record.name)]
            if record.exc_info:
                exception = self._formatter.formatException(record.exc_info)
                attrs.append(log_pb2.LogAttr(key="exception", value=_cut(exception)))
            message = _cut(record.getMessage())
            with self._guard:
                self._seq += 1
                self._records.append(
                    log_pb2.LogRecord(
                        seq=self._seq,
                        time=at,
                        level=_level(record.levelno),
                        message=message,
                        attrs=attrs,
                    )
                )
        except Exception:
            self.handleError(record)

    def pending(self) -> list[log_pb2.LogRecord]:
        """The oldest records not yet delivered, up to BATCH."""
        with self._guard:
            return list(islice(self._records, BATCH))

    def delivered(self, seq: int) -> None:
        """Drop records up to seq, which the API now has."""
        with self._guard:
            while self._records and self._records[0].seq <= seq:
                self._records.popleft()
