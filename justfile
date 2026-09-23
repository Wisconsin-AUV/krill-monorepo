set dotenv-load

default:
    @just --list

# generate protobuf code for Go, TS, and Python
gen:
    cd proto && buf lint && buf generate

# install dependencies for every project
setup:
    cd web && pnpm install
    cd api && go mod download
    cd worker && uv sync

# start Postgres + MinIO, then run api and web on the host
dev:
    docker compose -f deploy/docker-compose.yml up -d postgres minio
    @echo "Postgres :5432, MinIO :9000 (console :9001). Run 'just api' and 'just web' in separate shells."

api:
    cd api && go run ./cmd/api

web:
    cd web && pnpm dev

down:
    docker compose -f deploy/docker-compose.yml down

# lint everything
lint:
    cd proto && buf lint
    cd api && golangci-lint run
    cd web && pnpm lint && pnpm format:check
    cd worker && uv run ruff check . && uv run ruff format --check . && uv run mypy src

# auto-format everything
fmt:
    cd api && golangci-lint fmt
    cd web && pnpm format
    cd worker && uv run ruff format . && uv run ruff check --fix .

test:
    cd api && go test ./...
    cd web && pnpm test
    cd worker && uv run pytest

# build web bundle and api + ROCm worker images
build version="dev":
    cd web && pnpm build
    docker build -f deploy/api.Dockerfile --build-arg VERSION={{version}} -t krill-api:{{version}} .
    docker build -f deploy/worker.Dockerfile -t krill-worker:{{version}}-rocm .

build-worker-cuda version="dev":
    docker build -f deploy/worker.Dockerfile --build-arg BASE=pytorch/pytorch:2.6.0-cuda12.4-cudnn9-runtime -t krill-worker:{{version}}-cuda .
