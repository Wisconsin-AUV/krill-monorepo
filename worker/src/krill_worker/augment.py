"""Krill's own training augmentations, applied to each image as it is loaded,
before Ultralytics' mosaic and affine steps.

Every step returns a new array: Ultralytics hands back the same cached image
each time an index is loaded again.
"""

import logging
import random
from collections import OrderedDict
from collections.abc import Callable, Sequence
from pathlib import Path
from typing import Any

import cv2
import numpy as np
import numpy.typing as npt
from krill.v1 import export_pb2

log = logging.getLogger(__name__)

Image = npt.NDArray[np.uint8]
# An object's outline as (x, y) pixel corners in the loaded image.
Outline = npt.NDArray[np.float32]

# BGR, since Ultralytics loads images with OpenCV. Water absorbs red first,
# so gains fall toward red and the haze is blue-green.
WATER_GAIN_LOW = np.array([1.0, 0.95, 0.55])
WATER_GAIN_HIGH = np.array([1.2, 1.1, 0.85])
WATER_HAZE = np.array([120.0, 105.0, 35.0])
MAX_HAZE = 0.3

# Light and shadow patches scale brightness by at most this much either way.
MAX_PATCH_GAIN = 0.45
MAX_PATCHES = 3

NOISE_SIGMA = (2.0, 10.0)

# Pixels around each pasted object over which its surroundings fade into the
# distractor, so its edge isn't a hard cut.
FEATHER = 4
# Pasted objects are at least MIN_PASTE_SIDE pixels on their long side and at
# most MAX_PASTE_FRACTION of the frame either way.
MIN_PASTE_SIDE = 12
MAX_PASTE_FRACTION = 0.9
# A spot is rejected when it covers more than this share of the smaller of the
# new object and an object already pasted there.
MAX_PASTE_OVERLAP = 0.1
PASTE_TRIES = 20

# Decoded distractors kept by each dataloader process, shrunk to this long
# side first.
DISTRACTOR_CACHE_BYTES = 128 << 20
DISTRACTOR_MAX_SIDE = 1280


def stretch(im: Image, rng: np.random.Generator, max_ratio: float) -> Image:
    """Squeeze one side by up to max_ratio. Labels are normalized, so they
    still fit, and shrinking keeps the long side within the training size."""
    ratio = rng.uniform(1.0, max_ratio)
    h, w = im.shape[:2]
    if rng.random() < 0.5:
        w = max(1, round(w / ratio))
    else:
        h = max(1, round(h / ratio))
    return np.asarray(cv2.resize(im, (w, h), interpolation=cv2.INTER_AREA), dtype=np.uint8)


def to_image(a: npt.NDArray[Any]) -> Image:
    return np.asarray(np.clip(a, 0, 255), dtype=np.uint8)


def underwater(im: Image, rng: np.random.Generator) -> Image:
    gain = rng.uniform(WATER_GAIN_LOW, WATER_GAIN_HIGH)
    haze = rng.uniform(0.0, MAX_HAZE)
    out = im.astype(np.float32) * gain
    return to_image(out * (1 - haze) + WATER_HAZE * haze)


def shadows(im: Image, rng: np.random.Generator) -> Image:
    """Soft blobs of light and shadow, like sun through the surface or the
    sub's own shadow."""
    h, w = im.shape[:2]
    gain = np.ones((h, w), np.float32)
    for _ in range(rng.integers(1, MAX_PATCHES + 1)):
        mask = np.zeros((h, w), np.float32)
        center = (int(rng.uniform(0, w)), int(rng.uniform(0, h)))
        axes = (int(rng.uniform(0.1, 0.5) * w), int(rng.uniform(0.1, 0.5) * h))
        cv2.ellipse(mask, center, axes, rng.uniform(0, 180), 0, 360, 1.0, -1)
        blur = (max(axes) // 2) * 2 + 1
        soft = cv2.GaussianBlur(mask, (blur, blur), 0)
        gain += soft * rng.uniform(-MAX_PATCH_GAIN, MAX_PATCH_GAIN)
    return to_image(im.astype(np.float32) * gain[..., None])


def noise(im: Image, rng: np.random.Generator) -> Image:
    sigma = rng.uniform(*NOISE_SIGMA)
    return to_image(im.astype(np.float32) + rng.normal(0.0, sigma, im.shape))


class Distractors:
    """Distractor images on disk, decoded as they are picked."""

    def __init__(self, paths: Sequence[Path]) -> None:
        self.paths = list(paths)
        self.cache: OrderedDict[Path, Image] = OrderedDict()
        self.cached_bytes = 0

    def pick(self, rng: np.random.Generator) -> Image | None:
        while self.paths:
            path = self.paths[rng.integers(len(self.paths))]
            im = self.load(path)
            if im is not None:
                return im
            log.warning("skipping unreadable distractor %s", path.name)
            self.paths.remove(path)
        return None

    def load(self, path: Path) -> Image | None:
        cached = self.cache.get(path)
        if cached is not None:
            self.cache.move_to_end(path)
            return cached
        raw = cv2.imread(str(path), cv2.IMREAD_COLOR)
        if raw is None:
            return None
        h, w = raw.shape[:2]
        r = DISTRACTOR_MAX_SIDE / max(h, w)
        if r < 1:
            raw = cv2.resize(raw, (round(w * r), round(h * r)), interpolation=cv2.INTER_AREA)
        im = np.asarray(raw, dtype=np.uint8)
        self.cache[path] = im
        self.cached_bytes += im.nbytes
        while self.cached_bytes > DISTRACTOR_CACHE_BYTES and len(self.cache) > 1:
            _, old = self.cache.popitem(last=False)
            self.cached_bytes -= old.nbytes
        return im


def cover(src: Image, h: int, w: int, rng: np.random.Generator) -> Image:
    """Scale src to cover h by w, keeping its aspect, and crop a random part."""
    sh, sw = src.shape[:2]
    r = max(h / sh, w / sw)
    rw, rh = max(w, round(sw * r)), max(h, round(sh * r))
    scaled = cv2.resize(src, (rw, rh), interpolation=cv2.INTER_AREA if r < 1 else cv2.INTER_LINEAR)
    x, y = rng.integers(rw - w + 1), rng.integers(rh - h + 1)
    return np.asarray(scaled[y : y + h, x : x + w], dtype=np.uint8)


def recolor(im: Image, saturation: float, value: float) -> Image:
    hsv = cv2.cvtColor(im, cv2.COLOR_BGR2HSV).astype(np.float32)
    hsv[..., 1:] *= (saturation, value)
    return np.asarray(cv2.cvtColor(to_image(hsv), cv2.COLOR_HSV2BGR), dtype=np.uint8)


def paste(
    im: Image,
    polygons: Sequence[Outline],
    distractor: Image,
    aug: export_pb2.Augmentation,
    rng: np.random.Generator,
) -> tuple[Image, list[Outline], list[int]]:
    """Cut each polygon out of im and paste it, resized, rotated, stretched,
    and recolored, at a random free spot on the distractor. Returns the new
    image, the pasted polygons, and the indices of the objects that found a
    spot, in their original order."""
    h, w = im.shape[:2]
    canvas = cover(distractor, h, w, rng).astype(np.float32)
    owner = np.zeros((h, w), np.int32)
    areas = [0]
    placed: dict[int, Outline] = {}
    for i in rng.permutation(len(polygons)):
        p = np.asarray(polygons[i], np.float32).reshape(-1, 2)
        x0, y0 = np.maximum(np.floor(p.min(0)).astype(int) - FEATHER, 0)
        x1, y1 = np.minimum(np.ceil(p.max(0)).astype(int) + FEATHER + 1, (w, h))
        if x1 <= x0 or y1 <= y0:
            continue

        angle = np.radians(rng.uniform(-aug.degrees, aug.degrees))
        squeeze = np.ones(2)
        squeeze[rng.integers(2)] = 1 / rng.uniform(1.0, max(aug.aspect, 1.0))
        rotate = np.array([[np.cos(angle), -np.sin(angle)], [np.sin(angle), np.cos(angle)]])
        m = rotate * squeeze * rng.uniform(max(1 - aug.scale, 0.01), 1 + aug.scale)
        center = (p.min(0) + p.max(0)) / 2
        q = (p - center) @ m.T
        extent = q.max(0) - q.min(0)
        if extent.max() <= 0:
            continue
        fit = max(1.0, MIN_PASTE_SIDE / extent.max())
        fit = min(fit, *(MAX_PASTE_FRACTION * np.array([w, h]) / np.maximum(extent, 1e-6)))
        m, q = m * fit, q * fit

        pad = FEATHER + 1
        offset = pad - q.min(0)
        q = q + offset
        pw, ph = np.ceil(q.max(0)).astype(int) + pad + 1
        # Maps the crop's pixels to the patch's, as q maps the polygon.
        warp = np.hstack([m, ((np.array([x0, y0]) - center) @ m.T + offset)[:, None]])
        patch = cv2.warpAffine(
            im[y0:y1, x0:x1],
            warp.astype(np.float32),
            (int(pw), int(ph)),
            flags=cv2.INTER_LINEAR,
            borderMode=cv2.BORDER_REPLICATE,
        )
        patch = recolor(
            np.asarray(patch, np.uint8),
            rng.uniform(1 - aug.hsv_s, 1 + aug.hsv_s),
            rng.uniform(1 - aug.hsv_v, 1 + aug.hsv_v),
        )
        hard = np.zeros((ph, pw), np.uint8)
        cv2.fillPoly(hard, [np.round(q).astype(np.int32)], 1)
        area = int(hard.sum())
        if area == 0:
            continue
        # Only the blur's outer half is kept, so the blend falls outside the
        # polygon and every labeled pixel is the object's.
        k = 2 * FEATHER + 1
        alpha = np.maximum(cv2.GaussianBlur(hard.astype(np.float32), (k, k), 0), hard)

        lo, hi = q.min(0), q.max(0)
        xs = (int(np.ceil(-lo[0])), int(np.floor(w - hi[0])))
        ys = (int(np.ceil(-lo[1])), int(np.floor(h - hi[1])))
        if xs[1] < xs[0] or ys[1] < ys[0]:
            continue
        for _ in range(PASTE_TRIES):
            ox, oy = int(rng.integers(xs[0], xs[1] + 1)), int(rng.integers(ys[0], ys[1] + 1))
            # The patch clipped to the frame, in frame and patch coordinates.
            fx0, fy0 = max(ox, 0), max(oy, 0)
            fx1, fy1 = min(ox + pw, w), min(oy + ph, h)
            region = (slice(fy0, fy1), slice(fx0, fx1))
            local = (slice(fy0 - oy, fy1 - oy), slice(fx0 - ox, fx1 - ox))
            covered = np.bincount(owner[region][hard[local] > 0], minlength=len(areas))
            if any(c > MAX_PASTE_OVERLAP * min(area, areas[j]) for j, c in enumerate(covered) if j):
                continue
            blend = alpha[local][..., None]
            canvas[region] = patch[local] * blend + canvas[region] * (1 - blend)
            owner[region][hard[local] > 0] = len(areas)
            areas.append(area)
            placed[int(i)] = q + np.array([ox, oy], np.float32)
            break
    kept = sorted(placed)
    return to_image(np.rint(canvas)), [placed[i] for i in kept], kept


def vary(im: Image, aug: export_pb2.Augmentation, rng: np.random.Generator) -> Image:
    if aug.aspect > 1:
        im = stretch(im, rng, aug.aspect)
    if im.ndim != 3 or im.shape[2] != 3:
        return im
    if rng.random() < aug.underwater:
        im = underwater(im, rng)
    if rng.random() < aug.shadows:
        im = shadows(im, rng)
    if rng.random() < aug.noise:
        im = noise(im, rng)
    return im


def ultralytics_args(aug: export_pb2.Augmentation) -> dict[str, float]:
    return {
        "degrees": aug.degrees,
        "translate": aug.translate,
        "scale": aug.scale,
        "shear": aug.shear,
        "perspective": aug.perspective,
        "hsv_h": aug.hsv_h,
        "hsv_s": aug.hsv_s,
        "hsv_v": aug.hsv_v,
        "fliplr": aug.fliplr,
        "flipud": aug.flipud,
        "mosaic": aug.mosaic,
    }


def trainer(
    base: type[Any], aug: export_pb2.Augmentation, distractors: Sequence[Path] = ()
) -> type[Any]:
    """Subclass an Ultralytics trainer so its training images, and never its
    val images, go through paste and vary."""

    class Trainer(base):  # type: ignore[misc]
        def build_dataset(self, img_path: str, mode: str = "train", batch: Any = None) -> Any:
            dataset = super().build_dataset(img_path, mode, batch)
            if mode == "train":
                dataset.get_image_and_label = varied(dataset, aug, distractors)
            return dataset

    return Trainer


def pasted(
    label: dict[str, Any],
    distractor: Image,
    aug: export_pb2.Augmentation,
    rng: np.random.Generator,
) -> None:
    """Paste the objects of an Ultralytics label onto the distractor, keeping
    its instances in the box format and normalization they came in. OBB labels
    carry their corners as segments; detect labels only boxes."""
    im = label["img"]
    h, w = im.shape[:2]
    instances = label["instances"]
    box_format, normalized = instances._bboxes.format, instances.normalized
    instances.convert_bbox("xyxy")
    instances.denormalize(w, h)
    boxes = np.asarray(instances.bboxes, np.float32).reshape(-1, 4)
    segments = np.asarray(instances.segments, np.float32)
    oriented = len(segments) > 0 and len(segments) == len(boxes)
    if oriented:
        polygons = list(segments)
    else:
        polygons = [
            np.array([[x1, y1], [x2, y1], [x2, y2], [x1, y2]], np.float32)
            for x1, y1, x2, y2 in boxes
        ]

    im, polygons, kept = paste(im, polygons, distractor, aug, rng)

    points = np.stack(polygons) if polygons else np.zeros((0, 4, 2), np.float32)
    boxes = np.concatenate([points.min(1), points.max(1)], 1).astype(np.float32)
    if oriented:
        segments = points.astype(np.float32) if polygons else segments[:0]
    out = type(instances)(boxes, segments, None, bbox_format="xyxy", normalized=False)
    out.convert_bbox(box_format)
    if normalized:
        out.normalize(w, h)
    label["img"] = im
    label["cls"] = label["cls"][np.asarray(kept, int)]
    label["instances"] = out


def varied(
    dataset: Any, aug: export_pb2.Augmentation, distractors: Sequence[Path] = ()
) -> Callable[..., dict[str, Any]]:
    """Wrap the dataset's get_image_and_label, which both its own loading and
    Ultralytics' mosaic go through."""
    get: Callable[..., dict[str, Any]] = dataset.get_image_and_label
    library = Distractors(distractors) if distractors and aug.distractors > 0 else None

    def get_image_and_label(index: int, *args: Any, **kwargs: Any) -> dict[str, Any]:
        label = get(index, *args, **kwargs)
        # Ultralytics seeds the stdlib RNG in each dataloader worker, so this
        # differs between workers where a Generator made here would not.
        rng = np.random.default_rng(random.getrandbits(64))
        im = label["img"]
        if (
            library is not None
            and im.ndim == 3
            and im.shape[2] == 3
            and label["instances"].keypoints is None
            and rng.random() < aug.distractors
        ):
            distractor = library.pick(rng)
            if distractor is not None:
                pasted(label, distractor, aug, rng)
        label["img"] = vary(label["img"], aug, rng)
        h, w = label["resized_shape"] = label["img"].shape[:2]
        h0, w0 = label["ori_shape"]
        label["ratio_pad"] = (h / h0, w / w0)
        return label

    return get_image_and_label
