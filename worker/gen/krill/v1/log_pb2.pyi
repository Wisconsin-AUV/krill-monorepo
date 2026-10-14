import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class LogLevel(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    LOG_LEVEL_UNSPECIFIED: _ClassVar[LogLevel]
    LOG_LEVEL_DEBUG: _ClassVar[LogLevel]
    LOG_LEVEL_INFO: _ClassVar[LogLevel]
    LOG_LEVEL_WARN: _ClassVar[LogLevel]
    LOG_LEVEL_ERROR: _ClassVar[LogLevel]
LOG_LEVEL_UNSPECIFIED: LogLevel
LOG_LEVEL_DEBUG: LogLevel
LOG_LEVEL_INFO: LogLevel
LOG_LEVEL_WARN: LogLevel
LOG_LEVEL_ERROR: LogLevel

class LogAttr(_message.Message):
    __slots__ = ("key", "value")
    KEY_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    key: str
    value: str
    def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...

class LogRecord(_message.Message):
    __slots__ = ("seq", "time", "level", "message", "attrs")
    SEQ_FIELD_NUMBER: _ClassVar[int]
    TIME_FIELD_NUMBER: _ClassVar[int]
    LEVEL_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    ATTRS_FIELD_NUMBER: _ClassVar[int]
    seq: int
    time: _timestamp_pb2.Timestamp
    level: LogLevel
    message: str
    attrs: _containers.RepeatedCompositeFieldContainer[LogAttr]
    def __init__(self, seq: _Optional[int] = ..., time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., level: _Optional[_Union[LogLevel, str]] = ..., message: _Optional[str] = ..., attrs: _Optional[_Iterable[_Union[LogAttr, _Mapping]]] = ...) -> None: ...

class LogSource(_message.Message):
    __slots__ = ("id", "worker", "hostname", "online", "last_seen")
    ID_FIELD_NUMBER: _ClassVar[int]
    WORKER_FIELD_NUMBER: _ClassVar[int]
    HOSTNAME_FIELD_NUMBER: _ClassVar[int]
    ONLINE_FIELD_NUMBER: _ClassVar[int]
    LAST_SEEN_FIELD_NUMBER: _ClassVar[int]
    id: str
    worker: bool
    hostname: str
    online: bool
    last_seen: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., worker: _Optional[bool] = ..., hostname: _Optional[str] = ..., online: _Optional[bool] = ..., last_seen: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListLogSourcesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListLogSourcesResponse(_message.Message):
    __slots__ = ("sources",)
    SOURCES_FIELD_NUMBER: _ClassVar[int]
    sources: _containers.RepeatedCompositeFieldContainer[LogSource]
    def __init__(self, sources: _Optional[_Iterable[_Union[LogSource, _Mapping]]] = ...) -> None: ...

class ListLogsRequest(_message.Message):
    __slots__ = ("source_id", "min_level", "query", "after_seq", "limit")
    SOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    MIN_LEVEL_FIELD_NUMBER: _ClassVar[int]
    QUERY_FIELD_NUMBER: _ClassVar[int]
    AFTER_SEQ_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    source_id: str
    min_level: LogLevel
    query: str
    after_seq: int
    limit: int
    def __init__(self, source_id: _Optional[str] = ..., min_level: _Optional[_Union[LogLevel, str]] = ..., query: _Optional[str] = ..., after_seq: _Optional[int] = ..., limit: _Optional[int] = ...) -> None: ...

class ListLogsResponse(_message.Message):
    __slots__ = ("records", "last_seq")
    RECORDS_FIELD_NUMBER: _ClassVar[int]
    LAST_SEQ_FIELD_NUMBER: _ClassVar[int]
    records: _containers.RepeatedCompositeFieldContainer[LogRecord]
    last_seq: int
    def __init__(self, records: _Optional[_Iterable[_Union[LogRecord, _Mapping]]] = ..., last_seq: _Optional[int] = ...) -> None: ...
