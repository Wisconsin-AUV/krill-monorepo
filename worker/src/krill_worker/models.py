"""Keeps the SAM model the API asks for loaded, switching only between tasks."""

import logging
import os
import threading
from collections.abc import Callable
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
from typing import Generic, Protocol, TypeVar

from krill.v1 import worker_pb2

log = logging.getLogger(__name__)

DEFAULT_MODEL = "facebook/sam2.1-hiera-small"


class Model(Protocol):
    model_id: str


M = TypeVar("M", bound=Model)


class ModelLoadError(Exception):
    def __init__(self, model_id: str, cause: Exception) -> None:
        super().__init__(f"could not load {model_id}: {cause}")


def supports_exemplars(model_id: str) -> bool:
    """SAM 2.1 only segments what it is pointed at. SAM 3 also finds lookalikes."""
    return "sam3" in model_id


def hub_cache() -> Path:
    if path := os.environ.get("HF_HUB_CACHE"):
        return Path(path)
    home = os.environ.get("HF_HOME") or Path.home() / ".cache" / "huggingface"
    return Path(home) / "hub"


def cached_models(root: Path | None = None) -> list[str]:
    """SAM models with a downloaded snapshot in the Hugging Face cache."""
    out = []
    for repo in sorted((root or hub_cache()).glob("models--*--*")):
        _, org, name = repo.name.split("--", 2)
        snapshots = repo / "snapshots"
        if "sam" in name and snapshots.is_dir() and any(snapshots.iterdir()):
            out.append(f"{org}/{name}")
    return out


class Models(Generic[M]):
    """Loads models on demand and runs the API's warm-up and unload commands.

    A task holds the lock while it runs, so a command or a switch to another
    model waits for it to finish.
    """

    def __init__(
        self,
        load: Callable[[str], M],
        pinned: str = "",
        memory: Callable[[], tuple[int, int]] = lambda: (0, 0),
        release: Callable[[], None] = lambda: None,
    ) -> None:
        self._load = load
        self._memory = memory
        self._release = release
        self.pinned = pinned
        self.wanted = pinned or DEFAULT_MODEL
        self.model: M | None = None
        self.state: worker_pb2.ModelLoadState = worker_pb2.MODEL_LOAD_STATE_UNLOADED
        self.error = ""
        self.allocated = 0
        self.reserved = 0
        # The last command finished, and the last one handed to the thread.
        self.command_id = 0
        self._seen_command = 0
        self._loading = ""
        self._failed = ""
        self._lock = threading.Lock()
        self._commands = ThreadPoolExecutor(1, thread_name_prefix="model")

    @property
    def model_id(self) -> str:
        if self.model is not None:
            return self.model.model_id
        return self._loading or self.wanted

    def want(self, model_id: str) -> None:
        """Set the model to switch to between tasks, unless the worker pins its own."""
        if model_id and not self.pinned:
            self.wanted = model_id

    def command(self, cmd: worker_pb2.WorkerCommand) -> None:
        """Run cmd in the background unless it already ran, since the API resends it."""
        if cmd.id <= self._seen_command:
            return
        self._seen_command = cmd.id
        self._commands.submit(self._run_command, cmd)

    def _run_command(self, cmd: worker_pb2.WorkerCommand) -> None:
        try:
            if cmd.action == worker_pb2.WORKER_ACTION_WARM_UP:
                self.warm_up()
            elif cmd.action == worker_pb2.WORKER_ACTION_UNLOAD:
                with self._lock:
                    self._unload()
                log.info("unloaded the model")
        finally:
            self.command_id = cmd.id

    def warm_up(self) -> None:
        """Load the wanted model now. A failure shows as the error state."""
        try:
            with self._lock:
                self._ensure(self.wanted)
        except ModelLoadError:
            log.exception("warm up failed")

    def sync(self) -> None:
        """Switch to the wanted model if another one is loaded or the last load failed.

        Called between tasks. A worker that was unloaded stays unloaded until a
        task needs the model.
        """
        with self._lock:
            stale = self.model is not None and self.model.model_id != self.wanted
            retry = self.state == worker_pb2.MODEL_LOAD_STATE_ERROR and self._failed != self.wanted
            if not (stale or retry):
                return
            try:
                self._ensure(self.wanted)
            except ModelLoadError:
                log.exception("model switch failed")

    def run(self, task: Callable[[M], None]) -> None:
        """Run task with the loaded model, loading the wanted one if none is.

        The model is passed in rather than handed out so no caller keeps a
        reference that would stop an unload from freeing GPU memory.
        """
        with self._lock:
            task(self.model if self.model is not None else self._ensure(self.wanted))

    def _ensure(self, model_id: str) -> M:
        if self.model is not None and self.model.model_id == model_id:
            return self.model
        self._unload()
        self._loading = model_id
        self.error = ""
        self.state = worker_pb2.MODEL_LOAD_STATE_LOADING
        log.info("loading %s", model_id)
        try:
            model = self._load(model_id)
        except Exception as e:
            self._failed = model_id
            self.error = str(e)
            self.state = worker_pb2.MODEL_LOAD_STATE_ERROR
            self._loading = ""
            self._release()
            raise ModelLoadError(model_id, e) from e
        self.model = model
        self.allocated, self.reserved = self._memory()
        self.state = worker_pb2.MODEL_LOAD_STATE_READY
        self._loading = ""
        log.info("loaded %s", model_id)
        return model

    def _unload(self) -> None:
        if self.model is None and self.state != worker_pb2.MODEL_LOAD_STATE_ERROR:
            return
        self.model = None
        self.state, self.error = worker_pb2.MODEL_LOAD_STATE_UNLOADED, ""
        self.allocated = self.reserved = 0
        self._release()
