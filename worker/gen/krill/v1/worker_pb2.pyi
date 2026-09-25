from krill.v1 import annotation_pb2 as _annotation_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class NextTaskRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class NextTaskResponse(_message.Message):
    __slots__ = ("track",)
    TRACK_FIELD_NUMBER: _ClassVar[int]
    track: TrackTask
    def __init__(self, track: _Optional[_Union[TrackTask, _Mapping]] = ...) -> None: ...

class TaskFrame(_message.Message):
    __slots__ = ("id", "url")
    ID_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    id: int
    url: str
    def __init__(self, id: _Optional[int] = ..., url: _Optional[str] = ...) -> None: ...

class TrackTask(_message.Message):
    __slots__ = ("id", "frames", "point", "box")
    ID_FIELD_NUMBER: _ClassVar[int]
    FRAMES_FIELD_NUMBER: _ClassVar[int]
    POINT_FIELD_NUMBER: _ClassVar[int]
    BOX_FIELD_NUMBER: _ClassVar[int]
    id: int
    frames: _containers.RepeatedCompositeFieldContainer[TaskFrame]
    point: _annotation_pb2.Point
    box: _annotation_pb2.Box
    def __init__(self, id: _Optional[int] = ..., frames: _Optional[_Iterable[_Union[TaskFrame, _Mapping]]] = ..., point: _Optional[_Union[_annotation_pb2.Point, _Mapping]] = ..., box: _Optional[_Union[_annotation_pb2.Box, _Mapping]] = ...) -> None: ...

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
