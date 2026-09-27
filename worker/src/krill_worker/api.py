import json
import urllib.error
import urllib.request
from typing import TypeVar

from google.protobuf.message import Message
from krill.v1 import worker_pb2

M = TypeVar("M", bound=Message)

# NextTask holds the request for up to 30 seconds.
POLL_TIMEOUT = 45
TIMEOUT = 30


class ApiError(Exception):
    def __init__(self, code: str, message: str) -> None:
        super().__init__(f"{code}: {message}")
        self.code = code


class Api:
    """Client for the API's WorkerService over the Connect protocol."""

    def __init__(self, url: str, token: str) -> None:
        self.url = url.rstrip("/")
        self.token = token

    def _call(self, method: str, req: Message, out: type[M], timeout: float = TIMEOUT) -> M:
        request = urllib.request.Request(
            f"{self.url}/krill.v1.WorkerService/{method}",
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

    def fail_task(self, task_id: int, error: str) -> None:
        req = worker_pb2.FailTaskRequest(task_id=task_id, error=error)
        self._call("FailTask", req, worker_pb2.FailTaskResponse)
