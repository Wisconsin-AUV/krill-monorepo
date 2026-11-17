from copy import deepcopy
from pathlib import Path
from types import SimpleNamespace

import cv2
import numpy as np
from krill.v1 import export_pb2

from krill_worker import augment

EVERYTHING = export_pb2.Augmentation(aspect=1.5, underwater=1, shadows=1, noise=1)


def test_vary_leaves_the_cached_image_alone() -> None:
    im = np.random.default_rng(0).integers(0, 256, (90, 160, 3), dtype=np.uint8)
    before = im.copy()
    for seed in range(20):
        augment.vary(im, EVERYTHING, np.random.default_rng(seed))
    assert np.array_equal(im, before)


def test_stretch_only_shrinks_one_side_within_the_ratio() -> None:
    im = np.zeros((360, 640, 3), np.uint8)
    for seed in range(50):
        h, w = augment.stretch(im, np.random.default_rng(seed), 1.5).shape[:2]
        assert (h == 360) != (w == 640)
        assert h <= 360 and w <= 640
        assert 640 / 360 / 1.5 - 0.01 <= w / h <= 640 / 360 * 1.5 + 0.01


def test_overlapping_shadows_stay_within_the_patch_gain() -> None:
    gray = np.full((60, 80, 3), 200, np.uint8)
    lo, hi = 200 * (1 - augment.MAX_PATCH_GAIN), 200 * (1 + augment.MAX_PATCH_GAIN)
    for seed in range(200):
        out = augment.shadows(gray, np.random.default_rng(seed)).astype(int)
        assert out.min() >= int(lo) and out.max() <= min(255, round(hi))


def test_underwater_takes_red_away_not_blue() -> None:
    gray = np.full((8, 8, 3), 128, np.uint8)
    for seed in range(20):
        b, _, r = augment.underwater(gray, np.random.default_rng(seed)).mean(axis=(0, 1))
        assert b > r


RED, GREEN = (0, 0, 255), (0, 255, 0)
BLACK = np.zeros((120, 200, 3), np.uint8)
DISTRACTOR = np.full((50, 70, 3), GREEN, np.uint8)


def red(im: np.ndarray, polygons: list[np.ndarray]) -> np.ndarray:
    im = im.copy()
    cv2.fillPoly(im, [np.round(p).astype(np.int32) for p in polygons], RED)
    return im


def mask(shape: tuple[int, ...], polygon: np.ndarray) -> np.ndarray:
    m = np.zeros(shape[:2], np.uint8)
    cv2.fillPoly(m, [np.round(polygon).astype(np.int32)], 1)
    return m


def interior(shape: tuple[int, ...], polygon: np.ndarray) -> np.ndarray:
    """The polygon's pixels away from its edge, which resampling blurs."""
    m = cv2.erode(
        mask(shape, polygon),
        np.ones((5, 5), np.uint8),
        borderType=cv2.BORDER_CONSTANT,
        borderValue=0,
    )
    return np.asarray(m > 0)


def test_pasted_labels_land_on_the_pasted_pixels() -> None:
    diamond = np.array([[60, 10], [100, 50], [60, 90], [20, 50]], np.float32)
    square = np.array([[130, 30], [180, 30], [180, 80], [130, 80]], np.float32)
    im = red(BLACK, [diamond, square])
    before, distractor, unmoved = im.copy(), DISTRACTOR.copy(), square.copy()
    for polygon in (diamond, square):
        for seed in range(20):
            out, pasted, kept = augment.paste(
                im, [polygon], distractor, np.random.default_rng(seed)
            )
            assert kept == [0]
            p = pasted[0]
            # Ultralytics resizes and rotates the whole image afterwards, so
            # paste only moves objects.
            shift = p - polygon
            assert np.allclose(shift, shift[0])
            assert (p >= 0).all() and (p[:, 0] <= 200).all() and (p[:, 1] <= 120).all()
            inside = interior(out.shape, p)
            assert inside.sum() > 0
            assert (np.abs(out[inside].astype(int) - RED) <= 2).all()
            k = 2 * augment.FEATHER + 3
            outside = cv2.dilate(mask(out.shape, p), np.ones((k, k), np.uint8)) == 0
            assert (out[outside] == GREEN).all()
    assert np.array_equal(im, before) and np.array_equal(distractor, DISTRACTOR)
    assert np.array_equal(square, unmoved)


def test_paste_drops_what_cannot_be_placed_without_overlap() -> None:
    boxes = [
        np.array([[x, y], [x + 55, y], [x + 55, y + 55], [x, y + 55]], np.float32)
        for x in (0, 70, 140)
        for y in (0, 60)
    ]
    im = red(BLACK, boxes)
    dropped = 0
    for seed in range(20):
        _, pasted, kept = augment.paste(im, boxes, DISTRACTOR, np.random.default_rng(seed))
        dropped += len(boxes) - len(kept)
        masks = [mask(im.shape, p) for p in pasted]
        for i, a in enumerate(masks):
            for b in masks[i + 1 :]:
                overlap = (a & b).sum()
                assert overlap <= augment.MAX_PASTE_OVERLAP * min(a.sum(), b.sum())
    assert dropped > 0


def test_paste_without_labels_is_the_distractor() -> None:
    out, pasted, kept = augment.paste(BLACK, [], DISTRACTOR, np.random.default_rng(0))
    assert out.shape == BLACK.shape and (out == GREEN).all()
    assert pasted == [] and kept == []


class Instances:
    """The parts of Ultralytics' Instances that the wrapper relies on."""

    def __init__(
        self,
        bboxes: np.ndarray,
        segments: np.ndarray | None = None,
        keypoints: np.ndarray | None = None,
        bbox_format: str = "xywh",
        normalized: bool = True,
    ) -> None:
        self._bboxes = SimpleNamespace(format=bbox_format)
        self.bboxes = bboxes
        self.segments = segments if segments is not None else np.zeros((0, 0, 2), np.float32)
        self.keypoints = keypoints
        self.normalized = normalized

    def convert_bbox(self, format: str) -> None:
        b = self.bboxes
        if (self._bboxes.format, format) == ("xywh", "xyxy"):
            self.bboxes = np.concatenate([b[:, :2] - b[:, 2:] / 2, b[:, :2] + b[:, 2:] / 2], 1)
        elif (self._bboxes.format, format) == ("xyxy", "xywh"):
            self.bboxes = np.concatenate([(b[:, :2] + b[:, 2:]) / 2, b[:, 2:] - b[:, :2]], 1)
        self._bboxes.format = format

    def denormalize(self, w: int, h: int) -> None:
        if self.normalized:
            self.bboxes = self.bboxes * (w, h, w, h)
            self.segments = self.segments * (w, h)
            self.normalized = False

    def normalize(self, w: int, h: int) -> None:
        if not self.normalized:
            self.bboxes = self.bboxes / (w, h, w, h)
            self.segments = self.segments / (w, h)
            self.normalized = True


def test_wrapper_relabels_detect_and_obb_objects_where_they_were_pasted(
    tmp_path: Path,
) -> None:
    # Normalized to the 400x240 original; loaded at 200x120 like Ultralytics.
    im = red(BLACK, [np.array([[40, 30], [80, 30], [80, 90], [40, 90]], np.float32)])
    labels = {
        "detect": Instances(np.array([[0.3, 0.5, 0.2, 0.5]], np.float32)),
        "obb": Instances(
            np.array([[0.3, 0.5, 0.2, 0.5]], np.float32),
            # Ultralytics resamples OBB corners along the outline.
            np.array([[[0.2, 0.25], [0.4, 0.25], [0.4, 0.75], [0.2, 0.75], [0.2, 0.5]]]),
        ),
        "empty": Instances(np.zeros((0, 4), np.float32)),
    }

    def get(task: str) -> dict[str, object]:
        n = len(labels[task].bboxes)
        return {
            "img": im,
            "cls": np.ones((n, 1), np.float32),
            "instances": deepcopy(labels[task]),
            "ori_shape": (240, 400),
            "resized_shape": (120, 200),
        }

    distractor = tmp_path / "d.png"
    cv2.imwrite(str(distractor), DISTRACTOR)
    dataset = SimpleNamespace(get_image_and_label=get)
    load = augment.varied(dataset, export_pb2.Augmentation(distractors=1, degrees=45), [distractor])

    for task in ("detect", "obb"):
        label = load(task)
        out, instances = label["img"], label["instances"]
        assert instances._bboxes.format == "xywh" and instances.normalized
        assert label["cls"].shape == (1, 1)
        x, y, w, h = instances.bboxes[0] * (200, 120, 200, 120)
        assert (out[round(y), round(x)] == RED).all()
        m = augment.FEATHER + 2
        box = np.zeros(out.shape[:2], bool)
        box[
            max(0, round(y - h / 2) - m) : round(y + h / 2) + m,
            max(0, round(x - w / 2) - m) : round(x + w / 2) + m,
        ] = True
        assert (out[~box] == GREEN).all()
        if task == "obb":
            corners = instances.segments[0] * (200, 120)
            inside = interior(out.shape, corners)
            assert (np.abs(out[inside].astype(int) - RED) <= 2).all()

    label = load("empty")
    assert (label["img"] == GREEN).all() and len(label["cls"]) == 0
