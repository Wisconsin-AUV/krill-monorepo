import math
from collections.abc import Iterable, Iterator, Sequence
from pathlib import Path
from typing import cast

import cv2
import numpy as np
import pytest
from krill.v1 import annotation_pb2, worker_pb2

from krill_worker.api import Api
from krill_worker.frames import decode_frame
from krill_worker.geometry import Corners, bounds, ordered, rotated_rect
from krill_worker.main import run_orient, run_track
from krill_worker.orient import fit, load_reference
from krill_worker.prelabelers import Frame
from krill_worker.track import Box, BoxPrompt, Embedding, Mask, Prompt

FIXTURES = Path(__file__).parent / "fixtures"
H, W = 480, 640

# The prototype's corners for the two markers in fixtures/markers.jpg, found
# on the full 1920 by 1200 frame.
SURVEY_REPAIR = [(0.2958, 0.245), (0.3663, 0.3046), (0.315, 0.4104), (0.2453, 0.3469)]
SEARCH_RESCUE = [(0.5325, 0.2712), (0.6111, 0.2891), (0.6059, 0.4006), (0.5258, 0.3829)]


def angle(corners: Corners, shape: tuple[int, ...]) -> float:
    """Degrees the first side leans from the x axis, modulo a quarter turn."""
    h, w = shape[:2]
    (x0, y0), (x1, y1) = corners[0], corners[1]
    return math.degrees(math.atan2((y1 - y0) * h, (x1 - x0) * w)) % 90


def pixel_error(got: Corners, want: Corners, shape: tuple[int, ...]) -> float:
    h, w = shape[:2]
    return float(np.abs((np.array(got) - np.array(want)) * [w, h]).max())


def floor() -> Frame:
    frame = np.full((H, W, 3), 90, np.uint8)
    noise = np.random.default_rng(0).integers(-8, 9, frame.shape)
    return np.asarray(np.clip(frame.astype(int) + noise, 0, 255), dtype=np.uint8)


def rect_mask(center: tuple[float, float], size: tuple[float, float], deg: float) -> Mask:
    mask = np.zeros((H, W), np.uint8)
    cv2.fillPoly(mask, [cv2.boxPoints((center, size, deg)).astype(np.int32)], 1)
    return mask.astype(bool)


def markers() -> tuple[Frame, Path]:
    path = FIXTURES / "markers.jpg"
    return decode_frame(path.read_bytes()), path


def test_ordered_is_clockwise_from_the_top_left() -> None:
    clockwise = [(0.2, 0.1), (0.6, 0.3), (0.5, 0.5), (0.1, 0.3)]
    want = tuple(clockwise)
    assert ordered(clockwise[::-1]) == want
    assert ordered(clockwise[2:] + clockwise[:2]) == want
    # A diamond's top and left corners tie on x + y, so the top one starts.
    diamond = [(0.5, 0.6), (0.4, 0.5), (0.5, 0.4), (0.6, 0.5)]
    assert ordered(diamond) == ((0.5, 0.4), (0.6, 0.5), (0.5, 0.6), (0.4, 0.5))


def test_bounds_clip_corners_outside_the_frame() -> None:
    corners = ordered([(-0.1, 0.2), (0.3, -0.05), (0.4, 0.3), (0.0, 0.55)])
    assert bounds(corners) == pytest.approx((0, 0, 0.4, 0.55))


def test_fit_from_a_mask() -> None:
    want = rotated_rect(cv2.boxPoints(((320, 240), (200, 80), 30)), (H, W))
    corners = fit(floor(), bounds(want), rect_mask((320, 240), (200, 80), 30))
    assert corners is not None
    assert pixel_error(corners, want, (H, W)) < 1.5


def test_fit_finds_the_paper_without_a_mask() -> None:
    frame = floor()
    pts = cv2.boxPoints(((300, 220), (120, 120), 20))
    cv2.fillPoly(frame, [pts.astype(np.int32)], (235, 235, 235))
    want = rotated_rect(pts, (H, W))
    # Drawn a little tight, as boxes often are.
    x, y, w, h = bounds(want)
    corners = fit(frame, (x + 0.01, y + 0.01, w - 0.02, h - 0.02))
    assert corners is not None
    assert pixel_error(corners, want, (H, W)) < 2


def test_fit_rejects_a_box_on_bare_floor() -> None:
    frame = floor()
    cv2.fillPoly(
        frame, [cv2.boxPoints(((150, 150), (100, 100), 20)).astype(np.int32)], (235, 235, 235)
    )
    assert fit(frame, (0.6, 0.55, 0.15, 0.2)) is None


def test_fit_aligns_the_reference_past_glare_on_the_sleeve() -> None:
    reference = load_reference((FIXTURES / "surveyrepair.png").read_bytes())
    design = decode_frame((FIXTURES / "surveyrepair.png").read_bytes())
    # The design's cut line isn't printed.
    cv2.rectangle(design, (0, 0), (design.shape[1] - 1, design.shape[0] - 1), (255, 255, 255), 3)
    dh, dw = design.shape[:2]
    # A print seen at an angle, so its outline is not a rectangle.
    printed = np.array([[300, 130], [400, 205], [335, 315], [232, 245]], np.float32)
    homography = cv2.getPerspectiveTransform(
        np.array([[0, 0], [dw, 0], [dw, dh], [0, dh]], np.float32), printed
    )
    frame = floor()
    # Glare on the plastic sleeve past one corner is as bright as the paper.
    glare = np.array([[335, 315], [355, 300], [350, 335]], np.int32)
    cv2.fillPoly(frame, [glare], (240, 240, 240))
    cv2.warpPerspective(design, homography, (W, H), frame, borderMode=cv2.BORDER_TRANSPARENT)
    want = rotated_rect(printed, (H, W))

    outline = fit(frame, bounds(want))
    corners = fit(frame, bounds(want), reference=reference)

    assert outline is not None and corners is not None
    assert pixel_error(outline, want, (H, W)) > 5
    # The design's blank margins leave ECC a pixel or two of play in scale.
    assert pixel_error(corners, want, (H, W)) < 3


@pytest.mark.parametrize(
    "box, design, prototype, deg",
    [
        ((0.2412, 0.2453, 0.1279, 0.175), "surveyrepair.png", SURVEY_REPAIR, 38),
        ((0.5215, 0.2719, 0.0918, 0.1453), "searchrescue.png", SEARCH_RESCUE, 8),
    ],
)
def test_fit_real_markers_with_their_reference(
    box: Box, design: str, prototype: list[tuple[float, float]], deg: float
) -> None:
    frame, _ = markers()
    reference = load_reference((FIXTURES / design).read_bytes())

    corners = fit(frame, box, reference=reference)

    assert corners is not None
    fh, fw = frame.shape[:2]
    want = rotated_rect(np.array(prototype) * [fw, fh], frame.shape)
    assert pixel_error(corners, want, frame.shape) < 2.5
    assert angle(corners, frame.shape) == pytest.approx(deg, abs=1)


class FakeApi:
    def __init__(self) -> None:
        self.orients: list[worker_pb2.ReportOrientRequest] = []
        self.tracks: list[worker_pb2.ReportTrackRequest] = []

    def report_orient(self, req: worker_pb2.ReportOrientRequest) -> None:
        self.orients.append(req)

    def report_track(self, req: worker_pb2.ReportTrackRequest) -> None:
        self.tracks.append(req)


class MaskTracker:
    """Finds a rectangle turned a frame's brightness in degrees, so tests can
    tell which frame a mask was for."""

    model_id = "facebook/sam2.1-hiera-small"
    supports_exemplars = False

    def mask(self, frame: Frame) -> Mask:
        return rect_mask((W / 2, H / 2), (200, 80), int(frame.mean()))

    def track(
        self, frames: Iterable[tuple[Frame, Embedding | None]], prompt: Prompt
    ) -> Iterator[Mask | None]:
        for frame, _ in frames:
            yield self.mask(frame)

    def features(self, frame: Frame) -> Embedding:
        raise AssertionError("not called")

    def segment(
        self, frame: Frame, prompts: Sequence[Prompt], embedding: Embedding | None = None
    ) -> list[Mask | None]:
        return [
            self.mask(frame) if isinstance(p, BoxPrompt) and p.box[0] < 0.5 else None
            for p in prompts
        ]

    def find_similar(self, frame: Frame, box: Box) -> list[Box]:
        raise AssertionError("not called")

    def embed(self, frame: Frame) -> bytes:
        raise AssertionError("not called")

    def load_embedding(self, data: bytes) -> Embedding:
        raise AssertionError("not called")


def write_frames(tmp_path: Path, degrees: list[int]) -> list[str]:
    urls = []
    for d in degrees:
        path = tmp_path / f"{d}.png"
        cv2.imwrite(str(path), np.full((H, W, 3), d, np.uint8))
        urls.append(path.as_uri())
    return urls


def test_oriented_track_fits_each_frame_its_own_mask(tmp_path: Path) -> None:
    degrees = [10, 25, 40]
    frames = [
        worker_pb2.TaskFrame(id=i, url=u) for i, u in enumerate(write_frames(tmp_path, degrees))
    ]
    point = annotation_pb2.Point(x=0.5, y=0.5)
    api = FakeApi()

    task = worker_pb2.TrackTask(id=1, frames=frames, point=point, oriented=True)
    run_track(cast(Api, api), MaskTracker(), task)

    boxes = [b for r in api.tracks for b in r.boxes]
    assert [b.frame_id for b in boxes] == [0, 1, 2]
    for b, d in zip(boxes, degrees, strict=True):
        corners = cast(Corners, tuple((p.x, p.y) for p in b.box.corners))
        assert angle(corners, (H, W)) == pytest.approx(d, abs=1)
        assert (b.box.x, b.box.y, b.box.width, b.box.height) == pytest.approx(bounds(corners))

    api = FakeApi()
    run_track(cast(Api, api), MaskTracker(), worker_pb2.TrackTask(id=2, frames=frames, point=point))
    assert all(not b.box.corners for r in api.tracks for b in r.boxes)


def test_orient_with_a_reference_leaves_out_boxes_it_cannot_fit() -> None:
    _, path = markers()
    frame = worker_pb2.OrientFrame(
        frame=worker_pb2.TaskFrame(id=1, url=path.as_uri()),
        boxes=[
            worker_pb2.OrientBox(
                annotation_id=10,
                box=annotation_pb2.Box(x=0.2412, y=0.2453, width=0.1279, height=0.175),
            ),
            # Bare floor.
            worker_pb2.OrientBox(
                annotation_id=11, box=annotation_pb2.Box(x=0.45, y=0.65, width=0.1, height=0.15)
            ),
        ],
    )
    task = worker_pb2.OrientTask(
        id=5, frames=[frame], reference_url=(FIXTURES / "surveyrepair.png").as_uri()
    )
    api = FakeApi()

    run_orient(cast(Api, api), task)

    assert api.orients[-1].done
    (got,) = [b for r in api.orients for b in r.boxes]
    assert got.annotation_id == 10
    assert len(got.box.corners) == 4


def test_orient_without_a_reference_fits_sam_masks(tmp_path: Path) -> None:
    (url,) = write_frames(tmp_path, [30])
    frame = worker_pb2.OrientFrame(
        frame=worker_pb2.TaskFrame(id=1, url=url),
        boxes=[
            worker_pb2.OrientBox(
                annotation_id=10, box=annotation_pb2.Box(x=0.33, y=0.32, width=0.34, height=0.36)
            ),
            # SAM finds nothing here.
            worker_pb2.OrientBox(
                annotation_id=11, box=annotation_pb2.Box(x=0.7, y=0.1, width=0.1, height=0.1)
            ),
        ],
    )
    api = FakeApi()

    run_orient(cast(Api, api), worker_pb2.OrientTask(id=5, frames=[frame]), MaskTracker())

    (got,) = [b for r in api.orients for b in r.boxes]
    assert got.annotation_id == 10
    corners = cast(Corners, tuple((p.x, p.y) for p in got.box.corners))
    assert angle(corners, (H, W)) == pytest.approx(30, abs=1)
