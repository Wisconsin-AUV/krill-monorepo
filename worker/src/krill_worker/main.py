import gc
import logging
import os
import time
from collections.abc import Callable, Iterable, Iterator
from concurrent.futures import ThreadPoolExecutor
from typing import Any, Protocol

from krill.v1 import annotation_pb2, pipeline_pb2, pipeline_worker_pb2, worker_pb2

from krill_worker import __version__
from krill_worker.api import Api, ApiError
from krill_worker.frames import decode_frame, fetch, prefetch, upload
from krill_worker.heartbeat import Heartbeat
from krill_worker.logs import LogBuffer
from krill_worker.models import ModelLoadError, Models
from krill_worker.pipeline import YoloModels, run_prelabel, run_preview
from krill_worker.prelabelers import Frame
from krill_worker.prelabelers.plugins import PLUGINS
from krill_worker.stats import accelerator, gpu_stats
from krill_worker.track import Box, BoxPrompt, Embedding, PointPrompt, Prompt, drop_cut_off
from krill_worker.train import run_train

log = logging.getLogger(__name__)

REPORT_EVERY = 2.0
RETRY_DELAY = 5.0


class Tracker(Protocol):
    model_id: str
    supports_exemplars: bool

    def track(
        self, frames: Iterable[tuple[Frame, Embedding | None]], prompt: Prompt
    ) -> Iterator[Box | None]: ...

    def find_similar(self, frame: Frame, box: Box) -> list[Box]: ...

    def embed(self, frame: Frame) -> bytes: ...

    def load_embedding(self, data: bytes) -> Embedding: ...


class TaskCancelledError(Exception):
    """The API stopped waiting on the task, so its results would be thrown away."""


def never() -> bool:
    return False


def run_track(
    api: Api, tracker: Tracker, task: worker_pb2.TrackTask, cancelled: Callable[[], bool] = never
) -> None:
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
    for frame_id, box in zip(ids, drop_cut_off(tracker.track(frames, prompt)), strict=True):
        if cancelled():
            raise TaskCancelledError
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


def run_exemplar(api: Api, tracker: Tracker, task: worker_pb2.ExemplarTask) -> None:
    if not tracker.supports_exemplars:
        raise ValueError(
            f"Finding similar objects needs a SAM 3 model, but the GPU worker runs "
            f"{tracker.model_id}. Choose SAM 3 as the SAM model in the admin panel."
        )
    start = time.monotonic()
    frame = decode_frame(fetch(task.frame.url))
    b = task.box
    found = tracker.find_similar(frame, (b.x, b.y, b.width, b.height))
    boxes = [annotation_pb2.Box(x=x, y=y, width=w, height=h) for x, y, w, h in found]
    api.report_exemplars(
        worker_pb2.ReportExemplarsRequest(task_id=task.id, model=tracker.model_id, boxes=boxes)
    )
    elapsed = time.monotonic() - start
    log.info("exemplar task %d: found %d objects in %.1fs", task.id, len(boxes), elapsed)


def run_embed(
    api: Api,
    tracker: Tracker,
    task: worker_pb2.EmbedTask,
    put: Callable[[str, bytes], None] = upload,
    cancelled: Callable[[], bool] = never,
) -> None:
    start = time.monotonic()
    frames = prefetch(lambda f: decode_frame(fetch(f.url)), task.frames)
    with ThreadPoolExecutor(4) as uploads:
        pending = []
        for f, frame in zip(task.frames, frames, strict=True):
            if cancelled():
                raise TaskCancelledError
            pending.append(uploads.submit(put, f.upload_url, tracker.embed(frame)))
        for p in pending:
            p.result()
    api.report_embed(worker_pb2.ReportEmbedRequest(task_id=task.id, model=tracker.model_id))
    log.info(
        "embedded task %d: %d frames in %.1fs",
        task.id,
        len(task.frames),
        time.monotonic() - start,
    )


Task = (
    worker_pb2.TrackTask
    | worker_pb2.ExemplarTask
    | worker_pb2.EmbedTask
    | pipeline_pb2.TrainTask
    | pipeline_pb2.PrelabelTask
    | pipeline_pb2.PreviewTask
)


def run_task(
    api: Api,
    tracker: Tracker,
    res: worker_pb2.NextTaskResponse,
    cancelled: Callable[[], bool] = never,
) -> None:
    kind = res.WhichOneof("task")
    if kind is None:
        return
    task: Task = getattr(res, kind)
    try:
        if kind == "track":
            run_track(api, tracker, res.track, cancelled)
        elif kind == "exemplar":
            run_exemplar(api, tracker, res.exemplar)
        elif kind == "embed":
            run_embed(api, tracker, res.embed, cancelled=cancelled)
        else:
            raise ValueError(f"this worker cannot run {kind} tasks")
    except TaskCancelledError:
        log.info("stopped task %d", task.id)
    except ApiError as e:
        log.warning("task %d dropped by the API: %s", task.id, e)
    except Exception as e:
        log.exception("task %d failed", task.id)
        fail(api, task.id, str(e))


def fail(api: Api, task_id: int, error: str) -> None:
    try:
        api.fail_task(task_id, error)
    except (ApiError, OSError):
        log.exception("report failure for task %d", task_id)


def serve(
    api: Api,
    models: Models[Any],
    yolo: YoloModels,
    heartbeat: Heartbeat,
    res: worker_pb2.NextTaskResponse,
) -> None:
    """Run the task, loading the SAM model first for the tasks that need it.

    Training serves the tasks that come in meanwhile through this too, so
    they nest inside it.
    """
    kind = res.WhichOneof("task")
    if kind is None:
        return
    task: Task = getattr(res, kind)
    with heartbeat.working(task.id):
        try:
            if kind == "train":
                run_train(api, res.train, lambda r: serve(api, models, yolo, heartbeat, r))
            elif kind == "prelabel":
                run_prelabel(api, res.prelabel, yolo)
            elif kind == "preview":
                run_preview(api, res.preview)
            else:
                models.run(lambda tracker: run_task(api, tracker, res, heartbeat.cancelled))
        except ModelLoadError as e:
            log.exception("task %d failed", task.id)
            fail(api, task.id, str(e))
        except ApiError as e:
            log.warning("task %d dropped by the API: %s", task.id, e)
        except Exception as e:
            log.exception("task %d failed", task.id)
            fail(api, task.id, str(e))


def register_prelabelers(api: Api) -> bool:
    """Offer the CV plugins to the admin panel. Returns whether it worked."""
    req = pipeline_worker_pb2.RegisterPrelabelersRequest(
        prelabelers=[p.spec() for p in PLUGINS.values()]
    )
    try:
        api.register_prelabelers(req)
    except (ApiError, OSError) as e:
        log.warning("register pre-labelers: %s", e)
        return False
    return True


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
    logging.getLogger("httpx").setLevel(logging.WARNING)
    logs = LogBuffer()
    logging.getLogger().addHandler(logs)
    log.info("krill-worker %s starting", __version__)
    # Imported here so everything else works without torch installed.
    import torch

    from krill_worker.sam import SamTracker

    def load(model_id: str) -> SamTracker:
        tracker = SamTracker(model_id)
        tracker.warm_up()
        return tracker

    def memory() -> tuple[int, int]:
        if not torch.cuda.is_available():
            return 0, 0
        return torch.cuda.memory_allocated(), torch.cuda.memory_reserved()

    def release() -> None:
        gc.collect()
        if torch.cuda.is_available():
            torch.cuda.empty_cache()

    api = Api(
        os.environ.get("KRILL_API_URL", "http://localhost:8080"), os.environ["KRILL_WORKER_TOKEN"]
    )
    pinned = os.environ.get("KRILL_SAM_MODEL", "").strip()
    models = Models(load, pinned, memory, release)
    heartbeat = Heartbeat(api, models, accelerator(torch), lambda: gpu_stats(torch), logs)
    # One heartbeat first, so the worker loads the model the API asks for.
    try:
        heartbeat.beat()
    except (ApiError, OSError) as e:
        log.warning("first heartbeat failed, loading %s: %s", models.wanted, e)
    heartbeat.start()
    models.warm_up()

    yolo = YoloModels()
    registered = False
    while True:
        models.sync()
        registered = registered or register_prelabelers(api)
        try:
            res = api.next_task()
        except (ApiError, OSError) as e:
            log.warning("poll failed, retrying in %.0fs: %s", RETRY_DELAY, e)
            time.sleep(RETRY_DELAY)
            continue
        serve(api, models, yolo, heartbeat, res)


if __name__ == "__main__":
    main()
