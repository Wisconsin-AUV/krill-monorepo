"""Rotated boxes as the API stores them: four normalized corners, clockwise on
screen, starting from the corner nearest the frame's top left."""

from collections.abc import Sequence

import cv2
import numpy as np
import numpy.typing as npt
from krill.v1 import annotation_pb2

from krill_worker.prelabelers import Point
from krill_worker.track import Box

Corners = tuple[Point, Point, Point, Point]


def ordered(points: npt.ArrayLike) -> Corners:
    """Order a rectangle's four normalized corners clockwise on screen, starting
    from the one with the smallest x + y, with ties going to the smaller y."""
    pts = np.asarray(points, dtype=np.float64).reshape(4, 2)
    x, y = pts[:, 0], pts[:, 1]
    # With y pointing down, a positive shoelace sum is clockwise on screen.
    if float(np.sum(x * np.roll(y, -1) - np.roll(x, -1) * y)) < 0:
        pts = pts[::-1]
    start = min(range(4), key=lambda i: (pts[i, 0] + pts[i, 1], pts[i, 1]))
    pts = np.roll(pts, -start, axis=0)
    a, b, c, d = ((float(px), float(py)) for px, py in pts)
    return a, b, c, d


def rotated_rect(points: npt.ArrayLike, shape: Sequence[int]) -> Corners:
    """The tightest rotated rectangle around pixel points on a frame of shape."""
    h, w = shape[:2]
    pts = np.asarray(points, dtype=np.float32).reshape(-1, 2)
    corners = np.asarray(cv2.boxPoints(cv2.minAreaRect(pts))) / np.array([w, h])
    return ordered(corners)


def bounds(corners: Corners) -> Box:
    """The corners' axis-aligned bounds, clipped to the frame."""
    xs = np.clip([p[0] for p in corners], 0, 1)
    ys = np.clip([p[1] for p in corners], 0, 1)
    x0, y0 = float(xs.min()), float(ys.min())
    return x0, y0, float(xs.max()) - x0, float(ys.max()) - y0


def iou(a: Box, b: Box) -> float:
    ax, ay, aw, ah = a
    bx, by, bw, bh = b
    iw = min(ax + aw, bx + bw) - max(ax, bx)
    ih = min(ay + ah, by + bh) - max(ay, by)
    if iw <= 0 or ih <= 0:
        return 0.0
    inter = iw * ih
    return inter / (aw * ah + bw * bh - inter)


def to_proto(box: Box, corners: Corners | None = None) -> annotation_pb2.Box:
    """A box message, rotated when corners are given, with its axis-aligned
    box then their bounds."""
    if corners is None:
        x, y, w, h = box
        return annotation_pb2.Box(x=x, y=y, width=w, height=h)
    x, y, w, h = bounds(corners)
    points = [annotation_pb2.Point(x=px, y=py) for px, py in corners]
    return annotation_pb2.Box(x=x, y=y, width=w, height=h, corners=points)
