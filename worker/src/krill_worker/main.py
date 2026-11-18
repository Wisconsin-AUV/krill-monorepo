import gc
import logging
import os
import time
from collections import deque
from collections.abc import Callable, Iterable, Iterator, Sequence
from concurrent.futures import ThreadPoolExecutor
from typing import Any, Protocol

import cv2
import numpy as np
from krill.v1 import annotation_pb2, pipeline_pb2, pipeline_worker_pb2, worker_pb2

from krill_worker import __version__
from krill_worker.api import Api, ApiError
from krill_worker.frames import decode_frame, fetch, prefetch, upload
from krill_worker.geometry import to_proto
from krill_worker.heartbeat import Heartbeat
from krill_worker.keypoints import (
    KeypointTracker,
    Schema,
    Segment,
    find_print,
    from_proto,
    load_design,
    place_centers,
    print_visibility,
)
from krill_worker.keypoints import to_proto as keypoints_proto
from krill_worker.logs import LogBuffer
from krill_worker.models import ModelLoadError, Models
from krill_worker.orient import fit, load_reference
from krill_worker.pipeline import YoloModels, run_inspect, run_prelabel, run_preview
from krill_worker.prelabelers import Frame
from krill_worker.prelabelers.plugins import PLUGINS
from krill_worker.stats import accelerator, gpu_stats
from krill_worker.track import (
    Box,
    BoxPrompt,
    CutOff,
    Embedding,
    Mask,
    PointPrompt,
    Prompt,
    mask_box,
)
from krill_worker.train import run_train

log = logging.getLogger(__name__)

REPORT_EVERY = 2.0
RETRY_DELAY = 5.0


class Tracker(Protocol):
    model_id: str
    supports_exemplars: bool

    def track(
        self, frames: Iterable[tuple[Frame, Embedding | None]], prompt: Prompt
    ) -> Iterator[Mask | None]: ...

    def segment(
        self, frame: Frame, prompts: Sequence[Prompt], embedding: Embedding | None = None
    ) -> list[Mask | None]: ...

    def features(self, frame: Frame) -> Embedding: ...

    def find_similar(self, frame: Frame, box: Box) -> list[Box]: ...

    def embed(self, frame: Frame) -> bytes: ...

    def load_embedding(self, data: bytes) -> Embedding: ...


class TaskCancelledError(Exception):
    """The API stopped waiting on the task, so its results would be thrown away."""


def segmenter(tracker: Tracker, frame: Frame, embedding: Embedding | None) -> Segment:
    """SAM's mask of a box on frame. The frame is encoded on the first box
    asked for, unless its embedding is given, and never again."""
    features = embedding

    def segment(box: Box) -> Mask | None:
        nonlocal features
        if features is None:
            features = tracker.features(frame)
        return tracker.segment(frame, [BoxPrompt(box)], features)[0]

    return segment


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

    reference = None
    if task.oriented and task.reference_url:
        reference = load_reference(fetch(task.reference_url))
    schema = Schema.of(task.keypoint_count, task.quads)
    keypoints: KeypointTracker | None = None
    # The tracker takes the frames, so the ones it was given wait here for
    # fitting rotated boxes and keypoints to them.
    given: deque[tuple[Frame, Embedding | None]] = deque()

    def feed() -> Iterator[tuple[Frame, Embedding | None]]:
        for frame, embedding in prefetch(load, task.frames):
            given.append((frame, embedding))
            yield frame, embedding

    start = last_report = time.monotonic()
    batch: list[worker_pb2.TrackedBox] = []
    found = 0
    cut_off = CutOff()
    for f, mask in zip(task.frames, tracker.track(feed(), prompt), strict=True):
        if cancelled():
            raise TaskCancelledError
        frame_id = f.id
        frame, embedding = given.popleft()
        segment = segmenter(tracker, frame, embedding)
        if schema.count and keypoints is None:
            keypoints = start_keypoints(tracker, task, schema, frame, mask, segment)
        points = None
        if keypoints is not None:
            # Stepped on every frame, so keypoints follow the object through
            # frames where its box is dropped.
            points = keypoints.step(frame, mask, segment)
            if f.keypoints:
                keypoints.anchor(from_proto(f.keypoints, schema.count, frame.shape))
                points = keypoints.points.copy()
        box = None if mask is None else mask_box(mask)
        if box is not None and not cut_off(box):
            corners = fit(frame, box, mask, reference) if task.oriented else None
            kps = [] if points is None else keypoints_proto(points, frame.shape)
            batch.append(worker_pb2.TrackedBox(frame_id=frame_id, box=to_proto(box, corners, kps)))
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
        "tracked task %d: object on %d of %d frames in %.1fs",
        task.id,
        found,
        len(task.frames),
        elapsed,
    )


def start_keypoints(
    tracker: Tracker,
    task: worker_pb2.TrackTask,
    schema: Schema,
    frame: Frame,
    mask: Mask | None,
    segment: Segment,
) -> KeypointTracker:
    """Keypoints on the prompt frame: the ones the labeler placed, plus the
    quad they pointed at.

    A printed quad is found inside the object's mask rather than SAM's mask
    of the point, since a click on a print lands on its design more often
    than on bare paper, and SAM then segments only the design.
    """
    design = None
    if task.reference_url and schema.reference is not None:
        design = load_design(fetch(task.reference_url))
    if task.HasField("box"):
        start = from_proto(task.box.keypoints, schema.count, frame.shape)
    else:
        start = np.zeros((schema.count, 3))
    earlier = [from_proto(k.keypoints, schema.count, frame.shape) for k in task.labeled_before]
    keypoints = KeypointTracker(schema, design, start, earlier)
    if task.HasField("fill") and 0 <= task.fill.quad < len(schema.quads):
        surface = mask
        if not (schema.quads[task.fill.quad].reference and design is not None):
            point = PointPrompt(task.fill.point.x, task.fill.point.y)
            surface = tracker.segment(frame, [point])[0]
        if surface is not None and surface.any():
            keypoints.fill(frame, task.fill.quad, surface, segment)
    return keypoints


def run_orient(
    api: Api,
    task: worker_pb2.OrientTask,
    tracker: Tracker | None = None,
    cancelled: Callable[[], bool] = never,
) -> None:
    if task.keypoint_count:
        if tracker is None:
            raise ValueError("fitting keypoints needs SAM")
        run_fit_keypoints(api, task, tracker, cancelled)
        return
    reference = load_reference(fetch(task.reference_url)) if task.reference_url else None
    if reference is None and tracker is None:
        raise ValueError("fitting rotated boxes without a reference design needs SAM")
    start = last_report = time.monotonic()
    batch: list[worker_pb2.OrientBox] = []
    found = total = 0
    frames = prefetch(lambda f: decode_frame(fetch(f.frame.url)), task.frames)
    for f, frame in zip(task.frames, frames, strict=True):
        if cancelled():
            raise TaskCancelledError
        boxes = [(b.box.x, b.box.y, b.box.width, b.box.height) for b in f.boxes]
        masks: list[Mask | None] = (
            tracker.segment(frame, [BoxPrompt(b) for b in boxes])
            if tracker is not None and reference is None
            else [None] * len(boxes)
        )
        for b, box, mask in zip(f.boxes, boxes, masks, strict=True):
            total += 1
            if reference is None and mask is None:
                continue
            corners = fit(frame, box, mask, reference)
            if corners is not None:
                batch.append(
                    worker_pb2.OrientBox(annotation_id=b.annotation_id, box=to_proto(box, corners))
                )
                found += 1
        if time.monotonic() - last_report >= REPORT_EVERY:
            api.report_orient(worker_pb2.ReportOrientRequest(task_id=task.id, boxes=batch))
            batch = []
            last_report = time.monotonic()
    api.report_orient(worker_pb2.ReportOrientRequest(task_id=task.id, boxes=batch, done=True))
    log.info(
        "oriented task %d: fit %d of %d boxes in %.1fs",
        task.id,
        found,
        total,
        time.monotonic() - start,
    )


def run_fit_keypoints(
    api: Api,
    task: worker_pb2.OrientTask,
    tracker: Tracker,
    cancelled: Callable[[], bool] = never,
) -> None:
    """Find the reference design's quad in each box.

    The print is looked for inside SAM's mask of the object, not the whole
    box, since bright floor in a box's corners can join the paper and turn
    the design the wrong way.
    """
    schema = Schema.of(task.keypoint_count, task.quads)
    q = schema.reference
    if q is None or not task.reference_url:
        raise ValueError("fitting keypoints needs a reference quad and its design")
    design = load_design(fetch(task.reference_url))
    start = last_report = time.monotonic()
    batch: list[worker_pb2.OrientBox] = []
    found = total = 0
    frames = prefetch(lambda f: decode_frame(fetch(f.frame.url)), task.frames)
    for f, frame in zip(task.frames, frames, strict=True):
        if cancelled():
            raise TaskCancelledError
        gray = np.asarray(cv2.cvtColor(frame, cv2.COLOR_RGB2GRAY), np.uint8)
        boxes = [(b.box.x, b.box.y, b.box.width, b.box.height) for b in f.boxes]
        embedding = tracker.features(frame)
        masks = tracker.segment(frame, [BoxPrompt(b) for b in boxes], embedding)
        segment = segmenter(tracker, frame, embedding)
        for b, box, mask in zip(f.boxes, boxes, masks, strict=True):
            total += 1
            if mask is None or not mask.any():
                continue
            fit_print = find_print(frame, gray, mask, design, None, segment)
            if fit_print is None:
                continue
            corners, paper = fit_print
            points = np.zeros((schema.count, 3))
            for i, c, v in zip(q.corners, corners, print_visibility(corners, paper), strict=True):
                points[i] = (c[0], c[1], v)
            place_centers(points, schema)
            batch.append(
                worker_pb2.OrientBox(
                    annotation_id=b.annotation_id,
                    box=to_proto(box, keypoints=keypoints_proto(points, frame.shape)),
                )
            )
            found += 1
        if time.monotonic() - last_report >= REPORT_EVERY:
            api.report_orient(worker_pb2.ReportOrientRequest(task_id=task.id, boxes=batch))
            batch = []
            last_report = time.monotonic()
    api.report_orient(worker_pb2.ReportOrientRequest(task_id=task.id, boxes=batch, done=True))
    log.info(
        "fit keypoints task %d: %d of %d boxes in %.1fs",
        task.id,
        found,
        total,
        time.monotonic() - start,
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
    | worker_pb2.OrientTask
    | pipeline_pb2.TrainTask
    | pipeline_pb2.PrelabelTask
    | pipeline_pb2.PreviewTask
    | pipeline_pb2.InspectTask
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
        elif kind == "orient":
            run_orient(api, res.orient, tracker, cancelled)
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
            elif kind == "inspect":
                run_inspect(api, res.inspect)
            elif kind == "orient" and res.orient.reference_url and not res.orient.keypoint_count:
                run_orient(api, res.orient, cancelled=heartbeat.cancelled)
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
