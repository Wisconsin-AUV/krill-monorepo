"""Fitting rotated boxes to objects inside axis-aligned ones.

The object's outline comes from its mask when there is one, else from the
bright printed paper inside the box. With the printed reference design, the
outline's quadrilateral is aligned to the design and the design's own corners
are used, which is far more precise than the thresholded edge.
"""

from collections.abc import Sequence
from dataclasses import dataclass

import cv2
import numpy as np
import numpy.typing as npt
from cv2.typing import MatLike

from krill_worker.geometry import Corners, bounds, iou, rotated_rect
from krill_worker.prelabelers import Frame
from krill_worker.track import Box, Mask

# How far around the box to look for the paper, as a share of its size, since
# boxes are often drawn a little tight.
PAD = 0.15
# The reference design's long side, in pixels, while matching. Frames are at
# most 1024 pixels wide, so prints rarely cover more, and ECC's time grows
# with it.
DESIGN_SIDE = 200
# ECC often oscillates by hundredths of a pixel instead of converging.
ECC_ITERATIONS = 50
ECC_EPS = 1e-4
# A fit whose bounds overlap the box less than this found something else.
# Prints fit about 0.85 to the boxes around them, and the floor near them
# under 0.6.
MIN_IOU = 0.7
# ECC correlation below this aligned the design to something else. Prints
# reach about 0.93, and other designs or bare floor under 0.6.
MIN_MATCH = 0.8
# ECC that moved a corner further than this share of the quad's size locked on
# to something other than the print.
MAX_DRIFT = 0.25
# Outlines covering fewer pixels than this are specks, not objects.
MIN_AREA = 25
# Pixels at DESIGN_SIDE covering the thin black cut line around a design's
# edge, which isn't on the print.
CUT_LINE = 2

Points = npt.NDArray[np.float32]
Gray = npt.NDArray[np.uint8]


@dataclass(frozen=True)
class Reference:
    """A printed design, ready to match against frames."""

    design: Gray
    # Blurred the way ECC compares it.
    smooth: npt.NDArray[np.float32]
    # The design's corners clockwise from its top left, in its pixels.
    corners: Points
    # Where NCC compares, leaving out the border, where a quad that is a little
    # off samples the floor around the print.
    window: npt.NDArray[np.bool_]


def load_reference(data: bytes) -> Reference:
    img = cv2.imdecode(np.frombuffer(data, np.uint8), cv2.IMREAD_UNCHANGED)
    if img is None:
        raise ValueError("reference design is not a valid image")
    if img.ndim == 3 and img.shape[2] == 4:
        # Transparent parts are bare paper.
        alpha = img[..., 3:].astype(np.float32) / 255
        img = (img[..., :3] * alpha + 255 * (1 - alpha)).astype(np.uint8)
    gray = img if img.ndim == 2 else cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
    h, w = gray.shape[:2]
    scale = DESIGN_SIDE / max(h, w)
    size = (max(1, round(w * scale)), max(1, round(h * scale)))
    design = np.asarray(cv2.resize(gray, size, interpolation=cv2.INTER_AREA), dtype=np.uint8)
    b = CUT_LINE
    design[:b, :] = design[-b:, :] = 255
    design[:, :b] = design[:, -b:] = 255
    dw, dh = size
    window = np.zeros((dh, dw), bool)
    m = max(dw, dh) // 20
    window[m : dh - m, m : dw - m] = True
    return Reference(
        design=design,
        smooth=cv2.GaussianBlur(design, (5, 5), 0).astype(np.float32),
        corners=np.array([[0, 0], [dw, 0], [dw, dh], [0, dh]], np.float32),
        window=window,
    )


def fit(
    frame: Frame,
    box: Box,
    mask: Mask | None = None,
    reference: Reference | None = None,
) -> Corners | None:
    """The object's rotated box, or None when no plausible one is found.

    box is the object's normalized axis-aligned box, and mask its pixels when
    known.
    """
    gray = np.asarray(cv2.cvtColor(frame, cv2.COLOR_RGB2GRAY), dtype=np.uint8)
    outline = mask_outline(mask) if mask is not None else paper_outline(gray, box)
    if outline is None:
        return None
    points = outline if reference is None else align(gray, quad(outline), reference)
    corners = rotated_rect(points, frame.shape)
    if iou(bounds(corners), box) < MIN_IOU:
        return None
    return corners


def mask_outline(mask: Mask) -> Points | None:
    contours, _ = cv2.findContours(mask.astype(np.uint8), cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_NONE)
    return largest(contours)


def paper_outline(gray: Gray, box: Box) -> Points | None:
    """Outline of the brightest region around box, which is the paper an
    object is printed on."""
    h, w = gray.shape
    x, y, bw, bh = box
    x0, x1 = (round(min(max(v, 0), 1) * w) for v in (x - bw * PAD, x + bw * (1 + PAD)))
    y0, y1 = (round(min(max(v, 0), 1) * h) for v in (y - bh * PAD, y + bh * (1 + PAD)))
    crop = gray[y0:y1, x0:x1]
    if min(crop.shape) < 8:
        return None
    blur = cv2.GaussianBlur(crop, (5, 5), 0)
    _, bright = cv2.threshold(blur, 0, 255, cv2.THRESH_BINARY + cv2.THRESH_OTSU)
    k = max(3, min(crop.shape) // 25) | 1
    bright = cv2.morphologyEx(bright, cv2.MORPH_CLOSE, np.ones((k, k), np.uint8))
    contours, _ = cv2.findContours(bright, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_NONE)
    outline = largest(contours)
    return None if outline is None else outline + np.array([x0, y0], np.float32)


def largest(contours: Sequence[MatLike]) -> Points | None:
    if not contours:
        return None
    c = max(contours, key=cv2.contourArea)
    if cv2.contourArea(c) < MIN_AREA:
        return None
    return np.asarray(c.reshape(-1, 2), dtype=np.float32)


def quad(outline: Points) -> Points:
    """The outline's four corners, clockwise on screen."""
    hull = cv2.convexHull(outline)
    peri = cv2.arcLength(hull, True)
    for eps in np.linspace(0.01, 0.1, 40):
        q = cv2.approxPolyDP(hull, eps * peri, True)
        if len(q) == 4:
            break
    else:
        q = cv2.boxPoints(cv2.minAreaRect(outline))
    q = np.asarray(q, dtype=np.float32).reshape(4, 2)
    center = q.mean(axis=0)
    return q[np.argsort(np.arctan2(q[:, 1] - center[1], q[:, 0] - center[0]))]


def ncc(a: Gray, b: Gray, window: npt.NDArray[np.bool_]) -> float:
    x = a[window].astype(np.float32)
    y = b[window].astype(np.float32)
    x -= x.mean()
    y -= y.mean()
    return float((x * y).sum() / (np.sqrt((x * x).sum() * (y * y).sum()) + 1e-9))


def align(gray: Gray, q: Points, ref: Reference) -> Points:
    """The design's corners in the frame, found by trying it at each quarter
    turn on the quad and refining the best with ECC. Falls back to the quad."""
    dh, dw = ref.design.shape
    best = -np.inf
    homography = np.eye(3)
    start = q
    for k in range(4):
        rolled = np.roll(q, -k, axis=0)
        h = np.asarray(cv2.getPerspectiveTransform(ref.corners, rolled), dtype=np.float64)
        try:
            inverse = np.linalg.inv(h)
        except np.linalg.LinAlgError:
            return q
        rect = cv2.warpPerspective(gray, inverse, (dw, dh), flags=cv2.INTER_LINEAR)
        score = ncc(np.asarray(rect, dtype=np.uint8), ref.design, ref.window)
        if score > best:
            best, homography, start = score, h, rolled
    warp = (homography / homography[2, 2]).astype(np.float32)
    criteria = (cv2.TERM_CRITERIA_EPS | cv2.TERM_CRITERIA_COUNT, ECC_ITERATIONS, ECC_EPS)
    image = cv2.GaussianBlur(gray, (3, 3), 0).astype(np.float32)
    try:
        match, warp = cv2.findTransformECC(
            ref.smooth, image, warp, cv2.MOTION_HOMOGRAPHY, criteria, None
        )
    except cv2.error:
        return q
    if match < MIN_MATCH:
        return q
    corners = cv2.perspectiveTransform(ref.corners.reshape(-1, 1, 2), warp).reshape(4, 2)
    size = np.sqrt(cv2.contourArea(q))
    if not np.isfinite(corners).all() or np.linalg.norm(corners - start, axis=1).max() > (
        MAX_DRIFT * size
    ):
        return q
    return np.asarray(corners, dtype=np.float32)
