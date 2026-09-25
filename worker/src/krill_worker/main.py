import logging
import os
import time
import urllib.request
from collections.abc import Iterator
from concurrent.futures import ThreadPoolExecutor
from typing import TYPE_CHECKING

import cv2
import numpy as np
from krill.v1 import annotation_pb2, worker_pb2

from krill_worker import __version__
from krill_worker.api import Api, ApiError
from krill_worker.prelabelers import Frame
from krill_worker.track import BoxPrompt, PointPrompt, Prompt

if TYPE_CHECKING:
    from krill_worker.sam import SamTracker

log = logging.getLogger(__name__)

DEFAULT_MODEL = "facebook/sam2.1-hiera-small"
# SAM resizes to about 1024 pixels, so larger frames only cost decode time.
MAX_SIDE = 1024
REPORT_EVERY = 2.0
RETRY_DELAY = 5.0


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


def download_frames(urls: list[str]) -> Iterator[Frame]:
    with ThreadPoolExecutor(8) as pool:
        for data in pool.map(fetch, urls):
            yield decode_frame(data)


def run_track(api: Api, tracker: "SamTracker", task: worker_pb2.TrackTask) -> None:
    prompt: Prompt
    if task.HasField("point"):
        prompt = PointPrompt(task.point.x, task.point.y)
    else:
        b = task.box
        prompt = BoxPrompt((b.x, b.y, b.width, b.height))

    frames = download_frames([f.url for f in task.frames])
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
        if not res.HasField("track"):
            continue
        task = res.track
        try:
            run_track(api, tracker, task)
        except ApiError as e:
            log.warning("task %d dropped by the API: %s", task.id, e)
        except Exception as e:
            log.exception("task %d failed", task.id)
            try:
                api.fail_task(task.id, str(e))
            except (ApiError, OSError):
                log.exception("report failure for task %d", task.id)


if __name__ == "__main__":
    main()
