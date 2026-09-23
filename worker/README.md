# krill worker

Python GPU worker. Runs SAM 3 / SAM 2.1, YOLO, and classic CV pre-labelers.

```sh
uv sync            # base + dev deps (no torch)
uv run pytest
uv run ruff check . && uv run ruff format --check . && uv run mypy src
```

Torch is provided by the Docker base image (ROCm 6.4.1 for the RX 6600, CUDA for NVIDIA).
For local GPU work on Linux, install the matching torch wheel yourself, then `uv sync --extra ml`.
