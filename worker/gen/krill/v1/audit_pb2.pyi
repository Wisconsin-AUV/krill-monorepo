import datetime

from google.protobuf import struct_pb2 as _struct_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from krill.v1 import stats_pb2 as _stats_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class AuditEntry(_message.Message):
    __slots__ = ("id", "at", "actor", "actor_name", "action", "target_type", "target_id", "target_name", "details", "ip")
    ID_FIELD_NUMBER: _ClassVar[int]
    AT_FIELD_NUMBER: _ClassVar[int]
    ACTOR_FIELD_NUMBER: _ClassVar[int]
    ACTOR_NAME_FIELD_NUMBER: _ClassVar[int]
    ACTION_FIELD_NUMBER: _ClassVar[int]
    TARGET_TYPE_FIELD_NUMBER: _ClassVar[int]
    TARGET_ID_FIELD_NUMBER: _ClassVar[int]
    TARGET_NAME_FIELD_NUMBER: _ClassVar[int]
    DETAILS_FIELD_NUMBER: _ClassVar[int]
    IP_FIELD_NUMBER: _ClassVar[int]
    id: int
    at: _timestamp_pb2.Timestamp
    actor: _stats_pb2.Profile
    actor_name: str
    action: str
    target_type: str
    target_id: str
    target_name: str
    details: _struct_pb2.Struct
    ip: str
    def __init__(self, id: _Optional[int] = ..., at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., actor: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., actor_name: _Optional[str] = ..., action: _Optional[str] = ..., target_type: _Optional[str] = ..., target_id: _Optional[str] = ..., target_name: _Optional[str] = ..., details: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., ip: _Optional[str] = ...) -> None: ...

class ListAuditLogRequest(_message.Message):
    __slots__ = ("actor_id", "action_prefix", "target_type", "target_id", "page_size", "page_token")
    ACTOR_ID_FIELD_NUMBER: _ClassVar[int]
    ACTION_PREFIX_FIELD_NUMBER: _ClassVar[int]
    TARGET_TYPE_FIELD_NUMBER: _ClassVar[int]
    TARGET_ID_FIELD_NUMBER: _ClassVar[int]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    actor_id: str
    action_prefix: str
    target_type: str
    target_id: str
    page_size: int
    page_token: str
    def __init__(self, actor_id: _Optional[str] = ..., action_prefix: _Optional[str] = ..., target_type: _Optional[str] = ..., target_id: _Optional[str] = ..., page_size: _Optional[int] = ..., page_token: _Optional[str] = ...) -> None: ...

class ListAuditLogResponse(_message.Message):
    __slots__ = ("entries", "next_page_token")
    ENTRIES_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    entries: _containers.RepeatedCompositeFieldContainer[AuditEntry]
    next_page_token: str
    def __init__(self, entries: _Optional[_Iterable[_Union[AuditEntry, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...

class ListAuditFiltersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListAuditFiltersResponse(_message.Message):
    __slots__ = ("actions", "actors")
    ACTIONS_FIELD_NUMBER: _ClassVar[int]
    ACTORS_FIELD_NUMBER: _ClassVar[int]
    actions: _containers.RepeatedScalarFieldContainer[str]
    actors: _containers.RepeatedCompositeFieldContainer[_stats_pb2.Profile]
    def __init__(self, actions: _Optional[_Iterable[str]] = ..., actors: _Optional[_Iterable[_Union[_stats_pb2.Profile, _Mapping]]] = ...) -> None: ...
