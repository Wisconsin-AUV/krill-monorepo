import logging
import os
import time
import urllib.request
from collections import deque
from collections.abc import Callable, Iterable, Iterator
from concurrent.futures import Future, ThreadPoolExecutor
from typing import Protocol, TypeVar

import cv2
import numpy as np
from krill.v1 import annotation_pb2, worker_pb2

from krill_worker import __version__
from krill_worker.api import Api, ApiError
from krill_worker.prelabelers import Frame
from krill_worker.track import Box, BoxPrompt, Embedding, PointPrompt, Prompt

log = logging.getLogger(__name__)

DEFAULT_MODEL = "facebook/sam2.1-hiera-small"
# SAM resizes to about 1024 pixels, so larger frames only cost decode time.
MAX_SIDE = 1024
REPORT_EVERY = 2.0
RETRY_DELAY = 5.0
# Downloads run this many frames ahead of the GPU. Embeddings are about 8 MB
# each, so reading a whole clip ahead would hold gigabytes.
PREFETCH = 16

T = TypeVar("T")
R = TypeVar("R")


class Tracker(Protocol):
    model_id: str

    def track(
        self, frames: Iterable[tuple[Frame, Embedding | None]], prompt: Prompt
    ) -> Iterator[Box | None]: ...

    def embed(self, frame: Frame) -> bytes: ...

    def load_embedding(self, data: bytes) -> Embedding: ...


def decode_frame(data: bytes) -> Frame:
    bgr = cv2.imdecode(np.frombuffer(data, np.uint8), cv2.IMREAD_COLOR)
    if bgr is None:
        raise ValueError("frame is not a valid image")
    h, w = bgr.shape[:2]
    scale = MAX_SIDE / max(h, w)
    if scale < 1:
        bgr = cv2.resize(bgr, (round(w * scale), round(h * scale)), interpolation=cv2.INTER_AREA)
    return np.asarray(cv2.cvtColor(bgr, cv2.COLOR_BGR2RGB), dtype=np.uint8)


def fetch(url: str) -> bytes:
    with urllib.request.urlopen(url, timeout=30) as res:
        data: bytes = res.read()
        return data


def upload(url: str, data: bytes) -> None:
    req = urllib.request.Request(
        url, data=data, method="PUT", headers={"Content-Type": "application/octet-stream"}
    )
    with urllib.request.urlopen(req, timeout=30):
        pass


def prefetch(fn: Callable[[T], R], items: Iterable[T]) -> Iterator[R]:
    """Yield fn of each item in order, computing up to PREFETCH ahead in threads."""
    with ThreadPoolExecutor(8) as pool:
        pending: deque[Future[R]] = deque()
        for item in items:
            pending.append(pool.submit(fn, item))
            if len(pending) > PREFETCH:
                yield pending.popleft().result()
        while pending:
            yield pending.popleft().result()


def run_track(api: Api, tracker: Tracker, task: worker_pb2.TrackTask) -> None:
    prompt: Prompt
    if task.HasField("point"):
        prompt = PointPrompt(task.point.x, task.point.y)
    else:
        b = task.box
        prompt = BoxPrompt((b.x, b.y, b.width, b.height))

    use_embeddings = task.embedding_model == tracker.model_id

    def load(f: worker_pb2.TaskFrame) -> tuple[Frame, Embedding | None]:
        frame = decode_frame(fetch(f.url))
        if not (use_embeddings and f.embedding_url):
            return frame, None
        try:
            return frame, tracker.load_embedding(fetch(f.embedding_url))
        except (OSError, ValueError) as e:
            log.warning("embedding for frame %d unusable, encoding it instead: %s", f.id, e)
            return frame, None

    frames = prefetch(load, task.frames)
    ids = [f.id for f in task.frames]
    start = last_report = time.monotonic()
    batch: list[worker_pb2.TrackedBox] = []
    found = 0
    for frame_id, box in zip(ids, tracker.track(frames, prompt), strict=True):
        if box is not None:
            x, y, w, h = box
            box_msg = annotation_pb2.Box(x=x, y=y, width=w, height=h)
            batch.append(worker_pb2.TrackedBox(frame_id=frame_id, box=box_msg))
            found += 1
        if time.monotonic() - last_report >= REPORT_EVERY:
            api.report_track(
                worker_pb2.ReportTrackRequest(task_id=task.id, model=tracker.model_id, boxes=batch)
            )
            batch = []
            last_report = time.monotonic()
    api.report_track(
        worker_pb2.ReportTrackRequest(
            task_id=task.id, model=tracker.model_id, boxes=batch, done=True
        )
    )
    elapsed = time.monotonic() - start
    log.info(
        "tracked task %d: object on %d of %d frames in %.1fs", task.id, found, len(ids), elapsed
    )


def run_embed(
    api: Api,
    tracker: Tracker,
    task: worker_pb2.EmbedTask,
    put: Callable[[str, bytes], None] = upload,
) -> None:
    start = time.monotonic()
    frames = prefetch(lambda f: decode_frame(fetch(f.url)), task.frames)
    with ThreadPoolExecutor(4) as uploads:
        pending = [
            uploads.submit(put, f.upload_url, tracker.embed(frame))
            for f, frame in zip(task.frames, frames, strict=True)
        ]
        for p in pending:
            p.result()
    api.report_embed(worker_pb2.ReportEmbedRequest(task_id=task.id, model=tracker.model_id))
    log.info(
        "embedded task %d: %d frames in %.1fs",
        task.id,
        len(task.frames),
        time.monotonic() - start,
    )


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    logging.getLogger("httpx").setLevel(logging.WARNING)
    log.info("krill-worker %s starting", __version__)
    # Imported here so everything else works without torch installed.
    from krill_worker.sam import SamTracker

    api = Api(
        os.environ.get("KRILL_API_URL", "http://localhost:8080"), os.environ["KRILL_WORKER_TOKEN"]
    )
    tracker = SamTracker(os.environ.get("KRILL_SAM_MODEL", DEFAULT_MODEL))
    tracker.warm_up()
    log.info("loaded %s", tracker.model_id)

    while True:
        try:
            res = api.next_task()
        except (ApiError, OSError) as e:
            log.warning("poll failed, retrying in %.0fs: %s", RETRY_DELAY, e)
            time.sleep(RETRY_DELAY)
            continue
        if not (res.HasField("track") or res.HasField("embed")):
            continue
        task_id = res.track.id or res.embed.id
        try:
            if res.HasField("track"):
                run_track(api, tracker, res.track)
            else:
                run_embed(api, tracker, res.embed)
        except ApiError as e:
            log.warning("task %d dropped by the API: %s", task_id, e)
        except Exception as e:
            log.exception("task %d failed", task_id)
            try:
                api.fail_task(task_id, str(e))
            except (ApiError, OSError):
                log.exception("report failure for task %d", task_id)


if __name__ == "__main__":
    main()
