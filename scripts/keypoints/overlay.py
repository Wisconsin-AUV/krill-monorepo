"""Draw what SAM's mask and the keypoint geometry make of chosen frames.

For each box of the label type on a frame, SAM segments the object from the
box, as fitting keypoints does, and three panels show what comes of it:

- mask: SAM's mask, its convex hull, and the hull's four corners, which are
  where tracking snaps a quad's corners when the quad has no reference
  design.
- paper: the bright paper in yellow, outlined by its hull, which is where
  the design is scored, the whole sheet SAM segments from a box around it in
  cyan, and the print found from the sheet: unprompted in red,
  as fitting does, and forced in orange, as a click to fill the quad does.
  --bright-only leaves SAM out and finds the print from the bright paper.
- keypoints: the stored keypoints (white for a labeler's, cyan for SAM's),
  the print found, and where snapping moves stored corners (magenta arrows).
  With --track, keypoints tracked from each range's first frame are added in
  blue.

Each model gets a directory of images and a metrics.csv with, for every
box, how the mask matches the box and how many pixels (at the 1024-pixel
working size) each derived keypoint lands from a labeler's.

Run through overlay.sh, which exports the labels and starts the worker image.
Frames are numbered as the labeler shows them, from 1:

    VIDEO               every frame of the video with stored keypoints
    VIDEO:CLIP          every such frame in the clip
    VIDEO:CLIP:FRAME    one frame
    VIDEO:CLIP:A-B      frames A to B of the clip, tracked with --track

VIDEO is the video's name or id. The labeler's address of a frame works too,
as clip=ID frame=INDEX or its URL (/clips/ID?frame=INDEX), where INDEX may
also be a range A-B of the video's frame indexes.
"""

import argparse
import csv
import json
import os
import re
import statistics
import sys
from collections import defaultdict
from collections.abc import Iterator
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import boto3
import cv2
import numpy as np
import torch
from botocore.config import Config

from krill_worker import keypoints as kp
from krill_worker.frames import decode_frame
from krill_worker.orient import largest, quad
from krill_worker.sam import SamTracker
from krill_worker.track import BoxPrompt, Mask

PANEL_WIDTH = 640
# Context kept around the box, as a share of its larger side.
MARGIN = 0.25
# Mask pieces smaller than this share of the mask aren't counted as parts.
MIN_PART = 0.01

# BGR.
WHITE = (255, 255, 255)
GRAY = (150, 150, 150)
CYAN = (255, 255, 0)
RED = (40, 40, 255)
ORANGE = (0, 150, 255)
MAGENTA = (255, 0, 255)
BLUE = (255, 120, 0)
GREEN = (60, 200, 60)
DARK_GREEN = (30, 110, 30)
YELLOW = (0, 230, 255)


@dataclass(frozen=True)
class Stored:
    id: int
    track_id: int
    video_id: int
    idx: int
    box: tuple[float, float, float, float]
    keypoints: list[float] | None
    source: str
    status: str


@dataclass
class Labels:
    type_name: str
    names: list[str]
    schema: kp.Schema
    reference_key: str | None
    videos: dict[int, str]
    clips: dict[int, list[tuple[int, int, int]]]
    clip_videos: dict[int, int]
    boxes: dict[tuple[int, int], list[Stored]] = field(default_factory=dict)

    @staticmethod
    def load(path: Path) -> "Labels":
        data = json.loads(path.read_text())
        lt = data["label_type"]
        if lt is None:
            sys.exit("no such label type")
        quads = tuple(
            kp.Quad(tuple(q["corners"]), bool(q.get("reference")), q.get("center"))
            for q in lt["quads"]
            if len(q["corners"]) == 4
        )
        clips: dict[int, list[tuple[int, int, int]]] = defaultdict(list)
        for c in data["clips"]:
            clips[c["video_id"]].append((c["idx"], c["start_frame"], c["frame_count"]))
        boxes: dict[tuple[int, int], list[Stored]] = defaultdict(list)
        for b in data["boxes"]:
            s = Stored(**{**b, "box": tuple(b["box"])})
            boxes[(s.video_id, s.idx)].append(s)
        return Labels(
            type_name=lt["name"],
            names=[k["name"] for k in lt["keypoints"]],
            schema=kp.Schema(len(lt["keypoints"]), quads),
            reference_key=lt["reference_key"],
            videos={v["id"]: v["name"] for v in data["videos"]},
            clips=clips,
            clip_videos={c["id"]: c["video_id"] for c in data["clips"]},
            boxes=boxes,
        )

    def video(self, name: str) -> int:
        for vid, n in self.videos.items():
            if n == name or str(vid) == name:
                return vid
        sys.exit(f"no video {name!r}")

    def clip_range(self, vid: int, clip: int) -> range:
        """Frame indexes of clip, numbered from 1 as the labeler shows them."""
        for idx, start, count in self.clips[vid]:
            if idx == clip - 1:
                return range(start, start + count)
        sys.exit(f"{self.videos[vid]} has no clip {clip}")

    def position(self, vid: int, idx: int) -> tuple[int, int]:
        """The clip and frame the labeler shows for a frame index."""
        for c, start, count in self.clips[vid]:
            if start <= idx < start + count:
                return c + 1, idx - start + 1
        return 0, idx + 1

    def labeled(self, vid: int, frames: range | None = None) -> list[int]:
        return sorted(
            idx
            for (v, idx), boxes in self.boxes.items()
            if v == vid
            and (frames is None or idx in frames)
            and any(b.keypoints for b in boxes)
        )


def runs(idxs: list[int]) -> Iterator[list[int]]:
    """Consecutive frames, grouped."""
    run: list[int] = []
    for i in idxs:
        if run and i != run[-1] + 1:
            yield run
            run = []
        run.append(i)
    if run:
        yield run


# A frame's address in the labeler: clip=276 frame=338, or the URL
# /clips/276?frame=338, with the clip's id and the video's frame index.
ADDRESS = re.compile(r"clips?[/=](\d+)\D*?frame=(\d+)(?:-(\d+))?")


def parse(labels: Labels, specs: list[str]) -> list[tuple[int, list[int]]]:
    out: list[tuple[int, list[int]]] = []
    joined = " ".join(specs)
    for m in ADDRESS.finditer(joined):
        clip, first = int(m[1]), int(m[2])
        if clip not in labels.clip_videos:
            sys.exit(f"no clip {clip}")
        out.append((labels.clip_videos[clip], list(range(first, int(m[3] or first) + 1))))
    for spec in ADDRESS.sub(" ", joined).split():
        parts = spec.split(":")
        vid = labels.video(parts[0])
        if len(parts) == 1:
            out += [(vid, r) for r in runs(labels.labeled(vid))]
            continue
        frames = labels.clip_range(vid, int(parts[1]))
        if len(parts) == 2:
            out += [(vid, r) for r in runs(labels.labeled(vid, frames))]
            continue
        a, _, b = parts[2].partition("-")
        first, last = int(a), int(b or a)
        if not 1 <= first <= last <= len(frames):
            sys.exit(f"{spec}: clip has frames 1 to {len(frames)}")
        out.append((vid, list(frames[first - 1 : last])))
    if not out:
        sys.exit("no frames match")
    return out


class Store:
    def __init__(self) -> None:
        self.s3 = boto3.client(
            "s3",
            endpoint_url=os.environ.get("KRILL_S3_ENDPOINT", "http://minio:9000"),
            aws_access_key_id=os.environ["KRILL_S3_ACCESS_KEY"],
            aws_secret_access_key=os.environ["KRILL_S3_SECRET_KEY"],
            region_name="us-east-1",
            config=Config(signature_version="s3v4"),
        )
        self.bucket = os.environ.get("KRILL_S3_BUCKET") or "krill"
        self.frames: dict[tuple[int, int], bytes] = {}

    def get(self, key: str) -> bytes:
        data: bytes = self.s3.get_object(Bucket=self.bucket, Key=key)["Body"].read()
        return data

    def frame(self, vid: int, idx: int) -> bytes:
        if (vid, idx) not in self.frames:
            self.frames[(vid, idx)] = self.get(f"videos/{vid}/frames/{idx:06d}.jpg")
        return self.frames[(vid, idx)]


def points(stored: list[float] | None, count: int, shape: tuple[int, ...]) -> kp.Keypoints:
    h, w = shape[:2]
    out = np.zeros((count, 3))
    if stored:
        for i in range(min(count, len(stored) // 3)):
            x, y, v = stored[3 * i : 3 * i + 3]
            if v > kp.UNLABELED:
                out[i] = (x * w, y * h, v)
    return out


def box_px(box: tuple[float, float, float, float], shape: tuple[int, ...]) -> np.ndarray:
    h, w = shape[:2]
    return np.array([box[0] * w, box[1] * h, box[2] * w, box[3] * h])


@dataclass
class Analysis:
    mask: Mask
    outline: np.ndarray | None = None
    hull: np.ndarray | None = None
    corners: np.ndarray | None = None
    parts: int = 0
    box_iou: float = 0.0
    outside: float = 0.0
    paper: np.ndarray | None = None
    sheet: np.ndarray | None = None
    paper_corners: np.ndarray | None = None
    found: np.ndarray | None = None
    found_score: float | None = None
    forced: np.ndarray | None = None
    forced_score: float | None = None
    forced_margin: float | None = None
    snapped: dict[int, np.ndarray] = field(default_factory=dict)


def analyze(
    rgb: np.ndarray,
    gray: np.ndarray,
    mask: Mask,
    box: np.ndarray,
    stored: kp.Keypoints,
    schema: kp.Schema,
    design: kp.Design | None,
    segment: kp.Segment | None,
) -> Analysis:
    a = Analysis(mask=mask)
    m = mask.astype(np.uint8)
    contours, _ = cv2.findContours(m, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_NONE)
    total = float(m.sum())
    a.parts = sum(1 for c in contours if cv2.contourArea(c) >= MIN_PART * total)
    a.outline = largest(contours)
    if a.outline is not None:
        a.hull = cv2.convexHull(a.outline).reshape(-1, 2)
        a.corners = quad(a.outline)
    x, y, w, h = box
    inside = np.zeros_like(m)
    inside[int(y) : int(np.ceil(y + h)), int(x) : int(np.ceil(x + w))] = 1
    if total:
        a.outside = float((m & (1 - inside)).sum() / total)
    if (b := kp.bounds(mask)) is not None:
        a.box_iou = iou(np.array(b), box)
    if (papers := kp.paper_of(gray, mask, segment)) is not None:
        a.paper, a.sheet = papers
        pc, _ = cv2.findContours(a.sheet, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_NONE)
        if (outline := largest(pc)) is not None:
            a.paper_corners = quad(outline)
    if design is not None and a.paper is not None:
        area = kp.print_area(a.paper)
        img = kp.ink_on(rgb, a.paper, area)
        if (found := kp.find_print(rgb, gray, mask, design, None, segment)) is not None:
            a.found = found[0]
            a.found_score = kp.score(img, a.found, design, area)
        if (forced := kp.fill_quad(rgb, mask, design, segment)) is not None:
            a.forced = forced[0]
            _, a.forced_score, a.forced_margin = kp.turned(img, a.forced, design, area)
    for q in schema.quads:
        idx = list(q.corners)
        if q.reference or not (stored[idx, 2] > kp.UNLABELED).all():
            continue
        moved = kp.snap(stored[idx, :2].astype(np.float32), mask)
        for i, p in zip(idx, moved, strict=True):
            a.snapped[i] = p
    return a


def iou(a: np.ndarray, b: np.ndarray) -> float:
    x0, y0 = max(a[0], b[0]), max(a[1], b[1])
    x1, y1 = min(a[0] + a[2], b[0] + b[2]), min(a[1] + a[3], b[1] + b[3])
    inter = max(0.0, x1 - x0) * max(0.0, y1 - y0)
    union = a[2] * a[3] + b[2] * b[3] - inter
    return float(inter / union) if union > 0 else 0.0


class View:
    """A crop of the full-resolution frame around a box, with coordinates in
    working pixels mapped onto it."""

    def __init__(self, full: np.ndarray, work_shape: tuple[int, ...], box: np.ndarray) -> None:
        H, W = work_shape[:2]
        scale = full.shape[1] / W
        x, y, w, h = box
        m = MARGIN * max(w, h)
        self.x0, self.y0 = max(0, int(x - m)), max(0, int(y - m))
        self.x1, self.y1 = min(W, int(np.ceil(x + w + m))), min(H, int(np.ceil(y + h + m)))
        self.zoom = PANEL_WIDTH / max(1, self.x1 - self.x0)
        self.size = (PANEL_WIDTH, max(1, round((self.y1 - self.y0) * self.zoom)))
        crop = full[
            round(self.y0 * scale) : round(self.y1 * scale),
            round(self.x0 * scale) : round(self.x1 * scale),
        ]
        self.base = cv2.resize(
            cv2.cvtColor(crop, cv2.COLOR_RGB2BGR), self.size, interpolation=cv2.INTER_AREA
        )

    def panel(self) -> np.ndarray:
        return self.base.copy()

    def pt(self, p: Any) -> tuple[int, int]:
        q = (np.asarray(p, np.float64)[:2] - (self.x0, self.y0)) * self.zoom
        return int(round(q[0])), int(round(q[1]))

    def tint(self, img: np.ndarray, mask: np.ndarray, color: tuple[int, int, int]) -> None:
        crop = mask[self.y0 : self.y1, self.x0 : self.x1].astype(np.uint8)
        m = cv2.resize(crop, self.size, interpolation=cv2.INTER_NEAREST) > 0
        img[m] = (img[m] * 0.6 + np.array(color) * 0.4).astype(np.uint8)

    def poly(
        self, img: np.ndarray, pts: np.ndarray, color: tuple[int, int, int], thick: int = 2
    ) -> None:
        p = np.array([self.pt(q) for q in pts], np.int32).reshape(-1, 1, 2)
        cv2.polylines(img, [p], True, color, thick, cv2.LINE_AA)


def text(img: np.ndarray, s: str, at: tuple[int, int], color: Any = WHITE, size: float = 0.5) -> None:
    cv2.putText(img, s, at, cv2.FONT_HERSHEY_SIMPLEX, size, (0, 0, 0), 3, cv2.LINE_AA)
    cv2.putText(img, s, at, cv2.FONT_HERSHEY_SIMPLEX, size, color, 1, cv2.LINE_AA)


def mark(
    img: np.ndarray,
    at: tuple[int, int],
    color: tuple[int, int, int],
    style: str,
    label: str = "",
) -> None:
    if style == "ring":
        cv2.circle(img, at, 7, (0, 0, 0), 3, cv2.LINE_AA)
        cv2.circle(img, at, 7, color, 2, cv2.LINE_AA)
    elif style == "cross":
        for d in ((-6, -6, 6, 6), (-6, 6, 6, -6)):
            p, q = (at[0] + d[0], at[1] + d[1]), (at[0] + d[2], at[1] + d[3])
            cv2.line(img, p, q, (0, 0, 0), 4, cv2.LINE_AA)
            cv2.line(img, p, q, color, 2, cv2.LINE_AA)
    else:
        cv2.circle(img, at, 4, (0, 0, 0), -1, cv2.LINE_AA)
        cv2.circle(img, at, 3, color, -1, cv2.LINE_AA)
    if label:
        text(img, label, (at[0] + 8, at[1] - 6), color, 0.4)


def draw(
    view: View,
    a: Analysis,
    box: np.ndarray,
    stored: kp.Keypoints,
    human: bool,
    tracked: tuple[Mask | None, kp.Keypoints] | None,
    names: list[str],
) -> np.ndarray:
    x, y, w, h = box
    corners = np.array([[x, y], [x + w, y], [x + w, y + h], [x, y + h]])

    mask_panel = view.panel()
    view.tint(mask_panel, a.mask, GREEN)
    view.poly(mask_panel, corners, WHITE, 1)
    if a.outline is not None:
        view.poly(mask_panel, a.outline, GREEN, 1)
    if a.hull is not None:
        view.poly(mask_panel, a.hull, DARK_GREEN, 1)
    if a.corners is not None:
        for i, c in enumerate(a.corners):
            mark(mask_panel, view.pt(c), MAGENTA, "dot", f"s{i}")
    text(mask_panel, f"SAM mask  iou {a.box_iou:.2f}  out {a.outside:.0%}  parts {a.parts}", (8, 20))

    paper_panel = view.panel()
    if a.paper is not None:
        view.tint(paper_panel, a.paper > 0, YELLOW)
        contours, _ = cv2.findContours(kp.print_area(a.paper), cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_NONE)
        if (outline := largest(contours)) is not None:
            view.poly(paper_panel, outline, YELLOW, 1)
    if a.sheet is not None and a.sheet is not a.paper:
        contours, _ = cv2.findContours(a.sheet, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_NONE)
        if (outline := largest(contours)) is not None:
            view.poly(paper_panel, outline, CYAN, 2)
    if a.forced is not None:
        view.poly(paper_panel, a.forced, ORANGE, 1)
    if a.found is not None:
        view.poly(paper_panel, a.found, RED, 2)
        for i, c in enumerate(a.found):
            mark(paper_panel, view.pt(c), RED, "dot", str(i))
    found = "none" if a.found_score is None else f"{a.found_score:.2f}"
    forced = "none" if a.forced_score is None else f"{a.forced_score:.2f} (turn +{a.forced_margin:.2f})"
    text(paper_panel, f"paper  found {found}  forced {forced}", (8, 20))

    kp_panel = view.panel()
    if tracked is not None and tracked[0] is not None:
        view.tint(kp_panel, tracked[0], BLUE)
    for i, (px, py, v) in enumerate(stored):
        if v <= kp.UNLABELED:
            continue
        color = (WHITE if v == kp.VISIBLE else GRAY) if human else CYAN
        mark(kp_panel, view.pt((px, py)), color, "ring", names[i])
    for i, p in a.snapped.items():
        if np.linalg.norm(p - stored[i, :2]) >= 1:
            cv2.arrowedLine(
                kp_panel, view.pt(stored[i]), view.pt(p), MAGENTA, 2, cv2.LINE_AA, tipLength=0.2
            )
    if a.found is not None:
        for c in a.found:
            mark(kp_panel, view.pt(c), RED, "cross")
    if tracked is not None:
        for px, py, v in tracked[1]:
            if v > kp.UNLABELED:
                mark(kp_panel, view.pt((px, py)), BLUE, "dot")
    who = "labeler" if human else "stored SAM"
    text(kp_panel, f"{who}  print found  snap  tracked", (8, 20))

    return np.hstack([mask_panel, paper_panel, kp_panel])


def header(img: np.ndarray, lines: list[str]) -> np.ndarray:
    strip = np.zeros((14 + 20 * len(lines), img.shape[1], 3), np.uint8)
    for i, s in enumerate(lines):
        text(strip, s, (8, 22 + 20 * i))
    return np.vstack([strip, img])


def distance(a: np.ndarray, b: np.ndarray) -> float:
    return float(np.linalg.norm(np.asarray(a)[:2] - np.asarray(b)[:2]))


def metrics(
    a: Analysis,
    stored: kp.Keypoints,
    human: bool,
    tracked: tuple[Mask | None, kp.Keypoints] | None,
    schema: kp.Schema,
    names: list[str],
) -> dict[str, Any]:
    row: dict[str, Any] = {
        "mask_box_iou": round(a.box_iou, 3),
        "mask_outside_box": round(a.outside, 3),
        "mask_parts": a.parts,
        "found_score": None if a.found_score is None else round(a.found_score, 3),
        "forced_score": None if a.forced_score is None else round(a.forced_score, 3),
        "forced_margin": None if a.forced_margin is None else round(a.forced_margin, 3),
    }
    if not human:
        return row
    labeled = stored[:, 2] > kp.UNLABELED
    ref = schema.reference
    found: list[float] = []
    if ref is not None and a.found is not None:
        idx = list(ref.corners)
        for i, c in zip(idx, a.found, strict=True):
            if labeled[i]:
                found.append(distance(c, stored[i]))
                row[f"found_{names[i]}"] = round(found[-1], 1)
        # The same, at the best quarter turn, to tell a turned print from a
        # misplaced edge.
        turns = [
            [distance(c, stored[i]) for i, c in zip(idx, np.roll(a.found, -k, axis=0)) if labeled[i]]
            for k in range(4)
        ]
        row["found_max_any_turn"] = round(min(max(t, default=0) for t in turns), 1)
    row["found_max"] = round(max(found), 1) if found else None
    if ref is not None and a.forced is not None:
        forced = [distance(c, stored[i]) for i, c in zip(ref.corners, a.forced) if labeled[i]]
        row["forced_max"] = round(max(forced), 1) if forced else None
    snapped = [distance(p, stored[i]) for i, p in a.snapped.items()]
    for i, p in a.snapped.items():
        row[f"snap_{names[i]}"] = round(distance(p, stored[i]), 1)
    row["snap_max"] = round(max(snapped), 1) if snapped else None
    if tracked is not None:
        errs = []
        for i, (px, py, v) in enumerate(tracked[1]):
            if labeled[i] and v > kp.UNLABELED:
                errs.append(distance((px, py), stored[i]))
                row[f"tracked_{names[i]}"] = round(errs[-1], 1)
        row["tracked_max"] = round(max(errs), 1) if errs else None
    return row


def segmenter(sam: SamTracker | None, frame: np.ndarray) -> kp.Segment | None:
    """SAM's mask of a box on frame, encoding the frame once on first use, as
    the worker does."""
    if sam is None:
        return None
    features = None

    def segment(box: tuple[float, float, float, float]) -> Mask | None:
        nonlocal features
        if features is None:
            features = sam.features(frame)
        return sam.segment(frame, [BoxPrompt(box)], features)[0]

    return segment


def track(
    sam: SamTracker,
    store: Store,
    labels: Labels,
    design: kp.Design | None,
    vid: int,
    frames: list[int],
    anchor: bool,
    sheets: bool,
) -> dict[tuple[int, int, int], tuple[Mask | None, kp.Keypoints]]:
    """Each track with keypoints on the run's first frame, followed through
    the run as a track task would, from its stored box and keypoints."""
    out: dict[tuple[int, int, int], tuple[Mask | None, kp.Keypoints]] = {}
    images = [decode_frame(store.frame(vid, i)) for i in frames]
    for start in labels.boxes.get((vid, frames[0]), []):
        if not start.keypoints:
            continue
        shape = images[0].shape
        tracker = kp.KeypointTracker(
            labels.schema, design, points(start.keypoints, labels.schema.count, shape)
        )
        masks = sam.track(((im, None) for im in images), BoxPrompt(start.box))
        for idx, image, mask in zip(frames, images, masks, strict=True):
            pts = tracker.step(image, mask, segmenter(sam if sheets else None, image))
            if anchor:
                for b in labels.boxes.get((vid, idx), []):
                    if b.track_id == start.track_id and b.source == "human" and b.keypoints:
                        tracker.anchor(points(b.keypoints, labels.schema.count, image.shape))
                        pts = tracker.shown()
            out[(start.track_id, vid, idx)] = (mask, pts)
    return out


def summarize(rows: list[dict[str, Any]]) -> str:
    human = [r for r in rows if r["source"] == "human"]
    out = [f"{len(rows)} boxes, {len(human)} labeled by hand"]
    for key in ("mask_box_iou", "found_max", "found_max_any_turn", "forced_max", "snap_max", "tracked_max"):
        vals = [r[key] for r in (rows if key == "mask_box_iou" else human) if r.get(key) is not None]
        if vals:
            out.append(
                f"  {key:20} median {statistics.median(vals):7.2f}  worst {max(vals) if key != 'mask_box_iou' else min(vals):7.2f}  (n={len(vals)})"
            )
    missed = sum(1 for r in human if r.get("found_score") is None)
    if human:
        out.append(f"  print not found on {missed} of {len(human)} labeled boxes")
    return "\n".join(out)


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("frames", nargs="+", help="VIDEO[:CLIP[:FRAME[-FRAME]]] or clip=ID frame=INDEX")
    p.add_argument("--labels", type=Path, required=True, help="export.sql's output")
    p.add_argument("--out", type=Path, required=True)
    p.add_argument("--model", action="append", help="SAM model, repeatable")
    p.add_argument("--track", action="store_true", help="track each range from its first frame")
    p.add_argument("--anchor", action="store_true", help="re-anchor tracking on labeled frames")
    p.add_argument("--bright-only", action="store_true", help="find the paper by brightness alone")
    args = p.parse_args()

    labels = Labels.load(args.labels)
    selected = parse(labels, args.frames)
    store = Store()
    design = None
    if labels.reference_key and labels.schema.reference is not None:
        design = kp.load_design(store.get(labels.reference_key))
    models = args.model or [os.environ.get("KRILL_SAM_MODEL", "facebook/sam2.1-hiera-small")]
    frames = sorted({(vid, i) for vid, run in selected for i in run})
    print(f"{labels.type_name}: {len(frames)} frames, models {', '.join(models)}", flush=True)

    for model in models:
        out = args.out / model.replace("/", "--")
        out.mkdir(parents=True, exist_ok=True)
        sam = SamTracker(model)
        tracked: dict[tuple[int, int, int], tuple[Mask | None, kp.Keypoints]] = {}
        if args.track:
            for vid, run in selected:
                if len(run) > 1:
                    tracked.update(
                        track(sam, store, labels, design, vid, run, args.anchor, not args.bright_only)
                    )
        rows: list[dict[str, Any]] = []
        for vid, idx in frames:
            data = store.frame(vid, idx)
            work = decode_frame(data)
            full = cv2.cvtColor(cv2.imdecode(np.frombuffer(data, np.uint8), cv2.IMREAD_COLOR), cv2.COLOR_BGR2RGB)
            gray = np.asarray(cv2.cvtColor(work, cv2.COLOR_RGB2GRAY), np.uint8)
            boxes = labels.boxes.get((vid, idx), [])
            masks = sam.segment(work, [BoxPrompt(b.box) for b in boxes]) if boxes else []
            clip, frame = labels.position(vid, idx)
            name = labels.videos[vid]
            for b, mask in zip(boxes, masks, strict=True):
                if mask is None or not mask.any():
                    print(f"{name} clip {clip} frame {frame}: SAM found nothing in box {b.id}")
                    continue
                box = box_px(b.box, work.shape)
                stored = points(b.keypoints, labels.schema.count, work.shape)
                human = b.source == "human"
                t = tracked.get((b.track_id, vid, idx))
                segment = segmenter(None if args.bright_only else sam, work)
                a = analyze(work, gray, mask, box, stored, labels.schema, design, segment)
                img = draw(View(full, work.shape, box), a, box, stored, human, t, labels.names)
                row = {
                    "model": model,
                    "video": name,
                    "clip": clip,
                    "frame": frame,
                    "annotation": b.id,
                    "track": b.track_id,
                    "source": b.source,
                    "box_px": round(max(box[2], box[3]), 1),
                    **metrics(a, stored, human, t, labels.schema, labels.names),
                }
                rows.append(row)
                img = header(
                    img,
                    [
                        f"{name}  clip {clip}  frame {frame}  box {b.id}  track {b.track_id}  {b.source}  {model}",
                        "  ".join(
                            f"{k} {row[k]}"
                            for k in ("found_max", "found_max_any_turn", "forced_max", "snap_max", "tracked_max")
                            if row.get(k) is not None
                        )
                        + "  (px at 1024 wide)",
                        "mask: white box, magenta snap targets | paper: yellow bright, cyan SAM sheet, orange forced, "
                        "red found | keypoints: white labeler, cyan SAM, red found, blue tracked",
                    ],
                )
                cv2.imwrite(str(out / f"{name}_c{clip}_f{frame:03d}_b{b.id}.jpg"), img, [cv2.IMWRITE_JPEG_QUALITY, 90])
        keys: list[str] = []
        for r in rows:
            keys += [k for k in r if k not in keys]
        with open(out / "metrics.csv", "w", newline="") as f:
            w = csv.DictWriter(f, keys)
            w.writeheader()
            w.writerows(rows)
        print(f"\n{model} -> {out}\n{summarize(rows)}", flush=True)
        del sam
        torch.cuda.empty_cache()


if __name__ == "__main__":
    main()
