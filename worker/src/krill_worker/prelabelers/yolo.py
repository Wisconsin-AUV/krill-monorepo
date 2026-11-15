from collections.abc import Sequence
from pathlib import Path
from typing import Any

import numpy as np

from krill_worker.geometry import bounds, ordered
from krill_worker.prelabelers import Frame, Proposal


class YoloPreLabeler:
    """Proposes boxes with a model Krill trained."""

    name = "yolo"

    def __init__(self, weights: Path, min_confidence: float, image_size: int) -> None:
        # Imported here so the worker runs without Ultralytics installed.
        from ultralytics import YOLO

        self.model: Any = YOLO(str(weights))
        self.min_confidence = min_confidence
        self.image_size = image_size

    def __call__(self, frame: Frame) -> list[Proposal]:
        return self.batch([frame])[0]

    def batch(self, frames: Sequence[Frame]) -> list[list[Proposal]]:
        # Ultralytics reads arrays as BGR.
        bgr = [np.ascontiguousarray(f[..., ::-1]) for f in frames]
        results = self.model.predict(
            bgr, conf=self.min_confidence, imgsz=self.image_size, verbose=False
        )
        read = obb if self.model.task == "obb" else boxes
        return [read(r) for r in results]


def boxes(r: Any) -> list[Proposal]:
    xywhn = r.boxes.xywhn.tolist()
    conf = r.boxes.conf.tolist()
    cls = r.boxes.cls.tolist()
    return [
        Proposal(r.names[int(c)], (b[0], b[1], b[2], b[3]), float(p))
        for b, p, c in zip(xywhn, conf, cls, strict=True)
    ]


def obb(r: Any) -> list[Proposal]:
    """Proposals from an OBB model, whose axis-aligned box is its corners' bounds."""
    corners = r.obb.xyxyxyxyn.tolist()
    conf = r.obb.conf.tolist()
    cls = r.obb.cls.tolist()
    out = []
    for pts, p, c in zip(corners, conf, cls, strict=True):
        rect = ordered(pts)
        x, y, w, h = bounds(rect)
        box = (x + w / 2, y + h / 2, w, h)
        out.append(Proposal(r.names[int(c)], box, float(p), corners=rect))
    return out
