"""Krill's own training augmentations, applied to each image as it is loaded,
before Ultralytics' mosaic and affine steps.

Every step returns a new array: Ultralytics hands back the same cached image
each time an index is loaded again.
"""

import random
from collections.abc import Callable
from typing import Any

import cv2
import numpy as np
import numpy.typing as npt
from krill.v1 import export_pb2

Image = npt.NDArray[np.uint8]

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


def trainer(base: type[Any], aug: export_pb2.Augmentation) -> type[Any]:
    """Subclass an Ultralytics trainer so its training images, and never its
    val images, go through vary."""

    class Trainer(base):  # type: ignore[misc]
        def build_dataset(self, img_path: str, mode: str = "train", batch: Any = None) -> Any:
            dataset = super().build_dataset(img_path, mode, batch)
            if mode == "train":
                dataset.load_image = varied(dataset.load_image, aug)
            return dataset

    return Trainer


def varied(
    load: Callable[..., tuple[Image, Any, Any]], aug: export_pb2.Augmentation
) -> Callable[..., tuple[Image, Any, Any]]:
    def load_image(*args: Any, **kwargs: Any) -> tuple[Image, Any, Any]:
        im, original, _ = load(*args, **kwargs)
        # Ultralytics seeds the stdlib RNG in each dataloader worker, so this
        # differs between workers where a Generator made here would not.
        rng = np.random.default_rng(random.getrandbits(64))
        im = vary(im, aug, rng)
        return im, original, im.shape[:2]

    return load_image
