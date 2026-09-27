import cv2
import numpy as np
import pytest

from krill_worker.main import decode_frame
from krill_worker.track import mask_box


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
