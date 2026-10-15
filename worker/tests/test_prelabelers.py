import math

import cv2
import numpy as np
import pytest
from krill.v1 import pipeline_pb2

from krill_worker.prelabelers import Frame
from krill_worker.prelabelers.params import HsvRange, parse, specs
from krill_worker.prelabelers.path_marker import PathMarker, PathMarkerParams
from krill_worker.prelabelers.slalom import Slalom, SlalomParams

H, W = 480, 640
POOL = (20, 90, 130)
ORANGE = (255, 120, 0)
RED = (200, 20, 25)
WHITE = (235, 235, 235)


def pool() -> Frame:
    frame = np.zeros((H, W, 3), np.uint8)
    frame[:] = POOL
    # Sensor noise, so thresholds are not tested on flat colour.
    noise = np.random.default_rng(0).integers(-6, 7, frame.shape)
    return np.asarray(np.clip(frame.astype(int) + noise, 0, 255), dtype=np.uint8)


def draw_rect(
    frame: Frame,
    center: tuple[float, float],
    size: tuple[float, float],
    deg: float,
    rgb: tuple[int, int, int],
) -> None:
    pts = cv2.boxPoints((center, size, deg)).astype(np.int32)
    cv2.fillPoly(frame, [pts], rgb)


def test_path_marker_finds_the_rotated_marker_and_its_angle() -> None:
    frame = pool()
    draw_rect(frame, (320, 240), (220, 40), 30, ORANGE)
    # Orange, but square, so not a marker.
    draw_rect(frame, (80, 400), (60, 60), 0, ORANGE)
    # Long, but red.
    draw_rect(frame, (520, 80), (160, 30), 0, RED)

    got = PathMarker(PathMarkerParams())(frame)

    assert len(got) == 1
    p = got[0]
    assert p.label == "marker"
    assert p.angle is not None and p.corners is not None
    assert math.degrees(p.angle) == pytest.approx(30, abs=2)
    cx, cy, w, _ = p.box
    assert cx == pytest.approx(0.5, abs=0.01)
    assert cy == pytest.approx(0.5, abs=0.01)
    # The axis-aligned box covers the whole rotated marker.
    want_w = (220 * math.cos(math.radians(30)) + 40 * math.sin(math.radians(30))) / W
    assert w == pytest.approx(want_w, rel=0.05)
    assert p.confidence > 0.9


def test_path_marker_angle_is_the_long_side_either_way() -> None:
    for deg in (-60, 0, 80):
        frame = pool()
        draw_rect(frame, (320, 240), (40, 220), deg + 90, ORANGE)
        (p,) = PathMarker(PathMarkerParams())(frame)
        assert p.angle is not None
        diff = (math.degrees(p.angle) - deg + 90) % 180 - 90
        assert abs(diff) < 2


def test_path_marker_drops_specks() -> None:
    frame = pool()
    draw_rect(frame, (100, 100), (12, 3), 0, ORANGE)
    assert PathMarker(PathMarkerParams())(frame) == []


def test_slalom_tells_red_from_white_and_skips_lying_pipes() -> None:
    frame = pool()
    draw_rect(frame, (150, 240), (18, 260), 0, RED)
    draw_rect(frame, (330, 240), (18, 260), 8, WHITE)
    # A pipe lying on the floor is not a slalom pipe.
    draw_rect(frame, (500, 420), (200, 16), 0, RED)

    got = sorted(Slalom(SlalomParams())(frame), key=lambda p: p.box[0])

    assert [p.label for p in got] == ["red", "white"]
    assert got[0].box[0] == pytest.approx(150 / W, abs=0.01)
    assert got[0].box[3] == pytest.approx(260 / H, abs=0.02)
    assert all(p.angle is None for p in got)


def test_red_hue_wraps_around_zero() -> None:
    frame = pool()
    # Hue just below 180 and just above 0 are both red.
    draw_rect(frame, (200, 240), (18, 260), 0, (220, 10, 40))
    draw_rect(frame, (400, 240), (18, 260), 0, (220, 40, 10))
    got = Slalom(SlalomParams())(frame)
    assert [p.label for p in got] == ["red", "red"]


def test_mask_shows_what_the_thresholds_keep() -> None:
    frame = pool()
    draw_rect(frame, (320, 240), (220, 40), 0, ORANGE)
    mask = PathMarker(PathMarkerParams()).mask(frame)
    assert mask.shape == (H, W)
    assert mask[240, 320] == 255
    assert mask[20, 20] == 0


def test_hsv_range_without_wrap() -> None:
    hsv = np.array([[[10, 200, 200], [100, 200, 200]]], np.uint8)
    assert HsvRange(5, 100, 100, 20, 255, 255).mask(hsv).tolist() == [[255, 0]]
    assert HsvRange(170, 100, 100, 20, 255, 255).mask(hsv).tolist() == [[255, 0]]


def test_params_round_trip_through_the_api() -> None:
    by_name = {s.name: s for s in specs(SlalomParams)}
    assert by_name["red"].kind == pipeline_pb2.PARAM_KIND_HSV_RANGE
    assert by_name["kernel"].kind == pipeline_pb2.PARAM_KIND_INT
    assert by_name["max_tilt"].max == 90

    values = [s.default_value for s in by_name.values()]
    values[list(by_name).index("kernel")] = pipeline_pb2.ParamValue(name="kernel", number=9)
    got = parse(SlalomParams, values)
    assert got.kernel == 9
    assert got.red == SlalomParams().red

    with pytest.raises(ValueError):
        parse(SlalomParams, [pipeline_pb2.ParamValue(name="kernel", flag=True)])
