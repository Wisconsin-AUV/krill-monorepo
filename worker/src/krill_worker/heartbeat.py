import logging
import socket
import threading
import time
import uuid
from collections.abc import Callable, Iterator
from contextlib import contextmanager
from typing import Any

from google.protobuf.timestamp_pb2 import Timestamp
from krill.v1 import system_pb2, worker_pb2

from krill_worker import __version__
from krill_worker.api import Api, ApiError
from krill_worker.models import Models, cached_models, supports_exemplars
from krill_worker.stats import host_stats

log = logging.getLogger(__name__)

# Matches the API's sample interval, so the compute page's charts are even.
INTERVAL = 5.0


class Heartbeat:
    """Tells the API what the worker runs and how busy it is, every INTERVAL.

    The API answers with the model to run, commands, and tasks to stop.
    """

    def __init__(
        self,
        api: Api,
        models: Models[Any],
        accelerator: str,
        gpus: Callable[[], list[system_pb2.GpuStats]],
    ) -> None:
        self.api = api
        self.models = models
        self.accelerator = accelerator
        self.gpus = gpus
        self.instance_id = uuid.uuid4().hex
        self.hostname = socket.gethostname()
        self.started_at = Timestamp()
        self.started_at.GetCurrentTime()
        self.task_id = 0
        self._cancelled = 0

    @contextmanager
    def working(self, task_id: int) -> Iterator[None]:
        self.task_id = task_id
        self._cancelled = 0
        try:
            yield
        finally:
            self.task_id = 0

    def cancelled(self) -> bool:
        """Whether the API stopped waiting on the running task."""
        return self.task_id != 0 and self._cancelled == self.task_id

    def request(self) -> worker_pb2.HeartbeatRequest:
        m = self.models
        return worker_pb2.HeartbeatRequest(
            instance_id=self.instance_id,
            hostname=self.hostname,
            version=__version__,
            model=m.model_id,
            accelerator=self.accelerator,
            started_at=self.started_at,
            host=host_stats(),
            gpus=self.gpus(),
            task_id=self.task_id,
            model_state=m.state,
            model_error=m.error,
            model_memory_allocated_bytes=m.allocated,
            model_memory_reserved_bytes=m.reserved,
            pinned_model=m.pinned,
            cached_models=cached_models(),
            command_id=m.command_id,
            supports_exemplars=supports_exemplars(m.model_id),
        )

    def beat(self) -> None:
        res = self.api.heartbeat(self.request())
        self.models.want(res.model)
        if res.HasField("command"):
            self.models.command(res.command)
        if res.cancel_task_id and res.cancel_task_id == self.task_id:
            if self._cancelled != res.cancel_task_id:
                log.info("the API stopped waiting on task %d", res.cancel_task_id)
            self._cancelled = res.cancel_task_id

    def start(self) -> None:
        threading.Thread(target=self._run, name="heartbeat", daemon=True).start()

    def _run(self) -> None:
        failing = False
        while True:
            try:
                self.beat()
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
