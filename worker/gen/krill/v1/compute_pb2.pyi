import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from krill.v1 import stats_pb2 as _stats_pb2
from krill.v1 import worker_pb2 as _worker_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class JobState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    JOB_STATE_UNSPECIFIED: _ClassVar[JobState]
    JOB_STATE_QUEUED: _ClassVar[JobState]
    JOB_STATE_RUNNING: _ClassVar[JobState]
    JOB_STATE_RETRYABLE: _ClassVar[JobState]
    JOB_STATE_FAILED: _ClassVar[JobState]
    JOB_STATE_CANCELLED: _ClassVar[JobState]
    JOB_STATE_COMPLETED: _ClassVar[JobState]
JOB_STATE_UNSPECIFIED: JobState
JOB_STATE_QUEUED: JobState
JOB_STATE_RUNNING: JobState
JOB_STATE_RETRYABLE: JobState
JOB_STATE_FAILED: JobState
JOB_STATE_CANCELLED: JobState
JOB_STATE_COMPLETED: JobState

class Job(_message.Message):
    __slots__ = ("id", "kind", "queue", "state", "priority", "attempt", "max_attempts", "target", "args", "errors", "created_at", "scheduled_at", "attempted_at", "finalized_at", "started_by", "gpu", "cancelling")
    ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    QUEUE_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    PRIORITY_FIELD_NUMBER: _ClassVar[int]
    ATTEMPT_FIELD_NUMBER: _ClassVar[int]
    MAX_ATTEMPTS_FIELD_NUMBER: _ClassVar[int]
    TARGET_FIELD_NUMBER: _ClassVar[int]
    ARGS_FIELD_NUMBER: _ClassVar[int]
    ERRORS_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    SCHEDULED_AT_FIELD_NUMBER: _ClassVar[int]
    ATTEMPTED_AT_FIELD_NUMBER: _ClassVar[int]
    FINALIZED_AT_FIELD_NUMBER: _ClassVar[int]
    STARTED_BY_FIELD_NUMBER: _ClassVar[int]
    GPU_FIELD_NUMBER: _ClassVar[int]
    CANCELLING_FIELD_NUMBER: _ClassVar[int]
    id: int
    kind: str
    queue: str
    state: JobState
    priority: int
    attempt: int
    max_attempts: int
    target: str
    args: str
    errors: _containers.RepeatedCompositeFieldContainer[JobError]
    created_at: _timestamp_pb2.Timestamp
    scheduled_at: _timestamp_pb2.Timestamp
    attempted_at: _timestamp_pb2.Timestamp
    finalized_at: _timestamp_pb2.Timestamp
    started_by: _stats_pb2.Profile
    gpu: GpuTask
    cancelling: bool
    def __init__(self, id: _Optional[int] = ..., kind: _Optional[str] = ..., queue: _Optional[str] = ..., state: _Optional[_Union[JobState, str]] = ..., priority: _Optional[int] = ..., attempt: _Optional[int] = ..., max_attempts: _Optional[int] = ..., target: _Optional[str] = ..., args: _Optional[str] = ..., errors: _Optional[_Iterable[_Union[JobError, _Mapping]]] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., scheduled_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., attempted_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., finalized_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., started_by: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., gpu: _Optional[_Union[GpuTask, _Mapping]] = ..., cancelling: _Optional[bool] = ...) -> None: ...

class JobError(_message.Message):
    __slots__ = ("at", "attempt", "message")
    AT_FIELD_NUMBER: _ClassVar[int]
    ATTEMPT_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    at: _timestamp_pb2.Timestamp
    attempt: int
    message: str
    def __init__(self, at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., attempt: _Optional[int] = ..., message: _Optional[str] = ...) -> None: ...

class GpuTask(_message.Message):
    __slots__ = ("id", "picked_at", "worker_id", "worker_hostname")
    ID_FIELD_NUMBER: _ClassVar[int]
    PICKED_AT_FIELD_NUMBER: _ClassVar[int]
    WORKER_ID_FIELD_NUMBER: _ClassVar[int]
    WORKER_HOSTNAME_FIELD_NUMBER: _ClassVar[int]
    id: int
    picked_at: _timestamp_pb2.Timestamp
    worker_id: str
    worker_hostname: str
    def __init__(self, id: _Optional[int] = ..., picked_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., worker_id: _Optional[str] = ..., worker_hostname: _Optional[str] = ...) -> None: ...

class JobCount(_message.Message):
    __slots__ = ("kind", "state", "count")
    KIND_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    COUNT_FIELD_NUMBER: _ClassVar[int]
    kind: str
    state: JobState
    count: int
    def __init__(self, kind: _Optional[str] = ..., state: _Optional[_Union[JobState, str]] = ..., count: _Optional[int] = ...) -> None: ...

class ListJobsRequest(_message.Message):
    __slots__ = ("kinds", "states", "page_size", "page_token")
    KINDS_FIELD_NUMBER: _ClassVar[int]
    STATES_FIELD_NUMBER: _ClassVar[int]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    kinds: _containers.RepeatedScalarFieldContainer[str]
    states: _containers.RepeatedScalarFieldContainer[JobState]
    page_size: int
    page_token: str
    def __init__(self, kinds: _Optional[_Iterable[str]] = ..., states: _Optional[_Iterable[_Union[JobState, str]]] = ..., page_size: _Optional[int] = ..., page_token: _Optional[str] = ...) -> None: ...

class ListJobsResponse(_message.Message):
    __slots__ = ("jobs", "next_page_token", "counts")
    JOBS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    COUNTS_FIELD_NUMBER: _ClassVar[int]
    jobs: _containers.RepeatedCompositeFieldContainer[Job]
    next_page_token: str
    counts: _containers.RepeatedCompositeFieldContainer[JobCount]
    def __init__(self, jobs: _Optional[_Iterable[_Union[Job, _Mapping]]] = ..., next_page_token: _Optional[str] = ..., counts: _Optional[_Iterable[_Union[JobCount, _Mapping]]] = ...) -> None: ...

class RetryJobRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class RetryJobResponse(_message.Message):
    __slots__ = ("job",)
    JOB_FIELD_NUMBER: _ClassVar[int]
    job: Job
    def __init__(self, job: _Optional[_Union[Job, _Mapping]] = ...) -> None: ...

class CancelJobRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class CancelJobResponse(_message.Message):
    __slots__ = ("job",)
    JOB_FIELD_NUMBER: _ClassVar[int]
    job: Job
    def __init__(self, job: _Optional[_Union[Job, _Mapping]] = ...) -> None: ...

class SetJobPriorityRequest(_message.Message):
    __slots__ = ("id", "priority")
    ID_FIELD_NUMBER: _ClassVar[int]
    PRIORITY_FIELD_NUMBER: _ClassVar[int]
    id: int
    priority: int
    def __init__(self, id: _Optional[int] = ..., priority: _Optional[int] = ...) -> None: ...

class SetJobPriorityResponse(_message.Message):
    __slots__ = ("job",)
    JOB_FIELD_NUMBER: _ClassVar[int]
    job: Job
    def __init__(self, job: _Optional[_Union[Job, _Mapping]] = ...) -> None: ...

class WorkerModel(_message.Message):
    __slots__ = ("state", "error", "memory_allocated_bytes", "memory_reserved_bytes", "pinned", "supports_exemplars", "cached", "pending_action")
    STATE_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    MEMORY_ALLOCATED_BYTES_FIELD_NUMBER: _ClassVar[int]
    MEMORY_RESERVED_BYTES_FIELD_NUMBER: _ClassVar[int]
    PINNED_FIELD_NUMBER: _ClassVar[int]
    SUPPORTS_EXEMPLARS_FIELD_NUMBER: _ClassVar[int]
    CACHED_FIELD_NUMBER: _ClassVar[int]
    PENDING_ACTION_FIELD_NUMBER: _ClassVar[int]
    state: _worker_pb2.ModelLoadState
    error: str
    memory_allocated_bytes: int
    memory_reserved_bytes: int
    pinned: bool
    supports_exemplars: bool
    cached: _containers.RepeatedScalarFieldContainer[str]
    pending_action: _worker_pb2.WorkerAction
    def __init__(self, state: _Optional[_Union[_worker_pb2.ModelLoadState, str]] = ..., error: _Optional[str] = ..., memory_allocated_bytes: _Optional[int] = ..., memory_reserved_bytes: _Optional[int] = ..., pinned: _Optional[bool] = ..., supports_exemplars: _Optional[bool] = ..., cached: _Optional[_Iterable[str]] = ..., pending_action: _Optional[_Union[_worker_pb2.WorkerAction, str]] = ...) -> None: ...

class ControlWorkerRequest(_message.Message):
    __slots__ = ("worker_id", "action")
    WORKER_ID_FIELD_NUMBER: _ClassVar[int]
    ACTION_FIELD_NUMBER: _ClassVar[int]
    worker_id: str
    action: _worker_pb2.WorkerAction
    def __init__(self, worker_id: _Optional[str] = ..., action: _Optional[_Union[_worker_pb2.WorkerAction, str]] = ...) -> None: ...

class ControlWorkerResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class WorkerToken(_message.Message):
    __slots__ = ("id", "name", "created_by", "created_at", "last_used_at", "revoked_at", "workers")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    CREATED_BY_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    LAST_USED_AT_FIELD_NUMBER: _ClassVar[int]
    REVOKED_AT_FIELD_NUMBER: _ClassVar[int]
    WORKERS_FIELD_NUMBER: _ClassVar[int]
    id: int
    name: str
    created_by: _stats_pb2.Profile
    created_at: _timestamp_pb2.Timestamp
    last_used_at: _timestamp_pb2.Timestamp
    revoked_at: _timestamp_pb2.Timestamp
    workers: int
    def __init__(self, id: _Optional[int] = ..., name: _Optional[str] = ..., created_by: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., last_used_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., revoked_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., workers: _Optional[int] = ...) -> None: ...

class ListWorkerTokensRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListWorkerTokensResponse(_message.Message):
    __slots__ = ("tokens", "env_token", "env_token_workers")
    TOKENS_FIELD_NUMBER: _ClassVar[int]
    ENV_TOKEN_FIELD_NUMBER: _ClassVar[int]
    ENV_TOKEN_WORKERS_FIELD_NUMBER: _ClassVar[int]
    tokens: _containers.RepeatedCompositeFieldContainer[WorkerToken]
    env_token: bool
    env_token_workers: int
    def __init__(self, tokens: _Optional[_Iterable[_Union[WorkerToken, _Mapping]]] = ..., env_token: _Optional[bool] = ..., env_token_workers: _Optional[int] = ...) -> None: ...

class CreateWorkerTokenRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class CreateWorkerTokenResponse(_message.Message):
    __slots__ = ("token", "secret")
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    SECRET_FIELD_NUMBER: _ClassVar[int]
    token: WorkerToken
    secret: str
    def __init__(self, token: _Optional[_Union[WorkerToken, _Mapping]] = ..., secret: _Optional[str] = ...) -> None: ...

class RevokeWorkerTokenRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class RevokeWorkerTokenResponse(_message.Message):
    __slots__ = ("token",)
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    token: WorkerToken
    def __init__(self, token: _Optional[_Union[WorkerToken, _Mapping]] = ...) -> None: ...
