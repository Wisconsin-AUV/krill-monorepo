"""Tracking a label type's keypoints along with SAM's mask of the object.

Keypoints follow the object's motion between frames, measured by optical flow
inside its mask. A quad with the type's reference design is instead found
again on every frame, by aligning the design to the sheet of paper inside the
mask, so it can't drift and its corners come out in the design's own order
however the object is turned. Keypoints on other quads snap to the corners of
the mask's outline when they line up with them.
"""

from collections.abc import Callable, Sequence
from dataclasses import dataclass

import cv2
import numpy as np
import numpy.typing as npt
from krill.v1 import annotation_pb2, label_pb2

from krill_worker.orient import largest, ncc, quad
from krill_worker.prelabelers import Frame
from krill_worker.track import Box, Mask

UNLABELED = annotation_pb2.KEYPOINT_VISIBILITY_UNSPECIFIED
HIDDEN = annotation_pb2.KEYPOINT_VISIBILITY_HIDDEN
VISIBLE = annotation_pb2.KEYPOINT_VISIBILITY_VISIBLE

# Rows of x and y in pixels and the visibility.
Keypoints = npt.NDArray[np.float64]
Points = npt.NDArray[np.float32]
Gray = npt.NDArray[np.uint8]
# SAM's mask of whatever fills a box on the frame being looked at.
Segment = Callable[[Box], Mask | None]

# The design's side, in pixels, while matching. Frames are at most 1024
# pixels wide, so prints rarely cover more.
DESIGN_SIDE = 200
# Correlation between the design and the frame under a fit, measured over
# the paper that shows and the ink it surrounds. Aligned prints score about
# 0.7 to 0.9 under water, and a fit onto the crate under 0.45.
MIN_SCORE = 0.45
# A print found with nothing tracked yet to compare against must score
# higher, and beat its other quarter turns by a margin: a design that is one
# blob of ink matches itself turned by a quarter at up to 0.7, as little as
# 0.07 below its right turn.
MIN_FOUND_SCORE = 0.6
MIN_TURN_MARGIN = 0.04
# Under this share of the design on visible paper, a score says too little.
MIN_COVER = 0.25
# A fresh find further than this share of the quad's size from where the
# print was tracked to is a different reading of it, not a correction.
MAX_SHIFT = 0.3
# How much better than the tracked print a fresh find must score to replace
# it.
FRESH_MARGIN = 0.05
# Otherwise the tracked print moves this share of the way toward the fresh
# find. Flow drifts a little every frame and fresh finds are a few pixels off
# at random, so going part of the way stops the drift building up and evens
# out the jitter.
PULL = 0.2
# A fresh find smaller or larger than the tracked print by more than these
# factors outlines only the paper the crate's wall leaves in view, or SAM's
# sheet spilling onto the wall, so it doesn't pull.
MIN_FRESH_AREA = 0.85
MAX_FRESH_AREA = 1.2
# How far toward the quad's center, as a share of the way, a corner is
# checked for paper. The paper's own edge is darker than its middle.
INSET = 0.12
# Corners within this share of their quad's size of the outline's corners
# are snapped to them.
SNAP = 0.08
# How far, as a share of the quad's size, the snapped corners may be from a
# single affine move of the quad.
SNAP_SHAPE = 0.015
# Pixels past an object's outline a corner on its outline may sit, from the
# outline's own jaggedness.
OUTSIDE_PIXELS = 2.0
# A tracked keypoint further than this many box sizes from the object's mask
# has lost it.
LOST = 0.75
# Optical flow needs this many features that agree to give the motion.
MIN_FEATURES = 8
# Most features flow finds in the mask.
MAX_FEATURES = 300
# Pixels a feature may land from where it started after flowing there and
# back.
MAX_ROUND_TRIP = 1.0
# Pixels a feature may be off its fit motion and still agree with it.
RANSAC_PIXELS = 2.0
# Paper regions smaller than this many pixels can't be matched.
MIN_PAPER = 100
# A print found unprompted must be at least this many pixels across, and
# keep its corners further than EDGE_PIXELS inside the frame.
MIN_SIDE = 16
EDGE_PIXELS = 2
# A quad whose area changes by more than this factor in one frame has folded
# over or slid off its object, as when a crate turns edge on.
MAX_GROWTH = 2.5
# Room around the bright paper, as a share of its size, in the box SAM is
# asked for the whole sheet with, so the box takes in paper in the crate's
# shadow.
SHEET_PAD = 0.08
# SAM's sheet must cover this share of the bright paper, or it segmented the
# design alone, and at most this share of the object's mask, or it segmented
# the crate. Sheets fill at most about 0.6 of a crate seen from above.
SHEET_COVER = 0.9
SHEET_MAX = 0.8
# Bright paper narrower than this share of its own side, like a lit strip
# along a wall, doesn't stretch its hull, which would then reach over paper
# in shadow, and shadow reads as ink.
AREA_OPEN = 0.08


@dataclass(frozen=True)
class Quad:
    corners: tuple[int, int, int, int]
    reference: bool
    center: int | None


@dataclass(frozen=True)
class Schema:
    count: int
    quads: tuple[Quad, ...]

    @staticmethod
    def of(count: int, quads: Sequence[label_pb2.KeypointQuad]) -> "Schema":
        return Schema(
            count,
            tuple(
                Quad(
                    (q.corners[0], q.corners[1], q.corners[2], q.corners[3]),
                    q.reference,
                    q.center if q.HasField("center") else None,
                )
                for q in quads
                if len(q.corners) == 4
            ),
        )

    @property
    def reference(self) -> Quad | None:
        return next((q for q in self.quads if q.reference), None)


@dataclass(frozen=True)
class Design:
    """A reference design as ink: how far each pixel is from the paper."""

    ink: Gray
    corners: Points
    # Where scores compare, leaving out the border, where a fit that is a
    # little off samples the crate around the print.
    window: npt.NDArray[np.bool_]


def load_design(data: bytes) -> Design:
    img = cv2.imdecode(np.frombuffer(data, np.uint8), cv2.IMREAD_UNCHANGED)
    if img is None:
        raise ValueError("reference design is not a valid image")
    if img.ndim == 2:
        img = cv2.cvtColor(img, cv2.COLOR_GRAY2BGR)
    if img.shape[2] == 4:
        # Transparent parts are bare paper.
        alpha = img[..., 3:].astype(np.float32) / 255
        img = (img[..., :3] * alpha + 255 * (1 - alpha)).astype(np.uint8)
    rgb = np.asarray(cv2.cvtColor(img, cv2.COLOR_BGR2RGB), dtype=np.uint8)
    paper = np.full(rgb.shape[:2], 255, np.uint8)
    full = ink(rgb, paper, white=np.array([255, 255, 255], np.float32))
    s = DESIGN_SIDE
    small = np.asarray(cv2.resize(full, (s, s), interpolation=cv2.INTER_AREA), dtype=np.uint8)
    window = np.zeros((s, s), bool)
    m = s // 20
    window[m : s - m, m : s - m] = True
    return Design(
        ink=small,
        corners=np.array([[0, 0], [s, 0], [s, s], [0, s]], np.float32),
        window=window,
    )


def ink(
    rgb: Frame, paper: Gray, white: npt.NDArray[np.float32] | None = None
) -> npt.NDArray[np.uint8]:
    """Each pixel's distance from the paper's color.

    Under water the paper is tinted blue-green and the ink loses its hue, so
    gray levels hardly separate a yellow flame from the paper. Distance from
    the paper's own color does, whatever the tint.
    """
    f = rgb.astype(np.float32)
    if white is None:
        white = np.median(f[paper > 0], axis=0)
    return np.asarray(np.clip(np.linalg.norm(f - white, axis=2), 0, 255), np.uint8)


def ink_on(rgb: Frame, paper: Gray, area: Gray) -> Gray:
    """ink() inside the area's bounds only, which is all a score looks at.
    The whole frame takes longer than the rest of finding the print."""
    x, y, w, h = cv2.boundingRect(area)
    out = np.zeros(area.shape, np.uint8)
    out[y : y + h, x : x + w] = ink(rgb[y : y + h, x : x + w], paper[y : y + h, x : x + w])
    return out


def paper_in(gray: Gray, mask: Mask) -> Gray | None:
    """The brightest region inside the object's mask, which is the paper its
    design is printed on."""
    inside = cv2.erode(mask.astype(np.uint8), np.ones((3, 3), np.uint8)) > 0
    values = gray[inside]
    if values.size < MIN_PAPER:
        return None
    t, _ = cv2.threshold(values.reshape(-1, 1), 0, 255, cv2.THRESH_BINARY + cv2.THRESH_OTSU)
    bright = ((gray > t) & inside).astype(np.uint8) * 255
    k = max(3, int(np.sqrt(values.size) / 25)) | 1
    bright = np.asarray(
        cv2.morphologyEx(bright, cv2.MORPH_CLOSE, np.ones((k, k), np.uint8)), np.uint8
    )
    contours, _ = cv2.findContours(bright, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_NONE)
    outline = largest(contours)
    if outline is None or cv2.contourArea(outline) < MIN_PAPER:
        return None
    paper = np.zeros_like(bright)
    cv2.drawContours(paper, [outline.astype(np.int32).reshape(-1, 1, 2)], -1, 255, cv2.FILLED)
    return np.asarray(paper & bright, dtype=np.uint8)


def print_area(bright: Gray) -> Gray:
    """Where the design is compared with the frame: the bright paper's convex
    hull, which takes in the ink it surrounds. The ink is too dark to count
    as bright paper, and where it reaches the paper's edge it isn't a hole in
    it either, so without the hull a print would be matched on bare paper
    alone."""
    k = max(3, int(np.sqrt(np.count_nonzero(bright)) * AREA_OPEN)) | 1
    kernel = cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (k, k))
    body = np.asarray(cv2.morphologyEx(bright, cv2.MORPH_OPEN, kernel), np.uint8)
    out = np.zeros_like(bright)
    contours, _ = cv2.findContours(
        body if body.any() else bright, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_NONE
    )
    outline = largest(contours)
    if outline is not None:
        cv2.fillConvexPoly(out, cv2.convexHull(outline.astype(np.int32)), (255,))
    return out


def sheet_in(mask: Mask, bright: Gray, segment: Segment) -> Gray | None:
    """The whole sheet of paper the bright paper is part of, as SAM segments it
    from a box around the bright paper, or None when SAM found something else.

    Paper in the crate's shadow, or curled up toward its rim, is too dark to
    count as bright, though it's the same sheet, so SAM follows its edge where
    brightness stops short of it.
    """
    h, w = bright.shape
    x, y, bw, bh = cv2.boundingRect(bright)
    px, py = SHEET_PAD * bw, SHEET_PAD * bh
    x0, y0 = max(0.0, x - px), max(0.0, y - py)
    x1, y1 = min(float(w), x + bw + px), min(float(h), y + bh + py)
    found = segment((x0 / w, y0 / h, (x1 - x0) / w, (y1 - y0) / h))
    if found is None:
        return None
    contours, _ = cv2.findContours(
        (found & mask).astype(np.uint8), cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_NONE
    )
    outline = largest(contours)
    if outline is None:
        return None
    sheet = np.zeros_like(bright)
    cv2.drawContours(sheet, [outline.astype(np.int32).reshape(-1, 1, 2)], -1, (255,), cv2.FILLED)
    covered = np.count_nonzero(sheet & bright)
    if covered < SHEET_COVER * np.count_nonzero(bright):
        return None
    if np.count_nonzero(sheet) > SHEET_MAX * np.count_nonzero(mask):
        return None
    return sheet


def homography(design: Design, corners: Points) -> npt.NDArray[np.float64] | None:
    try:
        h = np.asarray(cv2.getPerspectiveTransform(design.corners, corners), np.float64)
    except cv2.error:
        return None
    if not np.isfinite(h).all() or abs(np.linalg.det(h)) < 1e-12:
        return None
    return h


def on_paper(design: Design, h: npt.NDArray[np.float64], area: Gray) -> npt.NDArray[np.bool_]:
    """The design's pixels that land on the area under h."""
    s = DESIGN_SIDE
    warped = cv2.warpPerspective(
        area, np.linalg.inv(h), (s, s), flags=cv2.INTER_NEAREST, borderValue=0
    )
    return np.asarray(warped > 0)


def score(img: Gray, corners: Points, design: Design, area: Gray) -> float:
    """How well the design under the corners matches the frame where it shows.

    The corners come from the sheet's outline as they are. Aligning the design
    to the frame's ink, by ECC or by searching for the best score, moves them
    away from where labelers put them: the ink's shading under water differs
    from the design's enough that the best match is a stretched one.
    """
    h = homography(design, corners)
    if h is None:
        return -1.0
    seen = on_paper(design, h, area) & design.window
    if seen.mean() < MIN_COVER:
        return -1.0
    s = DESIGN_SIDE
    rect = cv2.warpPerspective(img, np.linalg.inv(h), (s, s), flags=cv2.INTER_LINEAR)
    return ncc(np.asarray(rect, np.uint8), design.ink, seen)


def shift(a: Points, b: Points) -> float:
    """The furthest corner move from b to a, as a share of b's size."""
    size = np.sqrt(abs(cv2.contourArea(b.astype(np.float32)))) + 1e-9
    return float(np.linalg.norm(a - b, axis=1).max() / size)


def turned(img: Gray, q: Points, design: Design, area: Gray) -> tuple[Points, float, float]:
    """q starting from whichever corner best matches the design's top left,
    its score, and how far ahead of the next best turn that is."""
    scores = [score(img, np.roll(q, -k, axis=0), design, area) for k in range(4)]
    k = int(np.argmax(scores))
    runner_up = max(s for i, s in enumerate(scores) if i != k)
    return np.roll(q, -k, axis=0), scores[k], scores[k] - runner_up


def paper_of(gray: Gray, mask: Mask, segment: Segment | None) -> tuple[Gray, Gray] | None:
    """The bright paper inside the object's mask, and the whole sheet: SAM's
    when it found one, else the bright paper again.

    The sheet's outline places the corners. The design is matched only on
    the bright paper, since ink is told apart by its distance from the
    paper's color, and the sheet's shaded paper would count as ink.
    """
    bright = paper_in(gray, mask)
    if bright is None:
        return None
    sheet = None if segment is None else sheet_in(mask, bright, segment)
    return bright, bright if sheet is None else sheet


def find_print(
    rgb: Frame,
    gray: Gray,
    mask: Mask,
    design: Design,
    guess: Points | None,
    segment: Segment | None = None,
) -> tuple[Points, Gray] | None:
    """The print's corners in design order and the paper that shows, or None.

    guess, where the print was tracked to, is pulled toward a fresh find from
    the sheet's outline, or replaced by it when that matches clearly better,
    so neither a tracking slip nor paper curled up a wall decides alone.
    """
    papers = paper_of(gray, mask, segment)
    if papers is None:
        return None
    bright, sheet = papers
    area = print_area(bright)
    img = ink_on(rgb, bright, area)
    outline = largest(cv2.findContours(sheet, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_NONE)[0])
    if guess is None:
        if outline is None:
            return None
        corners, s, margin = turned(img, quad(outline), design, area)
        if s < MIN_FOUND_SCORE or margin < MIN_TURN_MARGIN or clipped(corners, sheet.shape):
            return None
        return corners, sheet
    best, best_score = guess, score(img, guess, design, area)
    # A print can't turn a quarter between frames, so the fresh find starts
    # from the corner nearest the tracked print's first.
    fresh = None if outline is None else nearest_turn(quad(outline), guess)
    if fresh is not None and shift(fresh, guess) <= MAX_SHIFT:
        corners, s = fresh, score(img, fresh, design, area)
        growth = quad_area(corners) / max(quad_area(guess), 1.0)
        if s > best_score + FRESH_MARGIN:
            best, best_score = corners, s
        elif MIN_FRESH_AREA <= growth <= MAX_FRESH_AREA:
            # Not by score: the tracked print mostly outscores the fresh find
            # even while it drifts away from the print's corners.
            best = np.asarray(guess + PULL * (corners - guess), np.float32)
            best_score = score(img, best, design, area)
    if best_score < MIN_SCORE:
        return None
    return best, sheet


def nearest_turn(q: Points, to: Points) -> Points:
    turns = [np.roll(q, -k, axis=0) for k in range(4)]
    return min(turns, key=lambda t: float(np.linalg.norm(t - to, axis=1).max()))


def quad_area(corners: Points) -> float:
    return abs(float(cv2.contourArea(corners.astype(np.float32))))


def clipped(corners: Points, shape: tuple[int, ...]) -> bool:
    """Whether a quad is too small to place its corners, or is the part of a
    print the frame's edge cuts off rather than the print, whose corners would
    lie beyond the edge."""
    h, w = shape[:2]
    x, y = corners[:, 0], corners[:, 1]
    on_edge = (x <= EDGE_PIXELS) | (x >= w - 1 - EDGE_PIXELS)
    on_edge |= (y <= EDGE_PIXELS) | (y >= h - 1 - EDGE_PIXELS)
    return bool(on_edge.any()) or np.sqrt(abs(cv2.contourArea(corners))) < MIN_SIDE


def print_visibility(corners: Points, paper: Gray) -> list[int]:
    """Visible for corners with paper just inside them, hidden for the rest."""
    h, w = paper.shape
    center = corners.mean(axis=0)
    out: list[int] = []
    for p in corners:
        x, y = (p + (center - p) * INSET).round().astype(int)
        inside = 0 <= p[0] < w and 0 <= p[1] < h and 0 <= x < w and 0 <= y < h
        out.append(VISIBLE if inside and paper[y, x] > 0 else HIDDEN)
    return out


def motion(
    prev: Gray, gray: Gray, prev_mask: Mask, mask: Mask | None
) -> npt.NDArray[np.float64] | None:
    """The object's motion between frames as a homography, from features
    that flow inside its mask and back to where they started.

    Features on the floor around the object are left out. The floor sits
    lower than a crate's rim, so it moves less as the camera moves, and
    following it drags the rim's keypoints down the crate's walls.
    """
    region = cv2.erode(prev_mask.astype(np.uint8) * 255, np.ones((7, 7), np.uint8))
    p0 = cv2.goodFeaturesToTrack(prev, MAX_FEATURES, 0.01, 5, mask=region)
    if p0 is not None and len(p0) >= MIN_FEATURES:
        p1, ok1, _ = cv2.calcOpticalFlowPyrLK(
            prev, gray, p0, np.empty_like(p0), winSize=(21, 21), maxLevel=4
        )
        back, ok2, _ = cv2.calcOpticalFlowPyrLK(
            gray, prev, p1, np.empty_like(p1), winSize=(21, 21), maxLevel=4
        )
        good = (
            (ok1[:, 0] == 1)
            & (ok2[:, 0] == 1)
            & (np.linalg.norm((back - p0).reshape(-1, 2), axis=1) < MAX_ROUND_TRIP)
        )
        if good.sum() >= MIN_FEATURES:
            h, _ = cv2.findHomography(p0[good], p1[good], cv2.RANSAC, RANSAC_PIXELS)
            if h is not None and np.isfinite(h).all():
                return np.asarray(h, np.float64)
    if mask is None:
        return None
    return box_motion(prev_mask, mask)


def box_motion(prev_mask: Mask, mask: Mask) -> npt.NDArray[np.float64] | None:
    """Scale and shift taking one mask's bounds onto the other's, for when the
    object has too little texture to flow."""
    a, b = bounds(prev_mask), bounds(mask)
    if a is None or b is None:
        return None
    sx, sy = b[2] / a[2], b[3] / a[3]
    return np.array([[sx, 0, b[0] - a[0] * sx], [0, sy, b[1] - a[1] * sy], [0, 0, 1]], np.float64)


def bounds(mask: Mask) -> tuple[float, float, float, float] | None:
    ys, xs = np.nonzero(mask)
    if len(xs) == 0:
        return None
    x0, y0 = float(xs.min()), float(ys.min())
    return x0, y0, float(xs.max()) - x0 + 1, float(ys.max()) - y0 + 1


def transform(h: npt.NDArray[np.float64], points: npt.NDArray[np.float64]) -> Points:
    out = cv2.perspectiveTransform(points.reshape(-1, 1, 2).astype(np.float64), h)
    return np.asarray(out.reshape(-1, 2), np.float32)


def snap(corners: Points, mask: Mask, floor: Points | None = None) -> Points:
    """Corners moved onto the mask outline's corners: all four when they line
    up with it, else the ones outside it and, given the print on the crate's
    floor, the ones the move takes away from the print's side.

    Looking straight down, a crate's outline is its rim, and snapping to it
    stops the rim's keypoints drifting. Seen at an angle the outline takes in
    the near walls down to the floor, so the corner between them belongs
    inside it, and snapping it would pull the rim down the wall a little more
    each frame. The rim is never outside the crate's outline though, so a
    corner that drifted out goes back to the outline's nearest corner.

    The other corners stay on the outline at any angle, but flow, following
    the floor and walls below the rim, sinks them inside it as the camera
    closes in. The floor shows shifted toward the near walls, so a corner
    whose snap points away from where the print shifted is climbing back up
    a far wall rather than sliding down a near one.
    """
    contours, _ = cv2.findContours(mask.astype(np.uint8), cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_NONE)
    outline = largest(contours)
    if outline is None:
        return corners
    vertices = quad(outline)
    size = np.sqrt(abs(cv2.contourArea(corners))) + 1e-9
    nearest = [int(np.argmin(np.linalg.norm(vertices - c, axis=1))) for c in corners]
    near = [
        float(np.linalg.norm(vertices[j] - c)) <= SNAP * size
        for j, c in zip(nearest, corners, strict=True)
    ]
    if all(near) and len(set(nearest)) == 4 and moves_as_one(corners, vertices[nearest], size):
        return np.asarray(vertices[nearest], np.float32)
    out = corners.copy()
    hull = outline.reshape(-1, 1, 2)
    shift = None if floor is None else floor.mean(axis=0) - corners.mean(axis=0)
    for i, c in enumerate(corners):
        if not near[i]:
            continue
        target = vertices[nearest[i]]
        outside = cv2.pointPolygonTest(hull, (float(c[0]), float(c[1])), True) < -OUTSIDE_PIXELS
        rising = shift is not None and float((target - c) @ shift) <= 0
        if outside or rising:
            out[i] = target
    return out


def moves_as_one(corners: Points, target: Points, size: float) -> bool:
    """Whether moving corners onto target moves the quad as a whole: shifts,
    turns, or scales it alike in every direction. A near wall the outline
    takes in stretches it one way only."""
    fit, _ = cv2.estimateAffinePartial2D(
        corners.reshape(-1, 1, 2), target.reshape(-1, 1, 2), method=cv2.LMEDS
    )
    if fit is None:
        return False
    moved = cv2.transform(corners.reshape(-1, 1, 2), fit).reshape(4, 2)
    return bool(np.linalg.norm(moved - target, axis=1).max() <= SNAP_SHAPE * size)


def center(corners: Points) -> npt.NDArray[np.float64] | None:
    """Where the quad's diagonals cross, its center in perspective."""
    a, b, c, d = corners.astype(np.float64)
    r, s = c - a, d - b
    denom = r[0] * s[1] - r[1] * s[0]
    if abs(denom) < 1e-9:
        return None
    t = ((b[0] - a[0]) * s[1] - (b[1] - a[1]) * s[0]) / denom
    return np.asarray(a + t * r, np.float64)


def place_centers(points: Keypoints, schema: Schema) -> None:
    for q in schema.quads:
        if q.center is None:
            continue
        idx = list(q.corners)
        v = points[idx, 2]
        c = center(points[idx, :2].astype(np.float32)) if (v > UNLABELED).all() else None
        if c is None:
            points[q.center] = (0, 0, UNLABELED)
        else:
            points[q.center] = (c[0], c[1], HIDDEN if (v == HIDDEN).all() else VISIBLE)


def fill_quad(
    rgb: Frame, mask: Mask, design: Design | None, segment: Segment | None = None
) -> tuple[Points, list[int]] | None:
    """A quad's corners from a mask: in the design's order when the reference
    design is printed on it, else clockwise from the top left on screen.

    For a printed quad the mask is the whole object's, and the print is the
    paper inside it. The labeler pointed at it, so the design's best quarter
    turn is taken even when it scores too low to have been found unprompted.
    """
    gray = np.asarray(cv2.cvtColor(rgb, cv2.COLOR_RGB2GRAY), np.uint8)
    if design is not None:
        papers = paper_of(gray, mask, segment)
        if papers is not None:
            bright, sheet = papers
            contours, _ = cv2.findContours(sheet, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_NONE)
            outline = largest(contours)
            if outline is not None:
                area = print_area(bright)
                corners, _, _ = turned(ink_on(rgb, bright, area), quad(outline), design, area)
                return corners, print_visibility(corners, sheet)
    contours, _ = cv2.findContours(mask.astype(np.uint8), cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_NONE)
    outline = largest(contours)
    if outline is None:
        return None
    q = quad(outline)
    first = int(np.argmin(q.sum(axis=1)))
    return np.roll(q, -first, axis=0), [VISIBLE] * 4


UNIT_SQUARE = np.array([[0, 0], [1, 0], [1, 1], [0, 1]], np.float32)


def surface(points: Keypoints, q: Quad) -> npt.NDArray[np.float64] | None:
    """The map from the unit square onto quad q, when all its corners are
    labeled."""
    idx = list(q.corners)
    if not (points[idx, 2] > UNLABELED).all():
        return None
    corners = points[idx, :2].astype(np.float32)
    if abs(cv2.contourArea(corners)) < 1:
        return None
    return np.asarray(cv2.getPerspectiveTransform(UNIT_SQUARE, corners), np.float64)


class KeypointTracker:
    """Carries keypoints from frame to frame along with the object's mask.

    Where keypoints off the reference quad sit on its surface is learned from
    the frames a labeler labeled. Flow carries them, since a crate's rim sits
    far enough above its print to shift against it as the camera moves, but
    ones flow loses, or that were only labeled on earlier frames, are put back
    from where they sat on the print.
    """

    def __init__(
        self,
        schema: Schema,
        design: Design | None,
        start: Keypoints,
        earlier: Sequence[Keypoints] = (),
    ) -> None:
        self.schema = schema
        self.design = design if schema.reference is not None else None
        self.points = start.copy()
        # What the labeler set, which hidden keypoints return to once they're
        # back in the frame.
        self.given = start[:, 2].copy()
        self.prev: tuple[Gray, Mask] | None = None
        # Keypoints off the reference quad, by where they sit on its unit
        # square.
        self.riding: dict[int, npt.NDArray[np.float64]] = {}
        for labeled in earlier:
            self.learn(labeled)
        self.learn(start)

    def learn(self, labeled: Keypoints, only_new: bool = False) -> None:
        """Remember where labeled keypoints sit on the reference quad."""
        ref = self.schema.reference
        h = None if ref is None else surface(labeled, ref)
        if ref is None or h is None:
            return
        on_quad = {*ref.corners, ref.center}
        inverse = np.linalg.inv(h)
        for i in np.flatnonzero(labeled[:, 2] > UNLABELED):
            if int(i) in on_quad or (only_new and int(i) in self.riding):
                continue
            self.riding[int(i)] = transform(inverse, labeled[i : i + 1, :2])[0].astype(np.float64)
            if self.given[i] == UNLABELED:
                self.given[i] = labeled[i, 2]

    def fill(self, rgb: Frame, index: int, mask: Mask, segment: Segment | None = None) -> None:
        """Place quad index's corners from the mask fill_quad takes."""
        q = self.schema.quads[index]
        found = fill_quad(rgb, mask, self.design if q.reference else None, segment)
        if found is None:
            return
        corners, vis = found
        for i, c, v in zip(q.corners, corners, vis, strict=True):
            self.points[i] = (c[0], c[1], v)
            self.given[i] = VISIBLE
        place_centers(self.points, self.schema)
        self.learn(self.points)

    def anchor(self, labeled: Keypoints) -> None:
        """Take the labeler's keypoints on this frame over the tracked ones,
        so tracking carries on from what they placed."""
        placed = labeled[:, 2] > UNLABELED
        self.points[placed] = labeled[placed]
        self.given[placed] = labeled[placed, 2]
        self.learn(self.points)

    def step(self, rgb: Frame, mask: Mask | None, segment: Segment | None = None) -> Keypoints:
        """The keypoints on the next frame, in pixels."""
        gray = np.asarray(cv2.cvtColor(rgb, cv2.COLOR_RGB2GRAY), np.uint8)
        before = self.points.copy()
        labeled = self.points[:, 2] > UNLABELED
        if self.prev is not None and labeled.any():
            prev_gray, prev_mask = self.prev
            h = motion(prev_gray, gray, prev_mask, mask)
            if h is None:
                self.points[:, 2] = UNLABELED
            else:
                self.points[labeled, :2] = transform(h, self.points[labeled, :2])
        if mask is None or not mask.any():
            self.prev = None
            self.points[:, 2] = UNLABELED
            return self.points.copy()
        self._lose_strays(mask)
        self._lose_folded(before)
        ref = self.schema.reference
        if ref is not None and self.design is not None:
            self._find_print(rgb, gray, mask, ref, segment)
        self._ride()
        floor = None
        if ref is not None and (self.points[list(ref.corners), 2] > UNLABELED).all():
            floor = self.points[list(ref.corners), :2].astype(np.float32)
        for q in self.schema.quads:
            if q is ref or not (self.points[list(q.corners), 2] > UNLABELED).all():
                continue
            idx = list(q.corners)
            self.points[idx, :2] = snap(self.points[idx, :2].astype(np.float32), mask, floor)
        self._out_of_frame(gray.shape)
        place_centers(self.points, self.schema)
        self.prev = (gray, mask)
        return self.points.copy()

    def _find_print(
        self, rgb: Frame, gray: Gray, mask: Mask, q: Quad, segment: Segment | None
    ) -> None:
        assert self.design is not None
        idx = list(q.corners)
        tracked = (self.points[idx, 2] > UNLABELED).all()
        guess = self.points[idx, :2].astype(np.float32) if tracked else None
        found = find_print(rgb, gray, mask, self.design, guess, segment)
        if found is None:
            # A tracked print stays where the motion took it, which is how
            # prints mostly hidden by the crate stay labeled.
            return
        corners, paper = found
        for i, c, v in zip(idx, corners, print_visibility(corners, paper), strict=True):
            self.points[i] = (c[0], c[1], v)
            self.given[i] = VISIBLE

    def _ride(self) -> None:
        ref = self.schema.reference
        h = None if ref is None else surface(self.points, ref)
        if h is None:
            return
        # Keypoints labeled before the print was first found learn their spot
        # from where the motion has carried them.
        self.learn(self.points, only_new=True)
        if not self.riding:
            return
        idx = [i for i in self.riding if self.points[i, 2] == UNLABELED]
        if not idx:
            return
        on_square = np.array([self.riding[i] for i in idx])
        self.points[idx, :2] = transform(h, on_square)
        self.points[idx, 2] = self.given[idx]

    def _lose_strays(self, mask: Mask) -> None:
        b = bounds(mask)
        if b is None:
            return
        x, y, w, h = b
        reach = LOST * max(w, h)
        p = self.points
        stray = (
            (p[:, 0] < x - reach)
            | (p[:, 0] > x + w + reach)
            | (p[:, 1] < y - reach)
            | (p[:, 1] > y + h + reach)
        )
        p[stray, 2] = UNLABELED

    def _lose_folded(self, before: Keypoints) -> None:
        for q in self.schema.quads:
            idx = list(q.corners)
            if not (self.points[idx, 2] > UNLABELED).all():
                continue
            was = abs(cv2.contourArea(before[idx, :2].astype(np.float32)))
            now = cv2.contourArea(self.points[idx, :2].astype(np.float32), oriented=True)
            # Corners go clockwise on screen, so a quad that turns
            # counterclockwise has flipped over.
            if now <= 0 or was <= 0 or not 1 / MAX_GROWTH <= now / was <= MAX_GROWTH:
                self.points[idx, 2] = UNLABELED

    def _out_of_frame(self, shape: tuple[int, ...]) -> None:
        h, w = shape[:2]
        p = self.points
        labeled = p[:, 2] > UNLABELED
        outside = (p[:, 0] < 0) | (p[:, 0] >= w) | (p[:, 1] < 0) | (p[:, 1] >= h)
        ref = self.schema.reference
        printed = set(ref.corners) if ref is not None and self.design is not None else set()
        for i in np.flatnonzero(labeled):
            if outside[i]:
                p[i, 2] = HIDDEN
            elif i not in printed:
                p[i, 2] = self.given[i]


def from_proto(
    keypoints: Sequence[annotation_pb2.Keypoint], count: int, shape: tuple[int, ...]
) -> Keypoints:
    """Keypoints in pixels, padded with unlabeled ones to count."""
    h, w = shape[:2]
    out = np.zeros((count, 3), np.float64)
    for i, k in enumerate(keypoints[:count]):
        if k.visibility != UNLABELED:
            out[i] = (k.x * w, k.y * h, k.visibility)
    return out


def to_proto(points: Keypoints, shape: tuple[int, ...]) -> list[annotation_pb2.Keypoint]:
    h, w = shape[:2]
    out = []
    for x, y, v in points:
        if int(v) == UNLABELED:
            out.append(annotation_pb2.Keypoint())
        else:
            out.append(
                annotation_pb2.Keypoint(
                    x=float(np.clip(x / w, -1, 2)),
                    y=float(np.clip(y / h, -1, 2)),
                    visibility=VISIBLE if int(v) == VISIBLE else HIDDEN,
                )
            )
    return out
