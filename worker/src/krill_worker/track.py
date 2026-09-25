from dataclasses import dataclass

import numpy as np
import numpy.typing as npt

# Normalized x, y, width, height with the origin at the top left.
Box = tuple[float, float, float, float]

# Masks thinner than this many pixels are noise, not an object.
MIN_MASK_PIXELS = 2


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
