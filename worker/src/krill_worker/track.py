from collections.abc import Iterable, Iterator
from dataclasses import dataclass
from typing import Any

import numpy as np
import numpy.typing as npt

# Normalized x, y, width, height with the origin at the top left.
Box = tuple[float, float, float, float]

# A frame's SAM image-encoder features, one tensor per feature level.
Embedding = list[Any]

# Masks thinner than this many pixels are noise, not an object.
MIN_MASK_PIXELS = 2

# A box within this much of the frame border is cut off by it.
EDGE = 0.005
# A cut-off box narrower than this share of its size before it reached the
# border is too little of the object to recognize on its own.
MIN_VISIBLE = 0.4


@dataclass(frozen=True)
class PointPrompt:
    x: float
    y: float


@dataclass(frozen=True)
class BoxPrompt:
    box: Box


Prompt = PointPrompt | BoxPrompt


def mask_box(mask: npt.NDArray[np.bool_]) -> Box | None:
    rows = np.flatnonzero(mask.any(axis=1))
    cols = np.flatnonzero(mask.any(axis=0))
    if len(rows) == 0:
        return None
    y0, y1 = int(rows[0]), int(rows[-1]) + 1
    x0, x1 = int(cols[0]), int(cols[-1]) + 1
    if y1 - y0 < MIN_MASK_PIXELS or x1 - x0 < MIN_MASK_PIXELS:
        return None
    h, w = mask.shape
    return x0 / w, y0 / h, (x1 - x0) / w, (y1 - y0) / h


def drop_cut_off(boxes: Iterable[Box | None]) -> Iterator[Box | None]:
    """Replace boxes on slivers of an object leaving the frame with None.

    Size is measured against the last box clear of the border rather than the
    prompt, and only across the border it touches, so an object that narrows
    by turning away from the camera is kept.
    """
    clear: Box | None = None
    for box in boxes:
        if box is None:
            yield None
            continue
        x, y, w, h = box
        sideways = x <= EDGE or x + w >= 1 - EDGE
        vertical = y <= EDGE or y + h >= 1 - EDGE
        if not (sideways or vertical):
            clear = box
        elif clear is not None and (
            (sideways and w < clear[2] * MIN_VISIBLE) or (vertical and h < clear[3] * MIN_VISIBLE)
        ):
            yield None
            continue
        yield box
