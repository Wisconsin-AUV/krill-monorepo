from collections.abc import Iterable, Iterator
from pathlib import Path

import pytest
from krill.v1 import annotation_pb2, worker_pb2

from krill_worker.api import Api
from krill_worker.heartbeat import Heartbeat
from krill_worker.main import run_task
from krill_worker.models import ModelLoadError, Models, cached_models
from krill_worker.prelabelers import Frame
from krill_worker.track import Box, Embedding, Prompt

SMALL = "facebook/sam2.1-hiera-small"
SAM3 = "facebook/sam3"


class FakeModel:
    def __init__(self, model_id: str) -> None:
        self.model_id = model_id


class Loader:
    def __init__(self, broken: str = "") -> None:
        self.loads: list[str] = []
        self.releases = 0
        self.broken = broken

    def load(self, model_id: str) -> FakeModel:
        self.loads.append(model_id)
        if model_id == self.broken:
            raise OSError("gated repo")
        return FakeModel(model_id)

    def release(self) -> None:
        self.releases += 1

    def models(self, pinned: str = "") -> Models[FakeModel]:
        return Models(self.load, pinned, lambda: (1 << 30, 2 << 30), self.release)


def command(models: Models[FakeModel], id: int, action: worker_pb2.WorkerAction) -> None:
    models.command(worker_pb2.WorkerCommand(id=id, action=action))
    models._commands.submit(lambda: None).result()


def test_commands_run_once_though_the_api_resends_them() -> None:
    loader = Loader()
    models = loader.models()
    for _ in range(3):
        command(models, 5, worker_pb2.WORKER_ACTION_WARM_UP)
    assert loader.loads == [SMALL]
    assert models.state == worker_pb2.MODEL_LOAD_STATE_READY
    assert (models.allocated, models.reserved, models.command_id) == (1 << 30, 2 << 30, 5)

    command(models, 6, worker_pb2.WORKER_ACTION_UNLOAD)
    command(models, 6, worker_pb2.WORKER_ACTION_UNLOAD)
    assert models.model is None and loader.releases == 1
    assert models.state == worker_pb2.MODEL_LOAD_STATE_UNLOADED and models.allocated == 0

    # An unloaded worker stays unloaded between tasks and loads for the next one.
    models.sync()
    assert loader.loads == [SMALL]
    used: list[str] = []
    models.run(lambda m: used.append(m.model_id))
    assert used == [SMALL] and loader.loads == [SMALL, SMALL]


def test_switches_model_between_tasks_unless_pinned() -> None:
    loader = Loader()
    models = loader.models()
    models.warm_up()
    models.want(SAM3)
    # A task already handed out finishes on the model it started with.
    used: list[str] = []
    models.run(lambda m: used.append(m.model_id))
    assert used == [SMALL]
    models.sync()
    assert models.model_id == SAM3 and loader.loads == [SMALL, SAM3]

    pinned = Loader().models(pinned=SMALL)
    pinned.want(SAM3)
    assert pinned.wanted == SMALL


def test_load_failure_fails_the_task_and_retries_another_model() -> None:
    loader = Loader(broken=SAM3)
    models = loader.models()
    models.want(SAM3)
    with pytest.raises(ModelLoadError, match="gated repo"):
        models.run(lambda m: None)
    assert models.state == worker_pb2.MODEL_LOAD_STATE_ERROR and "gated repo" in models.error
    models.sync()
    assert loader.loads == [SAM3]

    models.want(SMALL)
    models.sync()
    assert models.state == worker_pb2.MODEL_LOAD_STATE_READY and models.error == ""


class CancellingApi(Api):
    """Answers heartbeats by cancelling task 7, and records reports."""

    def __init__(self) -> None:
        super().__init__("http://api.invalid", "token")
        self.failed: list[int] = []
        self.tracks: list[worker_pb2.ReportTrackRequest] = []

    def heartbeat(self, req: worker_pb2.HeartbeatRequest) -> worker_pb2.HeartbeatResponse:
        return worker_pb2.HeartbeatResponse(model=SAM3, cancel_task_id=7)

    def fail_task(self, task_id: int, error: str) -> None:
        self.failed.append(task_id)

    def report_track(self, req: worker_pb2.ReportTrackRequest) -> None:
        self.tracks.append(req)


class EndlessTracker:
    model_id = SMALL
    supports_exemplars = False

    def track(
        self, frames: Iterable[tuple[Frame, Embedding | None]], prompt: Prompt
    ) -> Iterator[Box | None]:
        while True:
            yield None

    def embed(self, frame: Frame) -> bytes:
        raise AssertionError("not called")

    def load_embedding(self, data: bytes) -> Embedding:
        raise AssertionError("not called")

    def find_similar(self, frame: Frame, box: Box) -> list[Box]:
        raise AssertionError("not called")


def test_cancelled_task_stops_without_reporting() -> None:
    api = CancellingApi()
    hb = Heartbeat(api, Loader().models(), "CPU", lambda: [])
    task = worker_pb2.TrackTask(
        id=7,
        frames=[worker_pb2.TaskFrame(id=i, url="http://frames.invalid") for i in range(3)],
        point=annotation_pb2.Point(x=0.5, y=0.5),
    )
    with hb.working(8):
        hb.beat()
        assert not hb.cancelled()
    with hb.working(7):
        hb.beat()
        assert hb.cancelled()
        run_task(api, EndlessTracker(), worker_pb2.NextTaskResponse(track=task), hb.cancelled)
    assert api.failed == [] and api.tracks == []
    assert hb.models.wanted == SAM3
    # The next task starts uncancelled.
    with hb.working(9):
        assert not hb.cancelled()


def test_cached_models_lists_downloaded_sam_snapshots(tmp_path: Path) -> None:
    (tmp_path / "models--facebook--sam3" / "snapshots" / "abc").mkdir(parents=True)
    (tmp_path / "models--facebook--sam2.1-hiera-tiny" / "snapshots").mkdir(parents=True)
    (tmp_path / "models--Ultralytics--YOLO11" / "snapshots" / "abc").mkdir(parents=True)
    assert cached_models(tmp_path) == [SAM3]
