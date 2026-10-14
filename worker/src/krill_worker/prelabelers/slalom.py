from dataclasses import dataclass, field

import cv2
import numpy as np

from krill_worker.prelabelers import Frame, Proposal
from krill_worker.prelabelers.cv import CvPreLabeler, blobs, proposal, threshold, to_hsv
from krill_worker.prelabelers.params import HsvRange, Mask, param


@dataclass(frozen=True)
class SlalomParams:
    red: HsvRange = field(default=HsvRange(170, 100, 70, 10, 255, 255), metadata=param("Red"))
    white: HsvRange = field(default=HsvRange(0, 0, 190, 179, 45, 255), metadata=param("White"))
    min_area: float = field(default=0.001, metadata=param("Minimum area", 0, 0.2, 0.0005))
    min_elongation: float = field(
        default=3.0, metadata=param("Minimum length to width", 1, 30, 0.1)
    )
    max_tilt: float = field(default=20.0, metadata=param("Maximum tilt", 0, 90, 1))
    min_fill: float = field(default=0.5, metadata=param("Minimum fill", 0, 1, 0.05))
    blur: int = field(default=5, metadata=param("Blur", 1, 31, 2))
    kernel: int = field(default=5, metadata=param("Cleanup kernel", 1, 31, 2))


class Slalom(CvPreLabeler[SlalomParams]):
    """Upright red and white pipes."""

    name = "slalom"
    title = "Slalom pipes"
    outputs = ("red", "white")
    Params = SlalomParams

    def masks(self, frame: Frame) -> dict[str, Mask]:
        p = self.params
        hsv = to_hsv(frame, p.blur)
        return {
            "red": threshold(hsv, p.red, p.kernel),
            "white": threshold(hsv, p.white, p.kernel),
        }

    def mask(self, frame: Frame) -> Mask:
        red, white = self.masks(frame).values()
        return np.asarray(cv2.bitwise_or(red, white), dtype=np.uint8)

    def __call__(self, frame: Frame) -> list[Proposal]:
        p = self.params
        out = []
        for label, mask in self.masks(frame).items():
            for b in blobs(mask, p.min_area):
                if b.elongation < p.min_elongation or b.tilt > p.max_tilt or b.fill < p.min_fill:
                    continue
                out.append(proposal(b, label, b.fill, frame.shape, oriented=False))
        return out
