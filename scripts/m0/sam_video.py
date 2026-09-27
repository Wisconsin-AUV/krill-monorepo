"""M0: measure SAM click latency, tracking speed, and VRAM on the local GPU.

Runs the worker's tracker on a public test video looped to clip length. Run
inside the worker image:

    docker run --rm --device /dev/kfd --device /dev/dri -e HSA_OVERRIDE_GFX_VERSION=10.3.0 \
        -v krill-hf:/root/.cache/huggingface -v ./scripts/m0:/m0 krill-worker \
        python /m0/sam_video.py --model facebook/sam2.1-hiera-small
"""

import argparse
import time
import urllib.request
from collections.abc import Iterator
from pathlib import Path

import cv2
import numpy as np
import torch

from krill_worker.sam import SamTracker, attention_implementation
from krill_worker.track import PointPrompt

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


def clip(fixture: list[np.ndarray], n: int) -> Iterator[np.ndarray]:
    for i in range(n):
        yield fixture[i % len(fixture)]


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", default="facebook/sam2.1-hiera-small")
    parser.add_argument("--frames", type=int, nargs="+", default=[150, 450, 900, 1200])
    parser.add_argument("--size", default="1920x1080")
    args = parser.parse_args()

    width, height = (int(v) for v in args.size.split("x"))
    fixture = load_fixture(width, height)
    tracker = SamTracker(args.model)
    tracker.warm_up()
    prompt = PointPrompt(0.25, 0.73)

    attn = attention_implementation("cuda")
    print(
        f"{torch.cuda.get_device_name(0)}, {args.model}, {args.size}, {attn} attention"
    )
    print(
        f"{'frames':>6} {'click ms':>9} {'track fps':>10} {'peak MiB':>9} {'found':>6}"
    )
    for n in args.frames:
        torch.cuda.empty_cache()
        torch.cuda.reset_peak_memory_stats()
        boxes = tracker.track(clip(fixture, n), prompt)
        start = time.perf_counter()
        found = next(boxes) is not None
        click_ms = (time.perf_counter() - start) * 1000
        start = time.perf_counter()
        found += sum(box is not None for box in boxes)
        fps = (n - 1) / (time.perf_counter() - start)
        peak = torch.cuda.max_memory_allocated() // 2**20
        print(f"{n:>6} {click_ms:>9.0f} {fps:>10.1f} {peak:>9} {found:>6}", flush=True)


if __name__ == "__main__":
    main()
