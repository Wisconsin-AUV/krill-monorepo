# M0: RX 6600 validation

Benchmarks for the GPU worker on the club's RX 6600 (8 GB). Build the worker image first with `just build`, then run the commands at the top of each script.

## Results

Measured 2026-09-24 (SAM tracking re-measured 2026-10-14) on ROCm 6.4.1 with `HSA_OVERRIDE_GFX_VERSION=10.3.0`, PyTorch 2.6, and transformers 5.19.

### SAM video tracking

`sam_video.py`, one object, frames downsized to 1024×576 as the worker does. Click is the latency of the first frame after warm-up. "Embedded" tracks with precomputed embeddings, which is the production path; "encode" runs the image encoder on every frame.

| Model | Mode | Click | Tracking | Peak VRAM |
|---|---|---|---|---|
| SAM 2.1 small | embedded | 25 ms | 10.4 fps | 0.8 GB |
| SAM 2.1 small | encode | 110 ms | 5.4 fps | 0.8 GB |
| SAM 2.1 small, eager attention (before 2026-10-14) | embedded | 26 ms | 7.4 fps | 1.3 GB |
| SAM 2.1 small, eager attention (before 2026-10-14) | encode | 120 ms | 4.2 fps | 1.3 GB |

Embedding a frame takes 110 ms and stores 8 MiB. VRAM stays flat from 150 to 900 frames, so clip length is not a limit. A 15 s clip at 30 fps now tracks in about 45 s with embeddings.

Per tracked frame with embeddings (2026-10-14, fp16 attention):

| Stage | ms | Share |
|---|---|---|
| memory attention | 77 | 80% |
| memory encoder | 9 | 10% |
| prompt + mask decoder | 6 | 6% |
| preprocessing, cache upload, mask post-processing, box | 3 | 4% |

Memory attention is 4096 frame tokens attending to about 29k memory tokens (7 memories × 4096 plus object pointers) in four layers. Its matmuls and softmax take about 58 ms of the 77 ms and run near the card's matmul rate, so the rest of the frame is at most a few ms of further savings.

`sam_attention.py` compares attention variants and smaller memory banks on the same clip against eager:

| Variant | Tracking | Boxes vs eager |
|---|---|---|
| eager (transformers) | 7.9 fps | identical |
| SDPA (fp32 math fallback) | 5.0 fps | mean IoU 0.998 |
| worker fp16 attention | 10.5 fps | mean IoU 0.989 |
| worker fp16 attention, 1024-query chunks | 10.1 fps | mean IoU 0.989 |
| `num_maskmem` 4, 8 pointers | 9.2 fps | mean IoU 0.85, loses the object |
| `num_maskmem` 2, 4 pointers | 14.0 fps | mean IoU 0.65, loses the object |

The IoU below 1 for the fp16 variants is one frame where the mask flips between part of the object and all of it. Eager and SDPA flip against each other the same way on a different frame, so this is the model's own instability on this clip rather than an error in the attention.

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

- PyTorch has no fused SDPA kernel for gfx1030, and the math fallback runs in fp32. The worker registers its own fp16 attention on such cards (`krill_worker.attention`): it folds the score scaling into the queries, keeps the softmax in fp16, and applies SAM's rotary embedding to the memory keys in fp16 instead of fp32. That is 2× SDPA and 40% faster than transformers' eager attention, with 0.5 GB less VRAM, because the 4096×29k score matrix is never scaled, upcast, or built in fp32.
- Chunking the queries and `torch.compile` (on the rotary embedding or the whole memory-attention layer) gave nothing further on this card. Fewer memories (`num_maskmem` below 7) speed tracking up proportionally but lose the object within a few seconds, so they are not an option.
- The SAM video session keeps every frame and its memory by default. The worker streams frames and drops state the tracker no longer reads, which gives identical boxes with flat memory.
- The base image has no `render` group, so compose no longer adds it. The worker runs as root and can open `/dev/kfd` without it.
- Installing `ffmpeg` from apt makes torchaudio segfault on exit, and upgrading numpy to 2.x breaks the base image's pandas. The worker image no longer installs ffmpeg and keeps the base image's torch and numpy versions.
- YOLO training needs `--shm-size` above Docker's 64 MB default for its dataloader workers.
