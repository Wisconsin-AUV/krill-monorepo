import logging

import pytest
from krill.v1 import log_pb2, worker_pb2

from krill_worker.api import ApiError
from krill_worker.heartbeat import Heartbeat
from krill_worker.logs import BATCH, LogBuffer
from krill_worker.models import Models


class FlakyApi:
    def __init__(self) -> None:
        self.up = False
        self.sent: list[worker_pb2.HeartbeatRequest] = []

    def heartbeat(self, req: worker_pb2.HeartbeatRequest) -> worker_pb2.HeartbeatResponse:
        if not self.up:
            raise ApiError("unavailable", "down")
        self.sent.append(req)
        return worker_pb2.HeartbeatResponse()


def logger(buf: LogBuffer) -> logging.Logger:
    log = logging.getLogger(f"test.{id(buf)}")
    log.propagate = False
    log.setLevel(logging.INFO)
    log.addHandler(buf)
    return log


def test_records_wait_for_the_api_and_stay_bounded() -> None:
    buf = LogBuffer(capacity=5)
    log = logger(buf)
    api = FlakyApi()
    hb = Heartbeat(api, Models(lambda _: None), "CPU", lambda: [], buf)  # type: ignore[arg-type]

    for i in range(8):
        log.info("record %d", i)
    with pytest.raises(ApiError):
        hb.beat()
    # The API was down, so nothing is lost but the oldest beyond capacity.
    assert [r.message for r in buf.pending()] == [f"record {i}" for i in range(3, 8)]

    api.up = True
    send = api.heartbeat

    def slow_send(req: worker_pb2.HeartbeatRequest) -> worker_pb2.HeartbeatResponse:
        log.warning("logged while sending")
        return send(req)

    api.heartbeat = slow_send  # type: ignore[method-assign]
    hb.beat()
    assert [r.seq for r in api.sent[0].logs] == [4, 5, 6, 7, 8]
    pending = buf.pending()
    assert [r.message for r in pending] == ["logged while sending"]
    assert pending[0].level == log_pb2.LOG_LEVEL_WARN


def test_heartbeat_sends_a_batch_at_a_time() -> None:
    buf = LogBuffer()
    log = logger(buf)
    for i in range(BATCH + 3):
        log.info("record %d", i)
    assert len(buf.pending()) == BATCH


def test_exceptions_are_kept() -> None:
    buf = LogBuffer()
    log = logger(buf)
    try:
        raise ValueError("boom")
    except ValueError:
        log.exception("task failed")
    (record,) = buf.pending()
    attrs = {a.key: a.value for a in record.attrs}
    assert record.level == log_pb2.LOG_LEVEL_ERROR
    assert "ValueError: boom" in attrs["exception"]
