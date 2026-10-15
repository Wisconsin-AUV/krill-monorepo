from pathlib import Path
from typing import cast

import cv2
import numpy as np
import pytest
from krill.v1 import pipeline_pb2, pipeline_worker_pb2, worker_pb2

from krill_worker.api import Api, ApiError
from krill_worker.pipeline import YoloModels, run_prelabel, run_preview
from krill_worker.prelabelers.params import to_value
from krill_worker.train import CheckIn, RunCancelledError


class FakeApi:
    def __init__(self, check_ins: list[worker_pb2.NextTaskResponse | ApiError] | None = None):
        self.prelabels: list[pipeline_worker_pb2.ReportPrelabelsRequest] = []
        self.previews: list[pipeline_worker_pb2.ReportPreviewRequest] = []
        self.check_ins = check_ins or []
        self.checked = 0

    def report_prelabels(self, req: pipeline_worker_pb2.ReportPrelabelsRequest) -> None:
        self.prelabels.append(req)

    def report_preview(self, req: pipeline_worker_pb2.ReportPreviewRequest) -> None:
        self.previews.append(req)

    def check_in(self, task_id: int) -> pipeline_worker_pb2.CheckInResponse:
        self.checked += 1
        if not self.check_ins:
            return pipeline_worker_pb2.CheckInResponse()
        next_ = self.check_ins.pop(0)
        if isinstance(next_, ApiError):
            raise next_
        return pipeline_worker_pb2.CheckInResponse(task=next_)


def pool_frame(tmp_path: Path, name: str, pipes: list[tuple[int, tuple[int, int, int]]]) -> str:
    frame = np.full((240, 320, 3), (130, 90, 20), np.uint8)  # BGR pool blue
    for x, bgr in pipes:
        cv2.rectangle(frame, (x, 40), (x + 10, 200), bgr, -1)
    path = tmp_path / name
    cv2.imwrite(str(path), frame)
    return path.as_uri()


def track_task(i: int) -> worker_pb2.NextTaskResponse:
    return worker_pb2.NextTaskResponse(track=worker_pb2.TrackTask(id=i))


def test_check_in_serves_everything_waiting_then_trains_on() -> None:
    api = FakeApi([track_task(1), track_task(2)])
    served: list[int] = []
    now = [0.0]
    check_in = CheckIn(cast(Api, api), 9, lambda r: served.append(r.track.id), clock=lambda: now[0])

    check_in()
    assert served == [1, 2]
    assert api.checked == 3

    # Between check-ins training goes on without asking the API.
    now[0] = 0.5
    check_in()
    assert api.checked == 3
    now[0] = 1.5
    check_in()
    assert api.checked == 4


def test_check_in_stops_training_once_the_run_is_gone() -> None:
    api = FakeApi(
        [ApiError("unavailable", "restarting"), ApiError("not_found", "task is not running")]
    )
    now = [0.0]
    check_in = CheckIn(cast(Api, api), 9, lambda r: None, clock=lambda: now[0])
    # A blip keeps training.
    check_in()
    now[0] = 2
    with pytest.raises(RunCancelledError):
        check_in()


def slalom_config(**params: float) -> pipeline_pb2.PluginConfig:
    return pipeline_pb2.PluginConfig(
        name="slalom", params=[to_value(k, v) for k, v in params.items()]
    )


def test_prelabel_reports_cv_boxes_by_plugin(tmp_path: Path) -> None:
    red, white = (25, 20, 200), (235, 235, 235)
    frames = [
        pipeline_pb2.PrelabelFrame(id=1, url=pool_frame(tmp_path, "a.png", [(100, red)])),
        pipeline_pb2.PrelabelFrame(
            id=2, url=pool_frame(tmp_path, "b.png", [(50, red), (200, white)])
        ),
        pipeline_pb2.PrelabelFrame(id=3, url=pool_frame(tmp_path, "c.png", [])),
    ]
    task = pipeline_pb2.PrelabelTask(
        id=7, frames=frames, cv=pipeline_pb2.CvPlugins(plugins=[slalom_config(blur=1)])
    )
    api = FakeApi()

    run_prelabel(cast(Api, api), task, YoloModels())

    assert api.prelabels[-1].done
    boxes = [b for r in api.prelabels for b in r.boxes]
    got = sorted((b.frame_id, b.label, b.plugin) for b in boxes)
    assert got == [(1, "red", "slalom"), (2, "red", "slalom"), (2, "white", "slalom")]
    first = next(b for b in boxes if b.frame_id == 1)
    # Reported top-left, like every other box.
    assert first.box.x == pytest.approx(100 / 320, abs=0.01)
    assert first.box.y == pytest.approx(40 / 240, abs=0.01)


def test_prelabel_fails_on_a_plugin_the_worker_lacks(tmp_path: Path) -> None:
    task = pipeline_pb2.PrelabelTask(
        id=7,
        frames=[pipeline_pb2.PrelabelFrame(id=1, url=pool_frame(tmp_path, "a.png", []))],
        cv=pipeline_pb2.CvPlugins(plugins=[pipeline_pb2.PluginConfig(name="gate")]),
    )
    with pytest.raises(ValueError, match="gate"):
        run_prelabel(cast(Api, FakeApi()), task, YoloModels())


def test_preview_uploads_the_mask(tmp_path: Path) -> None:
    task = pipeline_pb2.PreviewTask(
        id=3,
        frame=pipeline_pb2.PrelabelFrame(
            id=1, url=pool_frame(tmp_path, "a.png", [(100, (25, 20, 200))])
        ),
        plugin=slalom_config(min_area=0.5),
        mask_upload_url="http://minio/mask.png",
    )
    uploads: list[tuple[str, bytes, str]] = []
    api = FakeApi()

    run_preview(cast(Api, api), task, lambda url, data, kind: uploads.append((url, data, kind)))

    # The pipe is too small for these params, but the mask still shows it.
    assert api.previews[0].boxes == []
    ((url, data, kind),) = uploads
    assert (url, kind) == ("http://minio/mask.png", "image/png")
    mask = cv2.imdecode(np.frombuffer(data, np.uint8), cv2.IMREAD_GRAYSCALE)
    assert mask is not None
    assert mask[120, 105] == 255
    assert mask[120, 250] == 0
