# M0: RX 6600 validation

Benchmarks for the GPU worker on the club's RX 6600 (8 GB). Build the worker image first with `just build`, then run the commands at the top of each script.

## Results

Measured 2026-09-24 on ROCm 6.4.1 with `HSA_OVERRIDE_GFX_VERSION=10.3.0`, PyTorch 2.6, and transformers 5.17.

### SAM video tracking

`sam_video.py`, one object, 1080p frames. Click is the latency of the first frame after warm-up.

| Model | Click | Tracking | Peak VRAM |
|---|---|---|---|
| SAM 2.1 tiny | 110 ms | 4.4 fps | 1.2 GB |
| SAM 2.1 small | 120 ms | 4.2 fps | 1.2 GB |
| SAM 2.1 base+ | 190 ms | 3.2 fps | 1.4 GB |
| SAM 3 | not run | | |

VRAM stays flat from 150 to 1200 frames (20 s at 60 fps), so clip length is not a limit. Speed is. A 15 s clip at 30 fps takes about 1 min 45 s with SAM 2.1 small, and boxes stream into the clip as they are tracked.

SAM 3 was not tested because `facebook/sam3` is gated on Hugging Face. It uses the same code path.

### YOLO

`yolo_epoch.py`, COCO128, batch 16, 640 px. Inference is one 1080p frame.

| Model | Training | Peak VRAM | Inference |
|---|---|---|---|
| YOLO11n | 53 img/s | 3.4 GB | 8.5 ms |
| YOLO11s | 31 img/s | 4.4 GB | 8.4 ms |
| YOLO26n | 41 img/s | 3.9 GB | 9.9 ms |
| YOLO26s | 22 img/s | 5.2 GB | 9.8 ms |

At 31 img/s, 100 epochs of YOLO11s on 5,000 images takes about 4.5 hours, which fits a nightly retrain.

## Findings

- PyTorch has no fused SDPA kernel for gfx1030, and the math fallback runs in fp32. The worker uses eager fp16 attention on such cards, which makes SAM tracking about 45% faster.
- The SAM video session keeps every frame and its memory by default. The worker streams frames and drops state the tracker no longer reads, which gives identical boxes with flat memory.
- The base image has no `render` group, so compose no longer adds it. The worker runs as root and can open `/dev/kfd` without it.
- Installing `ffmpeg` from apt makes torchaudio segfault on exit, and upgrading numpy to 2.x breaks the base image's pandas. The worker image no longer installs ffmpeg and keeps the base image's torch and numpy versions.
- YOLO training needs `--shm-size` above Docker's 64 MB default for its dataloader workers.
