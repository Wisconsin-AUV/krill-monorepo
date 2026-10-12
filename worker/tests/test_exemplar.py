from collections.abc import Iterable, Iterator

from krill.v1 import annotation_pb2, worker_pb2

from krill_worker.api import Api
from krill_worker.main import run_task
from krill_worker.prelabelers import Frame
from krill_worker.track import Box, Prompt


class FakeApi(Api):
    def __init__(self) -> None:
        super().__init__("http://api.invalid", "token")
        self.failed: list[tuple[int, str]] = []

    def fail_task(self, task_id: int, error: str) -> None:
        self.failed.append((task_id, error))


class Sam2Tracker:
    model_id = "facebook/sam2.1-hiera-small"
    supports_exemplars = False

    def track(self, frames: Iterable[Frame], prompt: Prompt) -> Iterator[Box | None]:
        raise AssertionError("not called")

    def find_similar(self, frame: Frame, box: Box) -> list[Box]:
        raise AssertionError("not called")


def test_exemplar_task_fails_clearly_without_sam3() -> None:
    api = FakeApi()
    task = worker_pb2.ExemplarTask(
        id=7,
        # Fetching this would raise a different error, so the check must come first.
        frame=worker_pb2.TaskFrame(id=1, url="http://frames.invalid/1.jpg"),
        box=annotation_pb2.Box(x=0.1, y=0.1, width=0.2, height=0.2),
    )
    run_task(api, Sam2Tracker(), worker_pb2.NextTaskResponse(exemplar=task))

    assert len(api.failed) == 1
    task_id, error = api.failed[0]
    assert task_id == 7
    assert "SAM 3" in error
    assert "facebook/sam2.1-hiera-small" in error
