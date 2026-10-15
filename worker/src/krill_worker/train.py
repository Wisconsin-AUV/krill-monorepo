"""Training YOLO on a dataset the API exported, giving the GPU to waiting
tasks between batches."""

import json
import logging
import os
import re
import tempfile
import time
import zipfile
from collections import deque
from collections.abc import Callable, Sequence
from pathlib import Path
from typing import Any

from krill.v1 import pipeline_pb2, pipeline_worker_pb2, worker_pb2

from krill_worker.api import Api, ApiError
from krill_worker.frames import upload
from krill_worker.pipeline import cache_dir, download

log = logging.getLogger(__name__)

# How often a training worker asks whether anything else wants the GPU.
CHECK_IN_EVERY = 1.0
LOG_LINES = 400
ANSI = re.compile(r"\x1b\[[0-9;]*[A-Za-z]")
SIZES = {pipeline_pb2.MODEL_SIZE_NANO: "n", pipeline_pb2.MODEL_SIZE_SMALL: "s"}


class RunCancelledError(Exception):
    """The API no longer knows the run: it was cancelled or the API restarted."""


class LogTail(logging.Handler):
    """Keeps the last lines logged, for the run's page in the admin panel."""

    def __init__(self) -> None:
        super().__init__()
        self.lines: deque[str] = deque(maxlen=LOG_LINES)
        self.setFormatter(logging.Formatter("%(message)s"))

    def emit(self, record: logging.LogRecord) -> None:
        for line in ANSI.sub("", self.format(record)).splitlines():
            if line.strip():
                self.lines.append(line.rstrip())

    def text(self) -> str:
        return "\n".join(self.lines)


def free_gpu_cache() -> None:
    """Return training's cached blocks to the driver so a task served between
    batches has room."""
    try:
        import torch
    except ImportError:
        return
    if torch.cuda.is_available():
        torch.cuda.empty_cache()


class CheckIn:
    """Called between batches. Runs every task waiting for the GPU before
    training goes on, and raises RunCancelledError once the run is gone."""

    def __init__(
        self,
        api: Api,
        task_id: int,
        serve: Callable[[worker_pb2.NextTaskResponse], None],
        every: float = CHECK_IN_EVERY,
        clock: Callable[[], float] = time.monotonic,
    ) -> None:
        self.api = api
        self.task_id = task_id
        self.serve = serve
        self.every = every
        self.clock = clock
        self.last = -every

    def __call__(self, _: object = None) -> None:
        if self.clock() - self.last < self.every:
            return
        while True:
            try:
                res = self.api.check_in(self.task_id)
            except ApiError as e:
                if e.code == "not_found":
                    raise RunCancelledError from e
                log.warning("check-in failed: %s", e)
                break
            except OSError as e:
                log.warning("check-in failed: %s", e)
                break
            if res.task.WhichOneof("task") is None:
                break
            free_gpu_cache()
            self.serve(res.task)
        self.last = self.clock()


def data_yaml(root: Path, classes: Sequence[str], has_test: bool) -> Path:
    """Point Ultralytics at the extracted dataset. JSON is valid YAML."""
    data: dict[str, Any] = {
        "path": str(root.resolve()),
        "train": "images/train",
        "val": "images/val",
        "names": dict(enumerate(classes)),
    }
    if has_test:
        data["test"] = "images/test"
    path = root / "krill.yaml"
    path.write_text(json.dumps(data))
    return path


def eval_metrics(m: Any) -> pipeline_pb2.EvalMetrics:
    """Convert Ultralytics' DetMetrics, with one row per class in the split."""
    box = m.box
    per_class = getattr(m, "nt_per_class", None)
    per_image = getattr(m, "nt_per_image", None)
    out = pipeline_pb2.EvalMetrics(
        all=pipeline_pb2.ClassMetrics(
            name="all",
            instances=int(per_class.sum()) if per_class is not None else 0,
            precision=float(box.mp),
            recall=float(box.mr),
            map50=float(box.map50),
            map50_95=float(box.map),
        )
    )
    for i, c in enumerate(box.ap_class_index):
        p, r, ap50, ap = box.class_result(i)
        out.classes.append(
            pipeline_pb2.ClassMetrics(
                name=str(m.names[int(c)]),
                images=int(per_image[c]) if per_image is not None else 0,
                instances=int(per_class[c]) if per_class is not None else 0,
                precision=float(p),
                recall=float(r),
                map50=float(ap50),
                map50_95=float(ap),
            )
        )
    return out


def run_train(
    api: Api,
    task: pipeline_pb2.TrainTask,
    serve: Callable[[worker_pb2.NextTaskResponse], None],
) -> None:
    # Imported here so the worker runs without torch installed.
    import torch
    from ultralytics import YOLO
    from ultralytics.utils import LOGGER

    tail = LogTail()
    LOGGER.addHandler(tail)
    log.addHandler(tail)
    cuda = torch.cuda.is_available()
    device: int | str = 0 if cuda else "cpu"
    device_name = torch.cuda.get_device_name(0) if cuda else "CPU"
    check_in = CheckIn(api, task.id, serve)

    def progress(epoch: pipeline_pb2.EpochMetrics | None = None, evaluating: bool = False) -> None:
        req = pipeline_worker_pb2.ReportTrainProgressRequest(
            task_id=task.id,
            epochs=task.epochs,
            epoch=epoch,
            evaluating=evaluating,
            log=tail.text(),
            device=device_name,
        )
        try:
            api.report_train_progress(req)
        except ApiError as e:
            if e.code == "not_found":
                raise RunCancelledError from e
            log.warning("progress report failed: %s", e)
        except OSError as e:
            log.warning("progress report failed: %s", e)

    reported = 0

    def on_epoch(trainer: Any) -> None:
        nonlocal reported
        n = trainer.epoch + 1
        # Ultralytics calls this again after its final validation.
        if n > task.epochs or n <= reported:
            return
        reported = n
        m = trainer.metrics
        loss = trainer.label_loss_items(trainer.tloss, prefix="train")
        progress(
            pipeline_pb2.EpochMetrics(
                epoch=n,
                box_loss=float(loss.get("train/box_loss", 0)),
                cls_loss=float(loss.get("train/cls_loss", 0)),
                precision=float(m.get("metrics/precision(B)", 0)),
                recall=float(m.get("metrics/recall(B)", 0)),
                map50=float(m.get("metrics/mAP50(B)", 0)),
                map50_95=float(m.get("metrics/mAP50-95(B)", 0)),
            )
        )

    def evaluate(weights: Path, data: Path, split: str) -> pipeline_pb2.EvalMetrics:
        m = YOLO(str(weights)).val(
            data=str(data),
            split=split,
            imgsz=task.image_size,
            batch=task.batch,
            device=device,
            plots=False,
            verbose=False,
        )
        return eval_metrics(m)

    start = time.monotonic()
    try:
        with tempfile.TemporaryDirectory(prefix="krill-train-") as tmp:
            root = Path(tmp)
            log.info("training run %d on %s", task.run_id, device_name)
            with zipfile.ZipFile(download(task.dataset_url, root / "dataset.zip")) as z:
                z.extractall(root / "data")
            data = data_yaml(root / "data", task.classes, task.has_test)
            if task.base_weights_url:
                base = download(task.base_weights_url, root / "base.pt")
            else:
                # Ultralytics downloads its COCO weights to this path.
                base = cache_dir() / f"yolo11{SIZES[task.size]}.pt"
            progress()
            check_in()

            model = YOLO(str(base))
            model.add_callback("on_train_batch_end", check_in)
            model.add_callback("on_val_batch_end", check_in)
            model.add_callback("on_fit_epoch_end", on_epoch)
            model.train(
                data=str(data),
                epochs=task.epochs,
                imgsz=task.image_size,
                batch=task.batch,
                device=device,
                project=str(root / "runs"),
                name="train",
                exist_ok=True,
                plots=False,
                workers=min(4, os.cpu_count() or 1),
            )
            best = Path(model.trainer.best)

            progress(evaluating=True)
            req = pipeline_worker_pb2.FinishTrainRequest(task_id=task.id)
            req.val.CopyFrom(evaluate(best, data, "val"))
            if task.has_test:
                req.test.CopyFrom(evaluate(best, data, "test"))
            if task.baseline_weights_url:
                baseline = download(task.baseline_weights_url, root / "baseline.pt")
                req.baseline_val.CopyFrom(evaluate(baseline, data, "val"))
                if task.has_test:
                    req.baseline_test.CopyFrom(evaluate(baseline, data, "test"))
            weights = best.read_bytes()
            upload(task.weights_upload_url, weights)
            req.weights_bytes = len(weights)
            log.info(
                "run %d finished in %.0f minutes", task.run_id, (time.monotonic() - start) / 60
            )
            req.log = tail.text()
            api.finish_train(req)
    except RunCancelledError:
        log.info("run %d was cancelled", task.run_id)
    finally:
        LOGGER.removeHandler(tail)
        log.removeHandler(tail)
        free_gpu_cache()
