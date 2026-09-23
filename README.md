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

The API image bundles the web app, so the full stack is served from `:8080`. Set `KRILL_S3_PUBLIC_ENDPOINT` to a MinIO address browsers can reach when it is not `localhost:9000`.

## Labeling workflow

1. **Labels**: create label types, or add the starter set. Attributes such as size or color are set once per track.
2. **Videos**: upload footage. Frames are extracted and split into 10 to 20 second clips.
3. **Clips**: pick a type with 1 to 9 and drag to draw. Step with J and K, press C to copy boxes from the previous frame, and press Space when every object in the frame has a box (E if there are none). Press ? for all shortcuts.
4. **Exports**: preview and download a YOLO dataset split by video.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md).


## License

Copyright © 2026 Wisconsin Autonomous Underwater Vehicles, Camden Rush, and contributors.

Code released under the [MIT License](./LICENSE.md).
