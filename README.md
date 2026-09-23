# Krill

Labeling tool for building and validating WAUV's RoboSub YOLO datasets. This app aims to help humans be as efficient as possible.  See the [design doc](docs/design-doc.md).

## Quick start

```sh
cp .env.example .env
just setup
just dev      # Postgres :5432, MinIO :9000 (console :9001)
just api      # :8080
just web      # :5173
```

Full stack on the GPU box:

```sh
docker compose -f deploy/docker-compose.yml --profile app --profile gpu up -d
```

Build the worker for CUDA Nvidia with `just build-worker-cuda`.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md).


## License

Copyright © 2026 Wisconsin Autonomous Underwater Vehicles, Camden Rush, and contributors.

Code released under the [MIT License](./LICENSE.md).
