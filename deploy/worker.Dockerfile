# ROCm is pinned to 6.4.1. 6.4.3+ crashes with HSA_OVERRIDE_GFX_VERSION on gfx1032.
ARG BASE=rocm/pytorch:rocm6.4.1_ubuntu24.04_py3.12_pytorch_release_2.6.0
FROM ${BASE}

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    UV_SYSTEM_PYTHON=1

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

WORKDIR /app
COPY worker/pyproject.toml worker/README.md ./
COPY worker/src ./src
COPY worker/gen ./gen

# Pin the base image's torch and numpy stack so our deps install around it.
RUN pip list --format=freeze | grep -iE '^(torch|torchvision|numpy|pandas|scipy|numba|pillow)==' > /tmp/base.txt \
    && uv pip install --system -c /tmp/base.txt ".[ml]"

CMD ["krill-worker"]
