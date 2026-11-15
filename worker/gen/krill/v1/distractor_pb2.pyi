import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class DistractorStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    DISTRACTOR_STATUS_UNSPECIFIED: _ClassVar[DistractorStatus]
    DISTRACTOR_STATUS_UPLOADING: _ClassVar[DistractorStatus]
    DISTRACTOR_STATUS_QUEUED: _ClassVar[DistractorStatus]
    DISTRACTOR_STATUS_PROCESSING: _ClassVar[DistractorStatus]
    DISTRACTOR_STATUS_READY: _ClassVar[DistractorStatus]
    DISTRACTOR_STATUS_FAILED: _ClassVar[DistractorStatus]
DISTRACTOR_STATUS_UNSPECIFIED: DistractorStatus
DISTRACTOR_STATUS_UPLOADING: DistractorStatus
DISTRACTOR_STATUS_QUEUED: DistractorStatus
DISTRACTOR_STATUS_PROCESSING: DistractorStatus
DISTRACTOR_STATUS_READY: DistractorStatus
DISTRACTOR_STATUS_FAILED: DistractorStatus

class Distractor(_message.Message):
    __slots__ = ("id", "url", "thumbnail_url", "filename", "width", "height", "size_bytes", "created_by", "created_at", "status", "error")
    ID_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    THUMBNAIL_URL_FIELD_NUMBER: _ClassVar[int]
    FILENAME_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    SIZE_BYTES_FIELD_NUMBER: _ClassVar[int]
    CREATED_BY_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    id: int
    url: str
    thumbnail_url: str
    filename: str
    width: int
    height: int
    size_bytes: int
    created_by: str
    created_at: _timestamp_pb2.Timestamp
    status: DistractorStatus
    error: str
    def __init__(self, id: _Optional[int] = ..., url: _Optional[str] = ..., thumbnail_url: _Optional[str] = ..., filename: _Optional[str] = ..., width: _Optional[int] = ..., height: _Optional[int] = ..., size_bytes: _Optional[int] = ..., created_by: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., status: _Optional[_Union[DistractorStatus, str]] = ..., error: _Optional[str] = ...) -> None: ...

class ListDistractorsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListDistractorsResponse(_message.Message):
    __slots__ = ("distractors",)
    DISTRACTORS_FIELD_NUMBER: _ClassVar[int]
    distractors: _containers.RepeatedCompositeFieldContainer[Distractor]
    def __init__(self, distractors: _Optional[_Iterable[_Union[Distractor, _Mapping]]] = ...) -> None: ...

class CreateDistractorRequest(_message.Message):
    __slots__ = ("filename", "size")
    FILENAME_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    filename: str
    size: int
    def __init__(self, filename: _Optional[str] = ..., size: _Optional[int] = ...) -> None: ...

class CreateDistractorResponse(_message.Message):
    __slots__ = ("distractor", "upload_url")
    DISTRACTOR_FIELD_NUMBER: _ClassVar[int]
    UPLOAD_URL_FIELD_NUMBER: _ClassVar[int]
    distractor: Distractor
    upload_url: str
    def __init__(self, distractor: _Optional[_Union[Distractor, _Mapping]] = ..., upload_url: _Optional[str] = ...) -> None: ...

class ProcessDistractorRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class ProcessDistractorResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class DeleteDistractorsRequest(_message.Message):
    __slots__ = ("ids",)
    IDS_FIELD_NUMBER: _ClassVar[int]
    ids: _containers.RepeatedScalarFieldContainer[int]
    def __init__(self, ids: _Optional[_Iterable[int]] = ...) -> None: ...

class DeleteDistractorsResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
