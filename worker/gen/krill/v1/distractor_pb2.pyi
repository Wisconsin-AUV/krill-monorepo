import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Distractor(_message.Message):
    __slots__ = ("id", "url", "filename", "width", "height", "size_bytes", "created_by", "created_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    FILENAME_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    SIZE_BYTES_FIELD_NUMBER: _ClassVar[int]
    CREATED_BY_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    id: int
    url: str
    filename: str
    width: int
    height: int
    size_bytes: int
    created_by: str
    created_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[int] = ..., url: _Optional[str] = ..., filename: _Optional[str] = ..., width: _Optional[int] = ..., height: _Optional[int] = ..., size_bytes: _Optional[int] = ..., created_by: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListDistractorsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListDistractorsResponse(_message.Message):
    __slots__ = ("distractors",)
    DISTRACTORS_FIELD_NUMBER: _ClassVar[int]
    distractors: _containers.RepeatedCompositeFieldContainer[Distractor]
    def __init__(self, distractors: _Optional[_Iterable[_Union[Distractor, _Mapping]]] = ...) -> None: ...

class CreateDistractorUploadRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CreateDistractorUploadResponse(_message.Message):
    __slots__ = ("key", "upload_url")
    KEY_FIELD_NUMBER: _ClassVar[int]
    UPLOAD_URL_FIELD_NUMBER: _ClassVar[int]
    key: str
    upload_url: str
    def __init__(self, key: _Optional[str] = ..., upload_url: _Optional[str] = ...) -> None: ...

class AddDistractorRequest(_message.Message):
    __slots__ = ("key", "filename")
    KEY_FIELD_NUMBER: _ClassVar[int]
    FILENAME_FIELD_NUMBER: _ClassVar[int]
    key: str
    filename: str
    def __init__(self, key: _Optional[str] = ..., filename: _Optional[str] = ...) -> None: ...

class AddDistractorResponse(_message.Message):
    __slots__ = ("distractor",)
    DISTRACTOR_FIELD_NUMBER: _ClassVar[int]
    distractor: Distractor
    def __init__(self, distractor: _Optional[_Union[Distractor, _Mapping]] = ...) -> None: ...

class DeleteDistractorRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class DeleteDistractorResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
