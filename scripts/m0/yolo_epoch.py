"""M0: measure YOLO training and inference speed on the local GPU.

Run inside the worker image (training dataloaders need more than Docker's
default 64 MB of shared memory):

    docker run --rm --device /dev/kfd --device /dev/dri -e HSA_OVERRIDE_GFX_VERSION=10.3.0 \
        --shm-size 2g -v krill-yolo:/root/.config/Ultralytics -v ./scripts/m0:/m0 krill-worker \
        python /m0/yolo_epoch.py yolo11n.pt yolo11s.pt
"""

import argparse
import time

import torch
from ultralytics import YOLO


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("models", nargs="+")
    parser.add_argument("--data", default="coco128.yaml")
    parser.add_argument("--epochs", type=int, default=3)
    parser.add_argument("--batch", type=int, default=16)
    parser.add_argument("--imgsz", type=int, default=640)
    args = parser.parse_args()

    rows = []
    for name in args.models:
        model = YOLO(name)
        epoch_ends: list[float] = []
        model.add_callback("on_train_epoch_end", lambda _: epoch_ends.append(time.perf_counter()))
        torch.cuda.reset_peak_memory_stats()
        model.train(
            data=args.data,
            epochs=args.epochs,
            batch=args.batch,
            imgsz=args.imgsz,
            device=0,
            workers=4,
            plots=False,
            val=False,
            project="/tmp/runs",
            exist_ok=True,
        )
        # The first epoch includes warm-up, so it is left out.
        images = len(model.trainer.train_loader.dataset) * (len(epoch_ends) - 1)
        train_rate = images / (epoch_ends[-1] - epoch_ends[0])
        train_peak = torch.cuda.max_memory_reserved() // 2**20

        frame = torch.randint(0, 255, (1080, 1920, 3), dtype=torch.uint8).numpy()
        model.predict(frame, device=0, verbose=False)
        start = time.perf_counter()
        for _ in range(20):
            model.predict(frame, device=0, verbose=False)
        infer_ms = (time.perf_counter() - start) / 20 * 1000
        rows.append((name, train_rate, train_peak, infer_ms))

    print(
        f"\n{torch.cuda.get_device_name(0)}, {args.data}, batch {args.batch}, imgsz {args.imgsz}"
    )
    print(f"{'model':>12} {'train img/s':>12} {'peak MiB':>9} {'infer ms':>9}")
    for name, train_rate, peak, infer_ms in rows:
        print(f"{name:>12} {train_rate:>12.1f} {peak:>9} {infer_ms:>9.1f}")


if __name__ == "__main__":
    main()
