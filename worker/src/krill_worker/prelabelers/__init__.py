from dataclasses import dataclass
from typing import Protocol

import numpy as np
import numpy.typing as npt

Frame = npt.NDArray[np.uint8]


@dataclass(frozen=True)
class Proposal:
    label: str
    # Normalized [0, 1] center-x, center-y, width, height which matches YOLO format
    box: tuple[float, float, float, float]
    confidence: float
    # Radians, only set for oriented boxes (YOLO-OBB)
    angle: float | None = None


class PreLabeler(Protocol):
    name: str

    def __call__(self, frame: Frame) -> list[Proposal]: ...
