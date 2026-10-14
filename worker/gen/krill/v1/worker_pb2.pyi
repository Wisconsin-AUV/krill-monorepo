import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from krill.v1 import annotation_pb2 as _annotation_pb2
from krill.v1 import log_pb2 as _log_pb2
from krill.v1 import system_pb2 as _system_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ModelLoadState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MODEL_LOAD_STATE_UNSPECIFIED: _ClassVar[ModelLoadState]
    MODEL_LOAD_STATE_UNLOADED: _ClassVar[ModelLoadState]
    MODEL_LOAD_STATE_LOADING: _ClassVar[ModelLoadState]
    MODEL_LOAD_STATE_READY: _ClassVar[ModelLoadState]
    MODEL_LOAD_STATE_ERROR: _ClassVar[ModelLoadState]

class WorkerAction(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    WORKER_ACTION_UNSPECIFIED: _ClassVar[WorkerAction]
    WORKER_ACTION_WARM_UP: _ClassVar[WorkerAction]
    WORKER_ACTION_UNLOAD: _ClassVar[WorkerAction]
MODEL_LOAD_STATE_UNSPECIFIED: ModelLoadState
MODEL_LOAD_STATE_UNLOADED: ModelLoadState
MODEL_LOAD_STATE_LOADING: ModelLoadState
MODEL_LOAD_STATE_READY: ModelLoadState
MODEL_LOAD_STATE_ERROR: ModelLoadState
WORKER_ACTION_UNSPECIFIED: WorkerAction
WORKER_ACTION_WARM_UP: WorkerAction
WORKER_ACTION_UNLOAD: WorkerAction

class NextTaskRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class NextTaskResponse(_message.Message):
    __slots__ = ("track", "exemplar", "embed")
    TRACK_FIELD_NUMBER: _ClassVar[int]
    EXEMPLAR_FIELD_NUMBER: _ClassVar[int]
    EMBED_FIELD_NUMBER: _ClassVar[int]
    track: TrackTask
    exemplar: ExemplarTask
    embed: EmbedTask
    def __init__(self, track: _Optional[_Union[TrackTask, _Mapping]] = ..., exemplar: _Optional[_Union[ExemplarTask, _Mapping]] = ..., embed: _Optional[_Union[EmbedTask, _Mapping]] = ...) -> None: ...

class TaskFrame(_message.Message):
    __slots__ = ("id", "url", "embedding_url")
    ID_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    EMBEDDING_URL_FIELD_NUMBER: _ClassVar[int]
    id: int
    url: str
    embedding_url: str
    def __init__(self, id: _Optional[int] = ..., url: _Optional[str] = ..., embedding_url: _Optional[str] = ...) -> None: ...

class TrackTask(_message.Message):
    __slots__ = ("id", "frames", "point", "box", "embedding_model")
    ID_FIELD_NUMBER: _ClassVar[int]
    FRAMES_FIELD_NUMBER: _ClassVar[int]
    POINT_FIELD_NUMBER: _ClassVar[int]
    BOX_FIELD_NUMBER: _ClassVar[int]
    EMBEDDING_MODEL_FIELD_NUMBER: _ClassVar[int]
    id: int
    frames: _containers.RepeatedCompositeFieldContainer[TaskFrame]
    point: _annotation_pb2.Point
    box: _annotation_pb2.Box
    embedding_model: str
    def __init__(self, id: _Optional[int] = ..., frames: _Optional[_Iterable[_Union[TaskFrame, _Mapping]]] = ..., point: _Optional[_Union[_annotation_pb2.Point, _Mapping]] = ..., box: _Optional[_Union[_annotation_pb2.Box, _Mapping]] = ..., embedding_model: _Optional[str] = ...) -> None: ...

class EmbedTask(_message.Message):
    __slots__ = ("id", "frames")
    ID_FIELD_NUMBER: _ClassVar[int]
    FRAMES_FIELD_NUMBER: _ClassVar[int]
    id: int
    frames: _containers.RepeatedCompositeFieldContainer[EmbedFrame]
    def __init__(self, id: _Optional[int] = ..., frames: _Optional[_Iterable[_Union[EmbedFrame, _Mapping]]] = ...) -> None: ...

class EmbedFrame(_message.Message):
    __slots__ = ("id", "url", "upload_url")
    ID_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    UPLOAD_URL_FIELD_NUMBER: _ClassVar[int]
    id: int
    url: str
    upload_url: str
    def __init__(self, id: _Optional[int] = ..., url: _Optional[str] = ..., upload_url: _Optional[str] = ...) -> None: ...

class ExemplarTask(_message.Message):
    __slots__ = ("id", "frame", "box")
    ID_FIELD_NUMBER: _ClassVar[int]
    FRAME_FIELD_NUMBER: _ClassVar[int]
    BOX_FIELD_NUMBER: _ClassVar[int]
    id: int
    frame: TaskFrame
    box: _annotation_pb2.Box
    def __init__(self, id: _Optional[int] = ..., frame: _Optional[_Union[TaskFrame, _Mapping]] = ..., box: _Optional[_Union[_annotation_pb2.Box, _Mapping]] = ...) -> None: ...

class TrackedBox(_message.Message):
    __slots__ = ("frame_id", "box")
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    BOX_FIELD_NUMBER: _ClassVar[int]
    frame_id: int
    box: _annotation_pb2.Box
    def __init__(self, frame_id: _Optional[int] = ..., box: _Optional[_Union[_annotation_pb2.Box, _Mapping]] = ...) -> None: ...

class ReportTrackRequest(_message.Message):
    __slots__ = ("task_id", "model", "boxes", "done")
    TASK_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    BOXES_FIELD_NUMBER: _ClassVar[int]
    DONE_FIELD_NUMBER: _ClassVar[int]
    task_id: int
    model: str
    boxes: _containers.RepeatedCompositeFieldContainer[TrackedBox]
    done: bool
    def __init__(self, task_id: _Optional[int] = ..., model: _Optional[str] = ..., boxes: _Optional[_Iterable[_Union[TrackedBox, _Mapping]]] = ..., done: _Optional[bool] = ...) -> None: ...

class ReportTrackResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ReportExemplarsRequest(_message.Message):
    __slots__ = ("task_id", "model", "boxes")
    TASK_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    BOXES_FIELD_NUMBER: _ClassVar[int]
    task_id: int
    model: str
    boxes: _containers.RepeatedCompositeFieldContainer[_annotation_pb2.Box]
    def __init__(self, task_id: _Optional[int] = ..., model: _Optional[str] = ..., boxes: _Optional[_Iterable[_Union[_annotation_pb2.Box, _Mapping]]] = ...) -> None: ...

class ReportExemplarsResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ReportEmbedRequest(_message.Message):
    __slots__ = ("task_id", "model")
    TASK_ID_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    task_id: int
    model: str
    def __init__(self, task_id: _Optional[int] = ..., model: _Optional[str] = ...) -> None: ...

class ReportEmbedResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class FailTaskRequest(_message.Message):
    __slots__ = ("task_id", "error")
    TASK_ID_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    task_id: int
    error: str
    def __init__(self, task_id: _Optional[int] = ..., error: _Optional[str] = ...) -> None: ...

class FailTaskResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class HeartbeatRequest(_message.Message):
    __slots__ = ("instance_id", "hostname", "version", "model", "accelerator", "started_at", "host", "gpus", "task_id", "logs", "model_state", "model_error", "model_memory_allocated_bytes", "model_memory_reserved_bytes", "pinned_model", "cached_models", "command_id", "supports_exemplars")
    INSTANCE_ID_FIELD_NUMBER: _ClassVar[int]
    HOSTNAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    ACCELERATOR_FIELD_NUMBER: _ClassVar[int]
    STARTED_AT_FIELD_NUMBER: _ClassVar[int]
    HOST_FIELD_NUMBER: _ClassVar[int]
    GPUS_FIELD_NUMBER: _ClassVar[int]
    TASK_ID_FIELD_NUMBER: _ClassVar[int]
    LOGS_FIELD_NUMBER: _ClassVar[int]
    MODEL_STATE_FIELD_NUMBER: _ClassVar[int]
    MODEL_ERROR_FIELD_NUMBER: _ClassVar[int]
    MODEL_MEMORY_ALLOCATED_BYTES_FIELD_NUMBER: _ClassVar[int]
    MODEL_MEMORY_RESERVED_BYTES_FIELD_NUMBER: _ClassVar[int]
    PINNED_MODEL_FIELD_NUMBER: _ClassVar[int]
    CACHED_MODELS_FIELD_NUMBER: _ClassVar[int]
    COMMAND_ID_FIELD_NUMBER: _ClassVar[int]
    SUPPORTS_EXEMPLARS_FIELD_NUMBER: _ClassVar[int]
    instance_id: str
    hostname: str
    version: str
    model: str
    accelerator: str
    started_at: _timestamp_pb2.Timestamp
    host: _system_pb2.HostStats
    gpus: _containers.RepeatedCompositeFieldContainer[_system_pb2.GpuStats]
    task_id: int
    logs: _containers.RepeatedCompositeFieldContainer[_log_pb2.LogRecord]
    model_state: ModelLoadState
    model_error: str
    model_memory_allocated_bytes: int
    model_memory_reserved_bytes: int
    pinned_model: str
    cached_models: _containers.RepeatedScalarFieldContainer[str]
    command_id: int
    supports_exemplars: bool
    def __init__(self, instance_id: _Optional[str] = ..., hostname: _Optional[str] = ..., version: _Optional[str] = ..., model: _Optional[str] = ..., accelerator: _Optional[str] = ..., started_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., host: _Optional[_Union[_system_pb2.HostStats, _Mapping]] = ..., gpus: _Optional[_Iterable[_Union[_system_pb2.GpuStats, _Mapping]]] = ..., task_id: _Optional[int] = ..., logs: _Optional[_Iterable[_Union[_log_pb2.LogRecord, _Mapping]]] = ..., model_state: _Optional[_Union[ModelLoadState, str]] = ..., model_error: _Optional[str] = ..., model_memory_allocated_bytes: _Optional[int] = ..., model_memory_reserved_bytes: _Optional[int] = ..., pinned_model: _Optional[str] = ..., cached_models: _Optional[_Iterable[str]] = ..., command_id: _Optional[int] = ..., supports_exemplars: _Optional[bool] = ...) -> None: ...

class HeartbeatResponse(_message.Message):
    __slots__ = ("model", "command", "cancel_task_id")
    MODEL_FIELD_NUMBER: _ClassVar[int]
    COMMAND_FIELD_NUMBER: _ClassVar[int]
    CANCEL_TASK_ID_FIELD_NUMBER: _ClassVar[int]
    model: str
    command: WorkerCommand
    cancel_task_id: int
    def __init__(self, model: _Optional[str] = ..., command: _Optional[_Union[WorkerCommand, _Mapping]] = ..., cancel_task_id: _Optional[int] = ...) -> None: ...

class WorkerCommand(_message.Message):
    __slots__ = ("id", "action")
    ID_FIELD_NUMBER: _ClassVar[int]
    ACTION_FIELD_NUMBER: _ClassVar[int]
    id: int
    action: WorkerAction
    def __init__(self, id: _Optional[int] = ..., action: _Optional[_Union[WorkerAction, str]] = ...) -> None: ...
