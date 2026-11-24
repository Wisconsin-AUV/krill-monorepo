from collections.abc import Iterable, Iterator, Sequence
from pathlib import Path
from typing import cast

import cv2
import numpy as np
import pytest
from krill.v1 import annotation_pb2, label_pb2, worker_pb2

from krill_worker.api import Api
from krill_worker.frames import decode_frame
from krill_worker.keypoints import (
    HIDDEN,
    UNLABELED,
    VISIBLE,
    Design,
    KeypointTracker,
    Quad,
    Schema,
    center,
    fill_quad,
    find_print,
    load_design,
    paper_in,
    sheet_in,
    snap,
)
from krill_worker.main import run_orient, run_track
from krill_worker.prelabelers import Frame
from krill_worker.track import Box, BoxPrompt, Embedding, Mask, PointPrompt, Prompt

FIXTURES = Path(__file__).parent / "fixtures"

# Paper corners 0 to 3 with the fire design on them and its center at 4, and
# a loose point on the crate's rim at 5.
SCHEMA = Schema(6, (Quad((0, 1, 2, 3), True, 4),))

# The fire design's corners on the fixture frames, clockwise from its own top
# left, which here is also the top left on screen.
PRINT_341 = np.array([[103, 83], [220, 73], [224, 195], [114, 205]], np.float32)
PRINT_342 = np.array([[82, 100], [200, 90], [207, 209], [94, 221]], np.float32)


def design() -> Design:
    return load_design((FIXTURES / "fire.png").read_bytes())


def bin_frame(index: int) -> tuple[Frame, Mask]:
    rgb = decode_frame((FIXTURES / f"bin_fire_{index}.jpg").read_bytes())
    mask = decode_frame((FIXTURES / f"bin_fire_{index}_mask.png").read_bytes())[..., 0] > 0
    return rgb, mask


def quarter_turns(rgb: Frame, mask: Mask, turns: int) -> tuple[Frame, Mask, np.ndarray]:
    """The frame and mask turned clockwise, and the map of a pixel's position."""
    h, w = mask.shape
    codes = [None, cv2.ROTATE_90_CLOCKWISE, cv2.ROTATE_180, cv2.ROTATE_90_COUNTERCLOCKWISE]
    code = codes[turns % 4]
    if code is None:
        return rgb, mask, np.eye(3)
    turned = np.asarray(cv2.rotate(rgb, code), np.uint8)
    turned_mask = cv2.rotate(mask.astype(np.uint8), code) > 0
    maps = {
        cv2.ROTATE_90_CLOCKWISE: [[0, -1, h - 1], [1, 0, 0], [0, 0, 1]],
        cv2.ROTATE_180: [[-1, 0, w - 1], [0, -1, h - 1], [0, 0, 1]],
        cv2.ROTATE_90_COUNTERCLOCKWISE: [[0, 1, 0], [-1, 0, w - 1], [0, 0, 1]],
    }
    return turned, turned_mask, np.array(maps[code], np.float64)


@pytest.mark.parametrize("turns", [0, 1, 2, 3])
def test_find_print_keeps_the_design_order_however_the_bin_is_turned(turns: int) -> None:
    rgb, mask = bin_frame(341)
    rgb, mask, move = quarter_turns(rgb, mask, turns)
    gray = np.asarray(cv2.cvtColor(rgb, cv2.COLOR_RGB2GRAY), np.uint8)

    found = find_print(rgb, gray, mask, design(), None)

    assert found is not None
    want = cv2.perspectiveTransform(PRINT_341.reshape(-1, 1, 2).astype(np.float64), move)
    assert np.abs(found[0] - want.reshape(4, 2)).max() < 4


def test_find_print_ignores_the_bin_without_its_design() -> None:
    rgb, mask = bin_frame(341)
    gray = np.asarray(cv2.cvtColor(rgb, cv2.COLOR_RGB2GRAY), np.uint8)
    blood = np.zeros((240, 240, 3), np.uint8)
    blood[:] = 255
    cv2.circle(blood, (120, 160), 60, (60, 20, 220), -1)
    cv2.fillPoly(blood, [np.array([[70, 140], [170, 140], [120, 30]], np.int32)], (60, 20, 220))

    assert find_print(rgb, gray, mask, load_design(png(blood)), None) is None


def test_find_print_leaves_a_print_the_frame_edge_cuts_off() -> None:
    rgb, mask = bin_frame(341)
    # Cut through the print's left side, so what shows is narrower than the
    # design and its left corners sit on the frame's edge.
    rgb, mask = np.ascontiguousarray(rgb[:, 120:]), np.ascontiguousarray(mask[:, 120:])
    gray = np.asarray(cv2.cvtColor(rgb, cv2.COLOR_RGB2GRAY), np.uint8)

    assert find_print(rgb, gray, mask, design(), None) is None


def png(img: Frame) -> bytes:
    ok, data = cv2.imencode(".png", img)
    assert ok
    return data.tobytes()


PAPER = np.array([[90, 70], [215, 85], [205, 210], [80, 195]], np.float32)


def printed_crate(art: Frame) -> tuple[Frame, Mask]:
    """A dark crate seen from above with the art printed on a sheet on its
    floor at PAPER, tinted as under water."""
    rgb = np.full((280, 300, 3), (25, 45, 80), np.uint8)
    h, w = art.shape[:2]
    corners = np.array([[0, 0], [w, 0], [w, h], [0, h]], np.float32)
    warp = cv2.getPerspectiveTransform(corners, PAPER)
    rgb_art = cv2.cvtColor(art, cv2.COLOR_BGR2RGB)
    sheet = cv2.warpPerspective(rgb_art, warp, (300, 280))
    on = cv2.warpPerspective(np.full((h, w), 255, np.uint8), warp, (300, 280)) > 0
    rgb[on] = (sheet[on] * np.array([0.7, 0.95, 0.9])).astype(np.uint8)
    mask = polygon_mask([(50, 40), (250, 40), (250, 240), (50, 240)])[:280]
    return rgb, mask


def test_find_print_matches_ink_that_runs_off_the_paper() -> None:
    # A drop whose top runs off the sheet, so the bare paper is a U around
    # it with no ink inside its outline.
    art = np.full((240, 240, 3), 255, np.uint8)
    cv2.ellipse(art, (120, 90), (45, 100), 0, 0, 360, (60, 20, 220), -1)
    cv2.circle(art, (120, 150), 55, (60, 20, 220), -1)
    rgb, mask = printed_crate(art)
    gray = np.asarray(cv2.cvtColor(rgb, cv2.COLOR_RGB2GRAY), np.uint8)

    found = find_print(rgb, gray, mask, load_design(png(art)), None)

    assert found is not None
    assert np.abs(found[0] - PAPER).max() < 3


def test_find_print_leaves_a_design_that_looks_the_same_turned() -> None:
    art = np.full((240, 240, 3), 255, np.uint8)
    cv2.circle(art, (120, 120), 70, (60, 20, 220), -1)
    rgb, mask = printed_crate(art)
    gray = np.asarray(cv2.cvtColor(rgb, cv2.COLOR_RGB2GRAY), np.uint8)

    assert find_print(rgb, gray, mask, load_design(png(art)), None) is None
    # A labeler pointing at it still gets its quad.
    filled = fill_quad(rgb, mask, load_design(png(art)))
    assert filled is not None
    nearest = [np.linalg.norm(PAPER - c, axis=1).min() for c in filled[0]]
    assert max(nearest) < 3


def test_tracked_print_drifts_back_onto_the_sheet() -> None:
    rgb, mask = bin_frame(341)
    start = np.zeros((6, 3))
    start[:4, :2] = PRINT_341 + np.array([5, -4])
    start[:4, 2] = VISIBLE
    tracker = KeypointTracker(SCHEMA, design(), start)

    for _ in range(12):
        points = tracker.step(rgb, mask)

    assert np.abs(points[:4, :2] - PRINT_341).max() < 2


def test_tracked_print_keeps_its_size_when_a_wall_covers_part_of_it() -> None:
    rgb, mask = bin_frame(341)
    # The crate's wall over the print's right fifth, as at an angle.
    wall = np.zeros(mask.shape, np.uint8)
    cv2.fillPoly(wall, [np.array([[198, 0], [340, 0], [340, 280], [202, 280]], np.int32)], (1,))
    rgb = rgb.copy()
    rgb[(wall > 0) & mask] = (20, 40, 70)
    start = np.zeros((6, 3))
    start[:4, :2] = PRINT_341
    start[:4, 2] = VISIBLE
    tracker = KeypointTracker(SCHEMA, design(), start)

    for _ in range(5):
        points = tracker.step(rgb, mask)

    assert np.abs(points[:4, :2] - PRINT_341).max() < 2


def polygon_mask(points: list[tuple[int, int]]) -> Mask:
    mask = np.zeros((300, 300), np.uint8)
    cv2.fillPoly(mask, [np.array(points, np.int32)], 1)
    return mask > 0


RIM = np.array([[100, 100], [200, 100], [200, 200], [100, 200]], np.float32)


def test_snap_leaves_the_rim_off_the_near_wall_below_it() -> None:
    # Seen a little from the front, the crate's outline runs on down its near
    # wall to the floor, a few pixels below the rim.
    seen = polygon_mask([(100, 100), (200, 100), (200, 206), (100, 206)])

    assert np.abs(snap(RIM, seen) - RIM).max() < 1


def test_snap_pulls_back_a_corner_that_drifted_out_of_the_outline() -> None:
    seen = polygon_mask([(100, 100), (200, 100), (200, 206), (100, 206)])
    drifted = RIM.copy()
    drifted[1] = (205, 96)

    got = snap(drifted, seen)

    assert got[1] == pytest.approx((200, 100), abs=1.5)
    assert np.abs(got[2:] - RIM[2:]).max() < 1


def test_snap_lifts_a_far_corner_that_sank_inside_back_onto_the_outline() -> None:
    seen = polygon_mask([(100, 100), (200, 100), (200, 206), (100, 206)])
    # The floor shows shifted toward the near wall at the bottom.
    floor = np.array([[115, 125], [185, 125], [185, 195], [115, 195]], np.float32)
    sunk = RIM.copy()
    sunk[0] = (103, 106)

    got = snap(sunk, seen, floor)

    assert got[0] == pytest.approx((100, 100), abs=1.5)
    assert np.abs(got[2:] - RIM[2:]).max() < 1


def test_snap_moves_the_whole_rim_onto_an_outline_seen_from_above() -> None:
    shifted = polygon_mask([(104, 103), (204, 103), (204, 203), (104, 203)])

    assert np.abs(snap(RIM, shifted) - RIM - np.array([4, 3])).max() < 1.5


def shaded_print() -> tuple[Frame, Mask, Mask]:
    """Frame 341 with the print's lower part in the crate's shadow, and the
    whole sheet as SAM segments it."""
    rgb, mask = bin_frame(341)
    sheet = np.zeros(mask.shape, np.uint8)
    cv2.fillConvexPoly(sheet, PRINT_341.astype(np.int32), (1,))
    shade = (sheet > 0) & (np.arange(mask.shape[0])[:, None] > 165)
    rgb = rgb.copy()
    rgb[shade] = rgb[shade] // 2
    return rgb, mask, sheet > 0


def test_find_print_takes_paper_in_shadow_from_sams_sheet() -> None:
    rgb, mask, sheet = shaded_print()
    gray = np.asarray(cv2.cvtColor(rgb, cv2.COLOR_RGB2GRAY), np.uint8)

    assert find_print(rgb, gray, mask, design(), None) is None
    found = find_print(rgb, gray, mask, design(), None, lambda box: sheet)
    assert found is not None
    assert np.abs(found[0] - PRINT_341).max() < 2


def test_sheet_in_ignores_sam_segmenting_the_crate_or_the_design() -> None:
    rgb, mask = bin_frame(341)
    gray = np.asarray(cv2.cvtColor(rgb, cv2.COLOR_RGB2GRAY), np.uint8)
    bright = paper_in(gray, mask)
    assert bright is not None
    design_only = np.zeros(mask.shape, np.uint8)
    cv2.circle(design_only, (165, 140), 30, (1,), -1)

    assert sheet_in(mask, bright, lambda box: mask) is None
    assert sheet_in(mask, bright, lambda box: design_only > 0) is None


def test_center_is_where_the_diagonals_cross_in_perspective() -> None:
    square = np.array([[0, 0], [100, 0], [100, 100], [0, 100]], np.float32)
    seen = np.array([[10, 10], [90, 20], [80, 70], [20, 90]], np.float32)
    h = cv2.getPerspectiveTransform(square, seen)
    want = cv2.perspectiveTransform(np.array([[[50.0, 50.0]]]), h).reshape(2)

    got = center(seen)

    assert got is not None
    assert got == pytest.approx(want, abs=1e-6)
    # The plain average of the corners is not the center under perspective.
    assert np.abs(seen.mean(axis=0) - want).max() > 1


def test_tracker_finds_the_print_and_carries_other_points_with_the_bin() -> None:
    first, first_mask = bin_frame(341)
    second, second_mask = bin_frame(342)
    start = np.zeros((6, 3))
    rim = (70.0, 50.0)
    start[5] = (*rim, VISIBLE)
    tracker = KeypointTracker(SCHEMA, design(), start)

    on_first = tracker.step(first, first_mask)
    on_second = tracker.step(second, second_mask)

    assert np.abs(on_first[:4, :2] - PRINT_341).max() < 4
    assert np.abs(on_second[:4, :2] - PRINT_342).max() < 4
    assert (on_second[:4, 2] == VISIBLE).all()
    assert on_second[4, 2] == VISIBLE
    # The bin moved about 21 pixels left and 17 down between the frames.
    moved = on_second[5, :2] - np.array(rim)
    assert moved == pytest.approx((-21, 17), abs=4)
    assert on_second[5, 2] == VISIBLE


def test_tracker_hides_points_that_leave_the_frame_and_drops_folded_quads() -> None:
    first, first_mask = bin_frame(341)
    second, second_mask = bin_frame(342)
    schema = Schema(5, (Quad((0, 1, 2, 3), False, None),))
    start = np.zeros((5, 3))
    # Counterclockwise on screen, which the front of a flat surface never is.
    start[:4] = [(150, 150, VISIBLE), (150, 250, VISIBLE), (250, 250, VISIBLE), (250, 150, VISIBLE)]
    start[4] = (15, 50, VISIBLE)
    tracker = KeypointTracker(schema, None, start)

    tracker.step(first, first_mask)
    on_second = tracker.step(second, second_mask)

    assert (on_second[:4, 2] == UNLABELED).all()
    assert on_second[4, 0] < 0
    assert on_second[4, 2] == HIDDEN


class FakeApi:
    def __init__(self) -> None:
        self.tracks: list[worker_pb2.ReportTrackRequest] = []
        self.orients: list[worker_pb2.ReportOrientRequest] = []

    def report_track(self, req: worker_pb2.ReportTrackRequest) -> None:
        self.tracks.append(req)

    def report_orient(self, req: worker_pb2.ReportOrientRequest) -> None:
        self.orients.append(req)


class BinTracker:
    """SAM that knows the fixtures: the bin's mask on each frame, and the
    paper's when pointed at."""

    model_id = "facebook/sam2.1-hiera-small"
    supports_exemplars = False

    def __init__(self) -> None:
        self.segmented: list[Prompt] = []

    def mask(self, frame: Frame) -> Mask:
        for index in (341, 342):
            rgb, mask = bin_frame(index)
            if np.array_equal(rgb, frame):
                return mask
        raise AssertionError("not a fixture frame")

    def track(
        self, frames: Iterable[tuple[Frame, Embedding | None]], prompt: Prompt
    ) -> Iterator[Mask | None]:
        for frame, _ in frames:
            yield self.mask(frame)

    def features(self, frame: Frame) -> Embedding:
        return []

    def segment(
        self, frame: Frame, prompts: Sequence[Prompt], embedding: Embedding | None = None
    ) -> list[Mask | None]:
        self.segmented.extend(prompts)
        return [self.mask(frame) for _ in prompts]

    def find_similar(self, frame: Frame, box: Box) -> list[Box]:
        raise AssertionError("not called")

    def embed(self, frame: Frame) -> bytes:
        raise AssertionError("not called")

    def load_embedding(self, data: bytes) -> Embedding:
        raise AssertionError("not called")


def task_frames() -> list[worker_pb2.TaskFrame]:
    return [
        worker_pb2.TaskFrame(id=i, url=(FIXTURES / f"bin_fire_{i}.jpg").as_uri())
        for i in (341, 342)
    ]


def quads() -> list[label_pb2.KeypointQuad]:
    return [label_pb2.KeypointQuad(corners=[0, 1, 2, 3], reference=True, center=4)]


def test_track_reports_every_keypoint_and_fills_the_quad_pointed_at() -> None:
    frames = task_frames()
    h, w = 280, 340
    keypoints = [annotation_pb2.Keypoint()] * 5 + [
        annotation_pb2.Keypoint(x=70 / w, y=50 / h, visibility=VISIBLE)
    ]
    task = worker_pb2.TrackTask(
        id=1,
        frames=frames,
        box=annotation_pb2.Box(x=0.2, y=0.1, width=0.5, height=0.7, keypoints=keypoints),
        keypoint_count=6,
        quads=quads(),
        # No reference design, so the quad is filled from the mask alone, in
        # screen order.
        fill=annotation_pb2.QuadPrompt(quad=0, point=annotation_pb2.Point(x=0.5, y=0.5)),
    )
    api = FakeApi()
    tracker = BinTracker()

    run_track(cast(Api, api), tracker, task)

    assert tracker.segmented == [PointPrompt(0.5, 0.5)]
    boxes = [b for r in api.tracks for b in r.boxes]
    assert [b.frame_id for b in boxes] == [341, 342]
    for b in boxes:
        assert len(b.box.keypoints) == 6
        assert all(k.visibility != UNLABELED for k in b.box.keypoints)
    first = boxes[0].box.keypoints
    assert (first[5].x, first[5].y) == pytest.approx((70 / w, 50 / h))


def test_tracking_carries_on_from_keypoints_labeled_on_later_frames() -> None:
    first, second, again = task_frames() + task_frames()[1:]
    labeled = (100.0, 60.0)
    second.keypoints.extend(
        [annotation_pb2.Keypoint()] * 5
        + [annotation_pb2.Keypoint(x=labeled[0] / 340, y=labeled[1] / 280, visibility=HIDDEN)]
    )
    task = worker_pb2.TrackTask(
        id=4,
        frames=[first, second, again],
        point=annotation_pb2.Point(x=0.5, y=0.5),
        keypoint_count=6,
        quads=quads(),
    )
    api = FakeApi()

    run_track(cast(Api, api), BinTracker(), task)

    boxes = [b for r in api.tracks for b in r.boxes]
    assert [b.frame_id for b in boxes] == [341, 342, 342]
    # The same frame again, so tracking leaves the point where it was labeled.
    rim = boxes[2].box.keypoints[5]
    assert (rim.x * 340, rim.y * 280) == pytest.approx(labeled, abs=1)
    assert rim.visibility == HIDDEN


def test_keypoints_labeled_on_earlier_frames_ride_on_the_print() -> None:
    (_, second) = task_frames()
    w, h = 340, 280
    rim = (70.0, 50.0)
    earlier = [annotation_pb2.Keypoint(x=x / w, y=y / h, visibility=VISIBLE) for x, y in PRINT_341]
    earlier += [
        annotation_pb2.Keypoint(),
        annotation_pb2.Keypoint(x=rim[0] / w, y=rim[1] / h, visibility=VISIBLE),
    ]
    task = worker_pb2.TrackTask(
        id=5,
        frames=[second],
        # Tracked again from a frame where only the print is labeled.
        box=annotation_pb2.Box(
            x=0.2,
            y=0.1,
            width=0.5,
            height=0.7,
            keypoints=[
                annotation_pb2.Keypoint(x=x / w, y=y / h, visibility=VISIBLE) for x, y in PRINT_342
            ],
        ),
        reference_url=(FIXTURES / "fire.png").as_uri(),
        keypoint_count=6,
        quads=quads(),
        labeled_before=[worker_pb2.KeypointSet(keypoints=earlier)],
    )
    api = FakeApi()

    run_track(cast(Api, api), BinTracker(), task)

    (got,) = [b for r in api.tracks for b in r.boxes]
    k = got.box.keypoints[5]
    # Where the rim sat on the print a frame earlier, carried by the print's
    # move of about 21 pixels left and 17 down.
    assert (k.x * w, k.y * h) == pytest.approx((rim[0] - 21, rim[1] + 17), abs=4)
    assert k.visibility == VISIBLE


class CountingTracker(BinTracker):
    def __init__(self) -> None:
        super().__init__()
        self.encoded = 0
        self.tracked_with: list[Embedding | None] = []

    def track(
        self, frames: Iterable[tuple[Frame, Embedding | None]], prompt: Prompt
    ) -> Iterator[Mask | None]:
        for frame, embedding in frames:
            self.tracked_with.append(embedding)
            yield self.mask(frame)

    def features(self, frame: Frame) -> Embedding:
        self.encoded += 1
        return ["features"]


def test_tracking_a_print_encodes_each_frame_once_for_tracking_and_its_sheet() -> None:
    task = worker_pb2.TrackTask(
        id=6,
        frames=task_frames(),
        box=annotation_pb2.Box(x=0.2, y=0.1, width=0.5, height=0.7),
        reference_url=(FIXTURES / "fire.png").as_uri(),
        keypoint_count=6,
        quads=quads(),
    )
    tracker = CountingTracker()

    run_track(cast(Api, FakeApi()), tracker, task)

    assert tracker.encoded == 2
    assert tracker.tracked_with == [["features"], ["features"]]
    assert tracker.segmented


def test_filling_a_printed_quad_finds_the_paper_in_the_object_not_the_click() -> None:
    task = worker_pb2.TrackTask(
        id=2,
        frames=task_frames()[:1],
        box=annotation_pb2.Box(x=0.2, y=0.1, width=0.5, height=0.7),
        reference_url=(FIXTURES / "fire.png").as_uri(),
        keypoint_count=6,
        quads=quads(),
        fill=annotation_pb2.QuadPrompt(quad=0, point=annotation_pb2.Point(x=0.5, y=0.5)),
    )
    api = FakeApi()
    tracker = BinTracker()

    run_track(cast(Api, api), tracker, task)

    assert not any(isinstance(p, PointPrompt) for p in tracker.segmented)
    (got,) = [b for r in api.tracks for b in r.boxes]
    corners = np.array([(k.x * 340, k.y * 280) for k in got.box.keypoints[:4]])
    assert np.abs(corners - PRINT_341).max() < 4


def test_fit_keypoints_finds_the_print_inside_each_box() -> None:
    (frame, _) = task_frames()
    task = worker_pb2.OrientTask(
        id=3,
        frames=[
            worker_pb2.OrientFrame(
                frame=frame,
                boxes=[
                    worker_pb2.OrientBox(
                        annotation_id=7,
                        box=annotation_pb2.Box(x=0.2, y=0.1, width=0.5, height=0.7),
                    )
                ],
            )
        ],
        reference_url=(FIXTURES / "fire.png").as_uri(),
        keypoint_count=6,
        quads=quads(),
    )
    api = FakeApi()
    tracker = BinTracker()

    run_orient(cast(Api, api), task, tracker)

    assert isinstance(tracker.segmented[0], BoxPrompt)
    (got,) = [b for r in api.orients for b in r.boxes]
    assert got.annotation_id == 7
    kps = got.box.keypoints
    corners = np.array([(k.x * 340, k.y * 280) for k in kps[:4]])
    assert np.abs(corners - PRINT_341).max() < 4
    assert kps[4].visibility == VISIBLE
    assert kps[5].visibility == UNLABELED
