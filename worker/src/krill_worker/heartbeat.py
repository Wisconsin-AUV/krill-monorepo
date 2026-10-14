import logging
import socket
import threading
import time
import uuid
from collections.abc import Callable, Iterator
from contextlib import contextmanager

from google.protobuf.timestamp_pb2 import Timestamp
from krill.v1 import system_pb2, worker_pb2

from krill_worker import __version__
from krill_worker.api import Api, ApiError
from krill_worker.stats import host_stats

log = logging.getLogger(__name__)

# Matches the API's sample interval, so the compute page's charts are even.
INTERVAL = 5.0


class Heartbeat:
    """Tells the API what the worker runs and how busy it is, every INTERVAL."""

    def __init__(
        self,
        api: Api,
        model: str,
        accelerator: str,
        gpus: Callable[[], list[system_pb2.GpuStats]],
    ) -> None:
        self.api = api
        self.model = model
        self.accelerator = accelerator
        self.gpus = gpus
        self.instance_id = uuid.uuid4().hex
        self.hostname = socket.gethostname()
        self.started_at = Timestamp()
        self.started_at.GetCurrentTime()
        self.task_id = 0

    @contextmanager
    def working(self, task_id: int) -> Iterator[None]:
        self.task_id = task_id
        try:
            yield
        finally:
            self.task_id = 0

    def request(self) -> worker_pb2.HeartbeatRequest:
        return worker_pb2.HeartbeatRequest(
            instance_id=self.instance_id,
            hostname=self.hostname,
            version=__version__,
            model=self.model,
            accelerator=self.accelerator,
            started_at=self.started_at,
            host=host_stats(),
            gpus=self.gpus(),
            task_id=self.task_id,
        )

    def start(self) -> None:
        threading.Thread(target=self._run, name="heartbeat", daemon=True).start()

    def _run(self) -> None:
        failing = False
        while True:
            try:
                self.api.heartbeat(self.request())
                if failing:
                    log.info("heartbeat reaching the API again")
                failing = False
            except (ApiError, OSError) as e:
                # Polling already logs when the API is down, so say it once.
                if not failing:
                    log.warning("heartbeat failed: %s", e)
                failing = True
            except Exception:
                log.exception("heartbeat failed")
            time.sleep(INTERVAL)
