from dataclasses import dataclass
from typing import Protocol

import numpy as np
import numpy.typing as npt

Frame = npt.NDArray[np.uint8]

Point = tuple[float, float]


@dataclass(frozen=True)
class Proposal:
    label: str
    # Normalized [0, 1] center-x, center-y, width, height which matches YOLO format.
    # Axis-aligned, even for oriented objects, since that is what labels store.
    box: tuple[float, float, float, float]
    confidence: float
    # Radians, only set for oriented boxes (YOLO-OBB). The long axis's angle from
    # the image's x axis toward its y axis, in pixels, from -pi/2 to pi/2.
    angle: float | None = None
    # Normalized corners of the rotated box, for oriented boxes.
    corners: tuple[Point, Point, Point, Point] | None = None


class PreLabeler(Protocol):
    name: str

    def __call__(self, frame: Frame) -> list[Proposal]: ...
