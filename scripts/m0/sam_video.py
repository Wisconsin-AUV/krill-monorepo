"""M0: measure SAM click latency, tracking speed, and VRAM on the local GPU.

Runs the worker's tracker on a public test video looped to clip length, with
and without precomputed embeddings, and breaks each tracked frame down by
stage. Run inside the worker image:

    docker run --rm --device /dev/kfd --device /dev/dri -e HSA_OVERRIDE_GFX_VERSION=10.3.0 \
        -v krill-hf:/root/.cache/huggingface -v ./scripts/m0:/m0 krill-worker \
        python /m0/sam_video.py --model facebook/sam2.1-hiera-small

Stage times are measured with a GPU sync after each stage, so the per-stage
sum is a little above the free-running fps the summary table reports.
"""

import argparse
import time
import urllib.request
from collections import defaultdict
from collections.abc import Callable, Iterator
from contextlib import contextmanager
from pathlib import Path
from typing import Any

import cv2
import numpy as np
import torch

import krill_worker.sam as sam_module
from krill_worker.sam import SamTracker, attention_implementation
from krill_worker.track import Embedding, PointPrompt

FIXTURE = "https://huggingface.co/datasets/hf-internal-testing/sam2-fixtures/resolve/main/bedroom.mp4"


def load_fixture(width: int, height: int) -> list[np.ndarray]:
    path = Path("/tmp/bedroom.mp4")
    if not path.exists():
        urllib.request.urlretrieve(FIXTURE, path)
    cap = cv2.VideoCapture(str(path))
    frames = []
    while True:
        ok, frame = cap.read()
        if not ok:
            break
        frames.append(
            cv2.cvtColor(cv2.resize(frame, (width, height)), cv2.COLOR_BGR2RGB)
        )
    # Ping-pong so long clips stay temporally continuous.
    return frames + frames[-2:0:-1]


def clip(
    fixture: list[np.ndarray], embeddings: list[Embedding] | None, n: int
) -> Iterator[tuple[np.ndarray, Embedding | None]]:
    for i in range(n):
        j = i % len(fixture)
        yield fixture[j], embeddings[j] if embeddings else None


class Stages:
    """Accumulates wall time per stage, syncing the GPU around each."""

    def __init__(self) -> None:
        self.total: dict[str, float] = defaultdict(float)
        self.calls: dict[str, int] = defaultdict(int)
        self.enabled = False

    @contextmanager
    def time(self, name: str) -> Iterator[None]:
        if not self.enabled:
            yield
            return
        torch.cuda.synchronize()
        start = time.perf_counter()
        try:
            yield
        finally:
            torch.cuda.synchronize()
            self.total[name] += time.perf_counter() - start
            self.calls[name] += 1

    def wrap(self, obj: Any, attr: str, name: str) -> None:
        fn: Callable[..., Any] = getattr(obj, attr)

        def timed(*args: Any, **kwargs: Any) -> Any:
            with self.time(name):
                return fn(*args, **kwargs)

        setattr(obj, attr, timed)

    def reset(self) -> None:
        self.total.clear()
        self.calls.clear()


def instrument(tracker: SamTracker, stages: Stages) -> None:
    model = tracker.model
    stages.wrap(type(tracker.processor), "__call__", "preprocess")
    stages.wrap(tracker.processor, "post_process_masks", "post_process_masks")
    stages.wrap(model, "get_image_features", "image encoder")
    stages.wrap(model, "_prepare_memory_conditioned_features", "memory attention")
    stages.wrap(model, "_single_frame_forward", "prompt+mask decoder")
    stages.wrap(model, "_batch_encode_memories", "memory encoder")
    stages.wrap(sam_module, "mask_box", "mask_box (cpu)")

    # The cache upload happens inside track() on a per-session object, so
    # patch the class.
    cache_cls = type(tracker.processor.init_video_session(dtype=torch.float16).cache)
    stages.wrap(cache_cls, "cache_vision_features", "cache upload")


def report_stages(stages: Stages, frames: int, loop_s: float) -> None:
    order = [
        "preprocess",
        "cache upload",
        "image encoder",
        "memory attention",
        "prompt+mask decoder",
        "memory encoder",
        "post_process_masks",
        "mask_box (cpu)",
    ]
    print(f"  {'stage':<22} {'ms/frame':>9} {'share':>6}")
    accounted = 0.0
    for name in order:
        if name not in stages.total:
            continue
        ms = stages.total[name] / frames * 1000
        accounted += stages.total[name]
        print(f"  {name:<22} {ms:>9.1f} {stages.total[name] / loop_s:>6.0%}")
    other = loop_s - accounted
    print(f"  {'other (.cpu, python)':<22} {other / frames * 1000:>9.1f} {other / loop_s:>6.0%}")


def run(
    tracker: SamTracker,
    fixture: list[np.ndarray],
    embeddings: list[Embedding] | None,
    n: int,
    stages: Stages | None,
) -> tuple[float, float, int, int, float]:
    torch.cuda.empty_cache()
    torch.cuda.reset_peak_memory_stats()
    boxes = tracker.track(clip(fixture, embeddings, n), PointPrompt(0.25, 0.73))
    start = time.perf_counter()
    found = int(next(boxes) is not None)
    click_ms = (time.perf_counter() - start) * 1000
    if stages is not None:
        stages.reset()
        stages.enabled = True
    start = time.perf_counter()
    found += sum(box is not None for box in boxes)
    loop_s = time.perf_counter() - start
    if stages is not None:
        stages.enabled = False
    peak = torch.cuda.max_memory_allocated() // 2**20
    return click_ms, (n - 1) / loop_s, peak, found, loop_s


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", default="facebook/sam2.1-hiera-small")
    parser.add_argument("--frames", type=int, nargs="+", default=[150, 450, 900])
    # The worker downsizes frames to a 1024 px long side before tracking.
    parser.add_argument("--size", default="1024x576")
    parser.add_argument("--stages-frames", type=int, default=150)
    args = parser.parse_args()

    width, height = (int(v) for v in args.size.split("x"))
    fixture = load_fixture(width, height)
    tracker = SamTracker(args.model)
    tracker.warm_up()

    attn = attention_implementation("cuda")
    print(
        f"{torch.cuda.get_device_name(0)}, {args.model}, {args.size}, {attn} attention, "
        f"{len(fixture)} fixture frames"
    )

    start = time.perf_counter()
    embeddings = [tracker.load_embedding(tracker.embed(f)) for f in fixture]
    embed_ms = (time.perf_counter() - start) / len(fixture) * 1000
    size_mb = sum(f.numel() * f.element_size() for f in embeddings[0]) / 2**20
    print(f"embed: {embed_ms:.0f} ms/frame, {size_mb:.1f} MiB/frame")

    print(
        f"\n{'mode':<10} {'frames':>6} {'click ms':>9} {'track fps':>10} {'peak MiB':>9} {'found':>6}"
    )
    for mode, embs in (("encode", None), ("embedded", embeddings)):
        for n in args.frames:
            click_ms, fps, peak, found, _ = run(tracker, fixture, embs, n, None)
            print(
                f"{mode:<10} {n:>6} {click_ms:>9.0f} {fps:>10.1f} {peak:>9} {found:>6}", flush=True
            )

    stages = Stages()
    instrument(tracker, stages)
    for mode, embs in (("encode", None), ("embedded", embeddings)):
        n = args.stages_frames
        _, fps, _, _, loop_s = run(tracker, fixture, embs, n, stages)
        print(f"\n{mode}, {n} frames, {fps:.1f} fps with per-stage syncs:")
        report_stages(stages, n - 1, loop_s)


if __name__ == "__main__":
    main()
