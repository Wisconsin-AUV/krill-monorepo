import urllib.request
from collections import deque
from collections.abc import Callable, Iterable, Iterator
from concurrent.futures import Future, ThreadPoolExecutor
from typing import TypeVar

import cv2
import numpy as np

from krill_worker.prelabelers import Frame

# SAM resizes to about 1024 pixels, so larger frames only cost decode time.
MAX_SIDE = 1024

# Downloads run this many frames ahead of the GPU. Embeddings are about 8 MB
# each, so reading a whole clip ahead would hold gigabytes.
PREFETCH = 16

T = TypeVar("T")
R = TypeVar("R")


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


def upload(url: str, data: bytes, content_type: str = "application/octet-stream") -> None:
    req = urllib.request.Request(
        url, data=data, method="PUT", headers={"Content-Type": content_type}
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
