import json
import urllib.error
import urllib.request
from typing import TypeVar

from google.protobuf.message import Message
from krill.v1 import pipeline_worker_pb2, worker_pb2

M = TypeVar("M", bound=Message)

# NextTask holds the request for up to 30 seconds.
POLL_TIMEOUT = 45
TIMEOUT = 30

WORKER = "krill.v1.WorkerService"
PIPELINE = "krill.v1.PipelineWorkerService"


class ApiError(Exception):
    def __init__(self, code: str, message: str) -> None:
        super().__init__(f"{code}: {message}")
        self.code = code


class Api:
    """Client for the API's worker services over the Connect protocol."""

    def __init__(self, url: str, token: str) -> None:
        self.url = url.rstrip("/")
        self.token = token

    def _call(
        self,
        method: str,
        req: Message,
        out: type[M],
        timeout: float = TIMEOUT,
        service: str = WORKER,
    ) -> M:
        request = urllib.request.Request(
            f"{self.url}/{service}/{method}",
            data=req.SerializeToString(),
            headers={
                "Content-Type": "application/proto",
                "Connect-Protocol-Version": "1",
                "Authorization": f"Bearer {self.token}",
            },
        )
        try:
            with urllib.request.urlopen(request, timeout=timeout) as res:
                return out.FromString(res.read())
        except urllib.error.HTTPError as e:
            try:
                body = json.loads(e.read())
            except ValueError:
                body = {}
            raise ApiError(body.get("code", str(e.code)), body.get("message", e.reason)) from e

    def next_task(self) -> worker_pb2.NextTaskResponse:
        return self._call(
            "NextTask", worker_pb2.NextTaskRequest(), worker_pb2.NextTaskResponse, POLL_TIMEOUT
        )

    def report_track(self, req: worker_pb2.ReportTrackRequest) -> None:
        self._call("ReportTrack", req, worker_pb2.ReportTrackResponse)

    def report_exemplars(self, req: worker_pb2.ReportExemplarsRequest) -> None:
        self._call("ReportExemplars", req, worker_pb2.ReportExemplarsResponse)

    def report_embed(self, req: worker_pb2.ReportEmbedRequest) -> None:
        self._call("ReportEmbed", req, worker_pb2.ReportEmbedResponse)

    def fail_task(self, task_id: int, error: str) -> None:
        req = worker_pb2.FailTaskRequest(task_id=task_id, error=error)
        self._call("FailTask", req, worker_pb2.FailTaskResponse)

    def heartbeat(self, req: worker_pb2.HeartbeatRequest) -> worker_pb2.HeartbeatResponse:
        return self._call("Heartbeat", req, worker_pb2.HeartbeatResponse)

    def register_prelabelers(self, req: pipeline_worker_pb2.RegisterPrelabelersRequest) -> None:
        self._call(
            "RegisterPrelabelers",
            req,
            pipeline_worker_pb2.RegisterPrelabelersResponse,
            service=PIPELINE,
        )

    def check_in(self, task_id: int) -> pipeline_worker_pb2.CheckInResponse:
        return self._call(
            "CheckIn",
            pipeline_worker_pb2.CheckInRequest(task_id=task_id),
            pipeline_worker_pb2.CheckInResponse,
            service=PIPELINE,
        )

    def report_train_progress(self, req: pipeline_worker_pb2.ReportTrainProgressRequest) -> None:
        self._call(
            "ReportTrainProgress",
            req,
            pipeline_worker_pb2.ReportTrainProgressResponse,
            service=PIPELINE,
        )

    def finish_train(self, req: pipeline_worker_pb2.FinishTrainRequest) -> None:
        self._call("FinishTrain", req, pipeline_worker_pb2.FinishTrainResponse, service=PIPELINE)

    def report_prelabels(self, req: pipeline_worker_pb2.ReportPrelabelsRequest) -> None:
        self._call(
            "ReportPrelabels", req, pipeline_worker_pb2.ReportPrelabelsResponse, service=PIPELINE
        )

    def report_preview(self, req: pipeline_worker_pb2.ReportPreviewRequest) -> None:
        self._call(
            "ReportPreview", req, pipeline_worker_pb2.ReportPreviewResponse, service=PIPELINE
        )
