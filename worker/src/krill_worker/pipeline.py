"""Pre-labeling unlabeled frames and previewing a CV pre-labeler on one."""

import logging
import os
import time
import urllib.request
from collections.abc import Callable, Iterable, Iterator
from pathlib import Path
from typing import TypeVar

import cv2
from krill.v1 import annotation_pb2, pipeline_pb2, pipeline_worker_pb2

from krill_worker.api import Api
from krill_worker.frames import decode_frame, fetch, prefetch, upload
from krill_worker.prelabelers import Proposal
from krill_worker.prelabelers.cv import CvPreLabeler
from krill_worker.prelabelers.plugins import PLUGINS
from krill_worker.prelabelers.yolo import YoloPreLabeler

log = logging.getLogger(__name__)

T = TypeVar("T")

REPORT_EVERY = 2.0
YOLO_BATCH = 16


def cache_dir() -> Path:
    base = os.environ.get("XDG_CACHE_HOME") or Path.home() / ".cache"
    path = Path(base) / "krill"
    path.mkdir(parents=True, exist_ok=True)
    return path


def download(url: str, path: Path) -> Path:
    """Stream url to path, which only appears once complete."""
    part = path.with_suffix(path.suffix + ".part")
    with urllib.request.urlopen(url, timeout=60) as res, part.open("wb") as f:
        while chunk := res.read(1 << 20):
            f.write(chunk)
    part.rename(path)
    return path


class YoloModels:
    """Keeps the pinned model loaded between clips, and only that one."""

    def __init__(self) -> None:
        self.version = ""
        self.labeler: YoloPreLabeler | None = None

    def get(self, m: pipeline_pb2.YoloModel) -> YoloPreLabeler:
        if self.labeler is None or self.version != m.version:
            path = cache_dir() / f"run-{m.version}.pt"
            if not path.exists():
                download(m.weights_url, path)
            self.labeler = YoloPreLabeler(path, m.min_confidence, m.image_size or 640)
            self.version = m.version
            log.info("loaded pre-label model from run %s", m.version)
        self.labeler.min_confidence = m.min_confidence
        return self.labeler


def batched(items: Iterable[T], n: int) -> Iterator[list[T]]:
    chunk: list[T] = []
    for item in items:
        chunk.append(item)
        if len(chunk) == n:
            yield chunk
            chunk = []
    if chunk:
        yield chunk


def to_box(p: Proposal) -> annotation_pb2.Box:
    cx, cy, w, h = p.box
    return annotation_pb2.Box(x=cx - w / 2, y=cy - h / 2, width=w, height=h)


def plugin(config: pipeline_pb2.PluginConfig) -> CvPreLabeler[object]:
    cls = PLUGINS.get(config.name)
    if cls is None:
        raise ValueError(f"this worker has no pre-labeler called {config.name}")
    return cls.from_values(config.params)


def run_prelabel(api: Api, task: pipeline_pb2.PrelabelTask, models: YoloModels) -> None:
    start = last_report = time.monotonic()
    batch: list[pipeline_worker_pb2.PrelabelBox] = []
    found = 0

    def report(boxes: list[pipeline_worker_pb2.PrelabelBox], done: bool = False) -> None:
        api.report_prelabels(
            pipeline_worker_pb2.ReportPrelabelsRequest(task_id=task.id, boxes=boxes, done=done)
        )

    def add(frame_id: int, proposals: list[Proposal], source: str = "") -> None:
        nonlocal found, batch, last_report
        for p in proposals:
            batch.append(
                pipeline_worker_pb2.PrelabelBox(
                    frame_id=frame_id,
                    label=p.label,
                    box=to_box(p),
                    confidence=p.confidence,
                    plugin=source,
                )
            )
            found += 1
        if time.monotonic() - last_report >= REPORT_EVERY:
            report(batch)
            batch = []
            last_report = time.monotonic()

    frames = prefetch(lambda f: (f.id, decode_frame(fetch(f.url))), task.frames)
    if task.WhichOneof("model") == "yolo":
        yolo = models.get(task.yolo)
        for chunk in batched(frames, YOLO_BATCH):
            ids = [fid for fid, _ in chunk]
            for fid, props in zip(ids, yolo.batch([f for _, f in chunk]), strict=True):
                add(fid, props)
    else:
        plugins = [(c.name, plugin(c)) for c in task.cv.plugins]
        for fid, frame in frames:
            for name, p in plugins:
                add(fid, p(frame), name)
    report(batch, done=True)
    log.info(
        "pre-labeled task %d: %d boxes on %d frames in %.1fs",
        task.id,
        found,
        len(task.frames),
        time.monotonic() - start,
    )


def run_preview(
    api: Api,
    task: pipeline_pb2.PreviewTask,
    put: Callable[[str, bytes, str], None] = upload,
) -> None:
    labeler = plugin(task.plugin)
    frame = decode_frame(fetch(task.frame.url))
    start = time.monotonic()
    proposals = labeler(frame)
    took = round((time.monotonic() - start) * 1000)
    ok, png = cv2.imencode(".png", labeler.mask(frame))
    if not ok:
        raise ValueError("could not encode the mask")
    put(task.mask_upload_url, png.tobytes(), "image/png")
    boxes = [
        pipeline_pb2.PreviewBox(
            output=p.label,
            box=to_box(p),
            confidence=p.confidence,
            corners=[annotation_pb2.Point(x=x, y=y) for x, y in p.corners or ()],
        )
        for p in proposals
    ]
    api.report_preview(
        pipeline_worker_pb2.ReportPreviewRequest(task_id=task.id, boxes=boxes, took_ms=took)
    )
