import cv2
import numpy as np
import pytest

from krill_worker.frames import decode_frame
from krill_worker.track import drop_cut_off, mask_box


def test_mask_box() -> None:
    mask = np.zeros((100, 200), dtype=bool)
    mask[10:30, 50:150] = True
    assert mask_box(mask) == pytest.approx((0.25, 0.1, 0.5, 0.2))


@pytest.mark.parametrize("rows, cols", [(slice(0, 0), slice(0, 0)), (slice(5, 6), slice(0, 50))])
def test_mask_box_ignores_empty_and_thin_masks(rows: slice, cols: slice) -> None:
    mask = np.zeros((100, 200), dtype=bool)
    mask[rows, cols] = True
    assert mask_box(mask) is None


def test_decode_frame_shrinks_to_max_side() -> None:
    ok, jpg = cv2.imencode(".jpg", np.zeros((1080, 1920, 3), dtype=np.uint8))
    assert ok
    assert decode_frame(jpg.tobytes()).shape == (576, 1024, 3)


def test_decode_frame_rejects_garbage() -> None:
    with pytest.raises(ValueError):
        decode_frame(b"not an image")


def test_drop_cut_off_keeps_object_turning_away_mid_frame() -> None:
    turning = [(0.4, 0.4, 0.2, 0.2), (0.45, 0.4, 0.1, 0.2), (0.49, 0.4, 0.02, 0.2)]
    assert list(drop_cut_off(turning)) == turning


def test_drop_cut_off_drops_sliver_leaving_frame() -> None:
    leaving = [
        (0.1, 0.4, 0.2, 0.2),
        (0.0, 0.4, 0.15, 0.2),
        (0.0, 0.4, 0.03, 0.2),
        None,
        (0.0, 0.4, 0.02, 0.2),
    ]
    assert list(drop_cut_off(leaving)) == [leaving[0], leaving[1], None, None, None]


def test_drop_cut_off_measures_against_last_clear_box_not_prompt() -> None:
    # Turned to a quarter of its width before reaching the border, then
    # pushed slightly past it.
    boxes = [(0.4, 0.4, 0.2, 0.2), (0.7, 0.4, 0.05, 0.2), (0.96, 0.4, 0.04, 0.2)]
    assert list(drop_cut_off(boxes)) == boxes


def test_drop_cut_off_only_measures_across_the_touched_border() -> None:
    # Flattened against the bottom border but still its full width.
    boxes = [(0.4, 0.4, 0.2, 0.2), (0.4, 0.9, 0.2, 0.1), (0.4, 0.95, 0.2, 0.05)]
    assert list(drop_cut_off(boxes)) == [boxes[0], boxes[1], None]


def test_drop_cut_off_keeps_prompt_already_at_border() -> None:
    boxes = [(0.0, 0.4, 0.02, 0.2), (0.0, 0.4, 0.01, 0.2)]
    assert list(drop_cut_off(boxes)) == boxes
