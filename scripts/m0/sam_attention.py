"""M0: compare memory-attention implementations for SAM tracking with cached embeddings.

Memory attention is most of the per-frame time once embeddings are cached
(see sam_video.py). This compares the worker's fp16 attention with eager and
SDPA, tries chunking it, and tries smaller memory banks, checking every
variant's boxes against eager. Run inside the worker image like sam_video.py:

    python /m0/sam_attention.py --model facebook/sam2.1-hiera-small
"""

import argparse
import time
from collections.abc import Callable
from typing import Any

import numpy as np
import torch
from torch import nn
from transformers import AttentionInterface

from krill_worker import attention
from krill_worker.sam import SamTracker
from krill_worker.track import Box, PointPrompt
from sam_video import clip, load_fixture


def chunked_attention(chunk: int) -> Callable[..., Any]:
    """The worker's fp16 attention over query chunks, so the full score matrix is never built."""

    def forward(
        module: nn.Module,
        query: torch.Tensor,
        key: torch.Tensor,
        value: torch.Tensor,
        attention_mask: torch.Tensor | None,
        scaling: float,
        dropout: float = 0.0,
        **kwargs: Any,
    ) -> tuple[torch.Tensor, None]:
        outs = [
            attention.attention_forward(
                module, query[:, :, i : i + chunk], key, value, attention_mask, scaling
            )[0]
            for i in range(0, query.shape[2], chunk)
        ]
        return torch.cat(outs, dim=1), None

    return forward


def iou(a: Box | None, b: Box | None) -> float:
    if a is None or b is None:
        return float(a is None and b is None)
    ax0, ay0, aw, ah = a
    bx0, by0, bw, bh = b
    ix = max(0.0, min(ax0 + aw, bx0 + bw) - max(ax0, bx0))
    iy = max(0.0, min(ay0 + ah, by0 + bh) - max(ay0, by0))
    inter = ix * iy
    return inter / (aw * ah + bw * bh - inter)


def run(tracker: SamTracker, fixture: list[np.ndarray], embeddings: list[Any], n: int) -> tuple[float, list[Box | None]]:
    torch.cuda.synchronize()
    boxes = tracker.track(clip(fixture, embeddings, n), PointPrompt(0.25, 0.73))
    out = [next(boxes)]
    start = time.perf_counter()
    out.extend(boxes)
    torch.cuda.synchronize()
    return (n - 1) / (time.perf_counter() - start), out


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", default="facebook/sam2.1-hiera-small")
    parser.add_argument("--frames", type=int, default=150)
    parser.add_argument("--size", default="1024x576")
    args = parser.parse_args()

    width, height = (int(v) for v in args.size.split("x"))
    fixture = load_fixture(width, height)
    tracker = SamTracker(args.model)
    tracker.warm_up()
    model = tracker.model
    embeddings = [tracker.load_embedding(tracker.embed(f)) for f in fixture[: args.frames]]
    print(f"{torch.cuda.get_device_name(0)}, {args.model}, {args.size}, {args.frames} frames")

    attention.register()
    AttentionInterface.register("chunk1024", chunked_attention(1024))

    default_maskmem = model.num_maskmem
    default_pointers = model.config.max_object_pointers_in_encoder
    baseline: list[Box | None] | None = None
    print(f"\n{'variant':<34} {'fps':>6} {'mean IoU':>9} {'same':>5} {'min IoU':>8}")

    def trial(label: str) -> None:
        nonlocal baseline
        run(tracker, fixture, embeddings, 10)  # warm kernels for this variant
        fps, boxes = run(tracker, fixture, embeddings, args.frames)
        if baseline is None:
            baseline = boxes
        ious = [iou(a, b) for a, b in zip(boxes, baseline, strict=True)]
        same = sum(a == b for a, b in zip(boxes, baseline, strict=True))
        print(
            f"{label:<34} {fps:>6.1f} {np.mean(ious):>9.3f} {same:>5} {min(ious):>8.3f}", flush=True
        )

    attn_variants = ["eager", "sdpa", attention.IMPLEMENTATION, "chunk1024"]
    for attn in attn_variants:
        try:
            model.set_attn_implementation(attn)
        except Exception as e:  # noqa: BLE001
            print(f"{attn:<34} skipped: {e}")
            continue
        trial(attn)

    print("\nsmaller memory banks (eager attention, boxes vs eager with default bank):")
    model.set_attn_implementation("eager")
    for maskmem, pointers in [(7, 16), (4, 16), (4, 8), (3, 8), (2, 4)]:
        model.num_maskmem = maskmem
        model.config.max_object_pointers_in_encoder = pointers
        tracker.memory_frames = max(maskmem, pointers)
        trial(f"num_maskmem={maskmem} pointers={pointers}")
    model.num_maskmem = default_maskmem
    model.config.max_object_pointers_in_encoder = default_pointers
    tracker.memory_frames = max(default_maskmem, default_pointers)


if __name__ == "__main__":
    main()
