"""Score how keypoints treat a sheet's corners where a crate's wall hides them.

walls.json holds tracks of crates seen at an angle, and on some of their
frames where each corner of the print shows, labeled by hand:

    [x, y]   the corner shows there, as a fraction of the frame
    null     the wall hides it, so it should be left unlabeled
    "?"      it sits at the wall's edge and can't be told apart; not scored

Each track's crate is followed with SAM from its box, once, and every way
keypoints get placed is replayed on the same masks, since SAM isn't
deterministic run to run on the GPU:

    track-fill      Shift+click on the print, then tracked from it
    track-labeled   tracked from the corners that show on the first frame
    track-none      tracked from the box alone
    fill            Shift+click on each labeled frame by itself
    fit             Fit keypoints on each labeled frame

Run through walls.sh, which exports the label type and starts the worker
image.
"""

import argparse
import json
import statistics
from collections.abc import Callable
from pathlib import Path
from typing import Any

import cv2
import numpy as np
from overlay import Labels, Store, segmenter

from krill_worker import keypoints as kp
from krill_worker.frames import decode_frame
from krill_worker.sam import SamTracker
from krill_worker.track import BoxPrompt, Mask

# Pixels, at the 1024-pixel working size, a placed corner may be from the
# labeled one and still count as found.
NEAR = 15.0
CORNERS = ("tl", "tr", "br", "bl")


class Track:
    def __init__(self, name: str, spec: dict[str, Any], labels: Labels, store: Store) -> None:
        self.name = name
        vid = labels.video(spec["video"])
        first, last = spec["frames"]
        self.idxs = list(range(first, last + 1))
        self.images = [decode_frame(store.frame(vid, i)) for i in self.idxs]
        self.box = tuple(spec["box"])
        h, w = self.images[0].shape[:2]
        self.corners: dict[int, list[Any]] = {
            int(i): [c if c is None or c == "?" else (c[0] * w, c[1] * h) for c in cs]
            for i, cs in spec["corners"].items()
        }
        self.masks: list[Mask | None] = []
        self.segments: list[kp.Segment] = []

    def follow(self, sam: SamTracker) -> None:
        self.masks = list(sam.track(((im, None) for im in self.images), BoxPrompt(self.box)))
        self.segments = [remembered(segmenter(sam, im)) for im in self.images]


def remembered(segment: kp.Segment | None) -> kp.Segment:
    """segment, answering the same box the same way every time it's asked."""
    assert segment is not None
    seen: dict[tuple[float, ...], Mask | None] = {}

    def again(box: tuple[float, float, float, float]) -> Mask | None:
        key = tuple(round(v, 4) for v in box)
        if key not in seen:
            seen[key] = segment(box)
        return seen[key]

    return again


def track(t: Track, schema: kp.Schema, design: kp.Design, start: str) -> dict[int, kp.Keypoints]:
    points = np.zeros((schema.count, 3))
    ref = schema.reference
    assert ref is not None
    if start == "labeled":
        for i, c in zip(ref.corners, t.corners.get(t.idxs[0], []), strict=False):
            if c is not None and c != "?":
                points[i] = (*c, kp.VISIBLE)
    tracker = kp.KeypointTracker(schema, design, points)
    out = {}
    for n, (idx, image, mask, segment) in enumerate(
        zip(t.idxs, t.images, t.masks, t.segments, strict=True)
    ):
        if n == 0 and start == "fill" and mask is not None:
            tracker.fill(image, schema.quads.index(ref), mask, segment)
        out[idx] = tracker.step(image, mask, segment)
    return out


def each_frame(
    t: Track, place: Callable[[np.ndarray, Mask, kp.Segment], kp.Keypoints | None]
) -> dict[int, kp.Keypoints]:
    out = {}
    for idx in t.corners:
        n = t.idxs.index(idx)
        mask = t.masks[n]
        placed = None if mask is None else place(t.images[n], mask, t.segments[n])
        if placed is not None:
            out[idx] = placed
    return out


def fill(schema: kp.Schema, design: kp.Design) -> Callable[..., kp.Keypoints]:
    def place(image: np.ndarray, mask: Mask, segment: kp.Segment) -> kp.Keypoints:
        tracker = kp.KeypointTracker(schema, design, np.zeros((schema.count, 3)))
        assert schema.reference is not None
        tracker.fill(image, schema.quads.index(schema.reference), mask, segment)
        return tracker.shown()

    return place


def fit(schema: kp.Schema, design: kp.Design) -> Callable[..., kp.Keypoints | None]:
    def place(image: np.ndarray, mask: Mask, segment: kp.Segment) -> kp.Keypoints | None:
        gray = np.asarray(cv2.cvtColor(image, cv2.COLOR_RGB2GRAY), np.uint8)
        found = kp.find_print(image, gray, mask, design, None, segment)
        if found is None:
            return None
        assert schema.reference is not None
        points = np.zeros((schema.count, 3))
        for i, c, v in zip(schema.reference.corners, *found, strict=True):
            points[i] = (c[0], c[1], v)
        return kp.shown(points, schema)

    return place


class Score:
    def __init__(self) -> None:
        self.hidden_placed = 0
        self.hidden = 0
        self.found = 0
        self.off = 0
        self.missed = 0
        self.unsure_placed = 0
        self.unsure = 0
        self.errors: list[float] = []

    def add(self, labeled: list[Any], placed: kp.Keypoints | None, ref: kp.Quad) -> list[str]:
        wrong = []
        for name, i, c in zip(CORNERS, ref.corners, labeled, strict=True):
            on = placed is not None and placed[i, 2] > kp.UNLABELED
            if c == "?":
                self.unsure += 1
                self.unsure_placed += on
            elif c is None:
                self.hidden += 1
                self.hidden_placed += on
                if on:
                    wrong.append(f"{name} placed behind the wall")
            elif not on:
                self.missed += 1
                wrong.append(f"{name} missed")
            else:
                assert placed is not None
                d = float(np.hypot(*(placed[i, :2] - c)))
                self.errors.append(d)
                if d <= NEAR:
                    self.found += 1
                else:
                    self.off += 1
                    wrong.append(f"{name} {d:.0f} px off")
        return wrong

    def merge(self, other: "Score") -> None:
        for k, v in vars(other).items():
            setattr(self, k, getattr(self, k) + v)

    def line(self) -> str:
        e = sorted(self.errors)
        err = f"median {statistics.median(e):4.1f} px" if e else "median    -   "
        return (
            f"behind the wall placed {self.hidden_placed:2d}/{self.hidden:<2d}  "
            f"showing found {self.found:2d} off {self.off:2d} missed {self.missed:2d} ({err})  "
            f"at the wall placed {self.unsure_placed:2d}/{self.unsure}"
        )


def main() -> None:
    p = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    p.add_argument("--labels", type=Path, required=True, help="export.sql's output")
    p.add_argument("--tracks", type=Path, required=True, help="walls.json")
    p.add_argument("--model", default="facebook/sam2.1-hiera-small")
    p.add_argument("--verbose", action="store_true", help="list each corner placed wrong")
    args = p.parse_args()

    labels = Labels.load(args.labels)
    spec = json.loads(args.tracks.read_text())
    if labels.type_name != spec["label_type"] or labels.reference_key is None:
        raise SystemExit(
            f"walls.json is for {spec['label_type']}, which needs its reference design"
        )
    store = Store()
    design = kp.load_design(store.get(labels.reference_key))
    schema = labels.schema
    ref = schema.reference
    assert ref is not None
    sam = SamTracker(args.model)
    totals: dict[str, Score] = {}
    for name, s in spec["tracks"].items():
        t = Track(name, s, labels, store)
        t.follow(sam)
        modes = {
            "track-fill": track(t, schema, design, "fill"),
            "track-labeled": track(t, schema, design, "labeled"),
            "track-none": track(t, schema, design, "none"),
            "fill": each_frame(t, fill(schema, design)),
            "fit": each_frame(t, fit(schema, design)),
        }
        for mode, placed in modes.items():
            score = Score()
            for idx, labeled in sorted(t.corners.items()):
                wrong = score.add(labeled, placed.get(idx), ref)
                if wrong and args.verbose:
                    print(f"    {name} {mode} frame {idx}: {', '.join(wrong)}")
            print(f"{name:8} {mode:13} {score.line()}")
            totals.setdefault(mode, Score()).merge(score)
    for mode, score in totals.items():
        print(f"{'all':8} {mode:13} {score.line()}")


if __name__ == "__main__":
    main()
