from dataclasses import dataclass, field

from krill_worker.prelabelers import Frame, Proposal
from krill_worker.prelabelers.cv import CvPreLabeler, blobs, proposal, threshold, to_hsv
from krill_worker.prelabelers.params import HsvRange, Mask, param


@dataclass(frozen=True)
class PathMarkerParams:
    color: HsvRange = field(default=HsvRange(5, 110, 90, 25, 255, 255), metadata=param("Orange"))
    min_area: float = field(default=0.002, metadata=param("Minimum area", 0, 0.2, 0.0005))
    min_elongation: float = field(
        default=2.5, metadata=param("Minimum length to width", 1, 20, 0.1)
    )
    min_fill: float = field(default=0.6, metadata=param("Minimum fill", 0, 1, 0.05))
    blur: int = field(default=5, metadata=param("Blur", 1, 31, 2))
    kernel: int = field(default=5, metadata=param("Cleanup kernel", 1, 31, 2))


class PathMarker(CvPreLabeler[PathMarkerParams]):
    """Long orange rectangles on the pool floor that point the way."""

    name = "path_marker"
    title = "Path marker"
    outputs = ("marker",)
    oriented = True
    Params = PathMarkerParams

    def mask(self, frame: Frame) -> Mask:
        p = self.params
        return threshold(to_hsv(frame, p.blur), p.color, p.kernel)

    def __call__(self, frame: Frame) -> list[Proposal]:
        p = self.params
        out = []
        for b in blobs(self.mask(frame), p.min_area):
            if b.elongation < p.min_elongation or b.fill < p.min_fill:
                continue
            out.append(proposal(b, "marker", b.fill, frame.shape, oriented=True))
        return out
