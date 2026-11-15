import numpy as np
from krill.v1 import pipeline_pb2

from krill_worker import augment

EVERYTHING = pipeline_pb2.Augmentation(aspect=1.5, underwater=1, shadows=1, noise=1)


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


def test_underwater_takes_red_away_not_blue() -> None:
    gray = np.full((8, 8, 3), 128, np.uint8)
    for seed in range(20):
        b, _, r = augment.underwater(gray, np.random.default_rng(seed)).mean(axis=(0, 1))
        assert b > r
