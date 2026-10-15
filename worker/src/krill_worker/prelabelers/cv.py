"""Shared pieces of the classic CV pre-labelers."""

import math
from abc import ABC, abstractmethod
from collections.abc import Iterable
from dataclasses import dataclass
from typing import Any, ClassVar, Generic, TypeVar

import cv2
import numpy as np
import numpy.typing as npt
from krill.v1 import pipeline_pb2

from krill_worker.prelabelers import Frame, Point, Proposal
from krill_worker.prelabelers.params import HsvRange, Mask, parse, specs

P = TypeVar("P")


class CvPreLabeler(ABC, Generic[P]):
    """A pre-labeler with typed params that thresholds pixels into a mask and
    turns blobs in it into proposals."""

    name: ClassVar[str]
    title: ClassVar[str]
    # The kinds of object it proposes, which the admin panel maps to labels.
    outputs: ClassVar[tuple[str, ...]]
    oriented: ClassVar[bool] = False
    Params: ClassVar[type[Any]]

    def __init__(self, params: P) -> None:
        self.params = params

    @classmethod
    def from_values(cls, values: Iterable[pipeline_pb2.ParamValue]) -> "CvPreLabeler[P]":
        return cls(parse(cls.Params, values))

    @classmethod
    def spec(cls) -> pipeline_pb2.PrelabelerSpec:
        return pipeline_pb2.PrelabelerSpec(
            name=cls.name,
            title=cls.title,
            outputs=cls.outputs,
            params=specs(cls.Params),
            oriented=cls.oriented,
        )

    @abstractmethod
    def mask(self, frame: Frame) -> Mask:
        """Pixels the thresholds keep, for tuning them by eye."""

    @abstractmethod
    def __call__(self, frame: Frame) -> list[Proposal]: ...


def to_hsv(frame: Frame, blur: int) -> npt.NDArray[np.uint8]:
    if blur > 1:
        k = blur | 1
        frame = np.asarray(cv2.GaussianBlur(frame, (k, k), 0), dtype=np.uint8)
    return np.asarray(cv2.cvtColor(frame, cv2.COLOR_RGB2HSV), dtype=np.uint8)


def threshold(hsv: npt.NDArray[np.uint8], color: HsvRange, kernel: int) -> Mask:
    """Keep pixels in color, then drop specks and fill pinholes."""
    mask = color.mask(hsv)
    if kernel > 1:
        k = cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (kernel, kernel))
        opened = cv2.morphologyEx(mask, cv2.MORPH_OPEN, k)
        mask = np.asarray(cv2.morphologyEx(opened, cv2.MORPH_CLOSE, k), dtype=np.uint8)
    return mask


@dataclass(frozen=True)
class Blob:
    """A connected region of a mask, with its tightest rotated rectangle."""

    area: float
    center: Point
    # Long side first, in pixels.
    length: float
    width: float
    # The long side's angle from the x axis toward y, from -pi/2 to pi/2.
    angle: float
    corners: npt.NDArray[np.float32]

    @property
    def elongation(self) -> float:
        return self.length / self.width if self.width > 0 else math.inf

    @property
    def fill(self) -> float:
        """How much of its rotated rectangle the blob covers, from 0 to 1."""
        rect = self.length * self.width
        return min(self.area / rect, 1.0) if rect > 0 else 0.0

    @property
    def tilt(self) -> float:
        """Degrees the long side leans away from vertical."""
        return 90 - abs(math.degrees(self.angle))


def blobs(mask: Mask, min_area: float) -> list[Blob]:
    """Blobs covering at least min_area of the frame."""
    h, w = mask.shape[:2]
    contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    out = []
    for c in contours:
        area = float(cv2.contourArea(c))
        if area < min_area * w * h:
            continue
        (cx, cy), (rw, rh), deg = cv2.minAreaRect(c)
        angle = math.radians(deg)
        if rw < rh:
            rw, rh = rh, rw
            angle += math.pi / 2
        # Normalize to (-pi/2, pi/2] since a rectangle has no front.
        angle = (angle + math.pi / 2) % math.pi - math.pi / 2
        if angle == -math.pi / 2:
            angle = math.pi / 2
        corners = cv2.boxPoints(((cx, cy), (rw, rh), math.degrees(angle)))
        out.append(Blob(area, (cx, cy), rw, rh, angle, np.asarray(corners, dtype=np.float32)))
    return out


def proposal(
    blob: Blob, label: str, confidence: float, shape: tuple[int, ...], oriented: bool
) -> Proposal:
    """A proposal boxing blob, normalized to a frame of shape."""
    h, w = shape[:2]
    xs = np.clip(blob.corners[:, 0], 0, w)
    ys = np.clip(blob.corners[:, 1], 0, h)
    x0, x1, y0, y1 = float(xs.min()), float(xs.max()), float(ys.min()), float(ys.max())
    box = ((x0 + x1) / 2 / w, (y0 + y1) / 2 / h, (x1 - x0) / w, (y1 - y0) / h)
    if not oriented:
        return Proposal(label, box, confidence)
    c = blob.corners
    corners = (
        (float(c[0, 0]) / w, float(c[0, 1]) / h),
        (float(c[1, 0]) / w, float(c[1, 1]) / h),
        (float(c[2, 0]) / w, float(c[2, 1]) / h),
        (float(c[3, 0]) / w, float(c[3, 1]) / h),
    )
    return Proposal(label, box, confidence, angle=blob.angle, corners=corners)
