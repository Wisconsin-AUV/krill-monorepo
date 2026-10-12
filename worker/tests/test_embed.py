from collections.abc import Iterable, Iterator
from pathlib import Path
from typing import cast

import cv2
import numpy as np
from krill.v1 import annotation_pb2, worker_pb2

from krill_worker.api import Api
from krill_worker.main import run_embed, run_track
from krill_worker.prelabelers import Frame
from krill_worker.track import Box, Embedding, Prompt

MODEL = "facebook/sam2.1-hiera-small"


class FakeApi:
    def __init__(self) -> None:
        self.embeds: list[worker_pb2.ReportEmbedRequest] = []
        self.tracks: list[worker_pb2.ReportTrackRequest] = []

    def report_embed(self, req: worker_pb2.ReportEmbedRequest) -> None:
        self.embeds.append(req)

    def report_track(self, req: worker_pb2.ReportTrackRequest) -> None:
        self.tracks.append(req)


class FakeTracker:
    """Embeds a frame as its brightness, so tests can tell frames apart."""

    model_id = MODEL

    def __init__(self) -> None:
        self.seen: list[tuple[int, Embedding | None]] = []

    def embed(self, frame: Frame) -> bytes:
        return bytes([int(frame.mean())])

    def load_embedding(self, data: bytes) -> Embedding:
        return [data[0]]

    def track(
        self, frames: Iterable[tuple[Frame, Embedding | None]], prompt: Prompt
    ) -> Iterator[Box | None]:
        for frame, embedding in frames:
            self.seen.append((int(frame.mean()), embedding))
            yield None


def write_frames(tmp_path: Path, n: int) -> list[str]:
    urls = []
    for i in range(n):
        path = tmp_path / f"{i}.png"
        cv2.imwrite(str(path), np.full((8, 8, 3), 6 * i, np.uint8))
        urls.append(path.as_uri())
    return urls


def test_run_embed_uploads_each_frame_to_its_own_key(tmp_path: Path) -> None:
    n = 40
    urls = write_frames(tmp_path, n)
    task = worker_pb2.EmbedTask(
        id=3,
        frames=[
            worker_pb2.EmbedFrame(id=i, url=u, upload_url=f"put/{i}") for i, u in enumerate(urls)
        ],
    )
    uploads: dict[str, bytes] = {}
    api = FakeApi()

    run_embed(cast(Api, api), FakeTracker(), task, put=uploads.__setitem__)

    assert uploads == {f"put/{i}": bytes([6 * i]) for i in range(n)}
    assert [(r.task_id, r.model) for r in api.embeds] == [(3, MODEL)]


def test_run_track_uses_only_its_models_embeddings(tmp_path: Path) -> None:
    urls = write_frames(tmp_path, 3)
    (tmp_path / "e0").write_bytes(bytes([100]))
    (tmp_path / "e2").write_bytes(bytes([102]))
    frames = [
        worker_pb2.TaskFrame(id=0, url=urls[0], embedding_url=(tmp_path / "e0").as_uri()),
        # A missing embedding falls back to the encoder instead of failing.
        worker_pb2.TaskFrame(id=1, url=urls[1], embedding_url=(tmp_path / "e1").as_uri()),
        worker_pb2.TaskFrame(id=2, url=urls[2], embedding_url=(tmp_path / "e2").as_uri()),
    ]
    prompt = annotation_pb2.Point(x=0.5, y=0.5)

    tracker = FakeTracker()
    task = worker_pb2.TrackTask(id=1, frames=frames, point=prompt, embedding_model=MODEL)
    run_track(cast(Api, FakeApi()), tracker, task)
    assert tracker.seen == [(0, [100]), (6, None), (12, [102])]

    tracker = FakeTracker()
    task = worker_pb2.TrackTask(id=2, frames=frames, point=prompt, embedding_model="facebook/sam3")
    run_track(cast(Api, FakeApi()), tracker, task)
    assert tracker.seen == [(0, None), (6, None), (12, None)]
