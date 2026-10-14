import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from krill.v1 import system_pb2 as _system_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class UpdateState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    UPDATE_STATE_UNSPECIFIED: _ClassVar[UpdateState]
    UPDATE_STATE_UP_TO_DATE: _ClassVar[UpdateState]
    UPDATE_STATE_AVAILABLE: _ClassVar[UpdateState]
    UPDATE_STATE_AHEAD: _ClassVar[UpdateState]

class CheckStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CHECK_STATUS_UNSPECIFIED: _ClassVar[CheckStatus]
    CHECK_STATUS_OK: _ClassVar[CheckStatus]
    CHECK_STATUS_WARNING: _ClassVar[CheckStatus]
    CHECK_STATUS_ERROR: _ClassVar[CheckStatus]
    CHECK_STATUS_OFF: _ClassVar[CheckStatus]

class SettingKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SETTING_KIND_UNSPECIFIED: _ClassVar[SettingKind]
    SETTING_KIND_STRING: _ClassVar[SettingKind]
    SETTING_KIND_SECRET: _ClassVar[SettingKind]
    SETTING_KIND_BOOL: _ClassVar[SettingKind]
    SETTING_KIND_INT: _ClassVar[SettingKind]
    SETTING_KIND_FLOAT: _ClassVar[SettingKind]
    SETTING_KIND_CHOICE: _ClassVar[SettingKind]

class SettingSource(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SETTING_SOURCE_UNSPECIFIED: _ClassVar[SettingSource]
    SETTING_SOURCE_DEFAULT: _ClassVar[SettingSource]
    SETTING_SOURCE_ENVIRONMENT: _ClassVar[SettingSource]
    SETTING_SOURCE_SAVED: _ClassVar[SettingSource]

class NodeRole(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    NODE_ROLE_UNSPECIFIED: _ClassVar[NodeRole]
    NODE_ROLE_API: _ClassVar[NodeRole]
    NODE_ROLE_WORKER: _ClassVar[NodeRole]
UPDATE_STATE_UNSPECIFIED: UpdateState
UPDATE_STATE_UP_TO_DATE: UpdateState
UPDATE_STATE_AVAILABLE: UpdateState
UPDATE_STATE_AHEAD: UpdateState
CHECK_STATUS_UNSPECIFIED: CheckStatus
CHECK_STATUS_OK: CheckStatus
CHECK_STATUS_WARNING: CheckStatus
CHECK_STATUS_ERROR: CheckStatus
CHECK_STATUS_OFF: CheckStatus
SETTING_KIND_UNSPECIFIED: SettingKind
SETTING_KIND_STRING: SettingKind
SETTING_KIND_SECRET: SettingKind
SETTING_KIND_BOOL: SettingKind
SETTING_KIND_INT: SettingKind
SETTING_KIND_FLOAT: SettingKind
SETTING_KIND_CHOICE: SettingKind
SETTING_SOURCE_UNSPECIFIED: SettingSource
SETTING_SOURCE_DEFAULT: SettingSource
SETTING_SOURCE_ENVIRONMENT: SettingSource
SETTING_SOURCE_SAVED: SettingSource
NODE_ROLE_UNSPECIFIED: NodeRole
NODE_ROLE_API: NodeRole
NODE_ROLE_WORKER: NodeRole

class GetStatusRequest(_message.Message):
    __slots__ = ("refresh",)
    REFRESH_FIELD_NUMBER: _ClassVar[int]
    refresh: bool
    def __init__(self, refresh: _Optional[bool] = ...) -> None: ...

class GetStatusResponse(_message.Message):
    __slots__ = ("version", "checks", "started_at")
    VERSION_FIELD_NUMBER: _ClassVar[int]
    CHECKS_FIELD_NUMBER: _ClassVar[int]
    STARTED_AT_FIELD_NUMBER: _ClassVar[int]
    version: VersionStatus
    checks: _containers.RepeatedCompositeFieldContainer[ServiceCheck]
    started_at: _timestamp_pb2.Timestamp
    def __init__(self, version: _Optional[_Union[VersionStatus, _Mapping]] = ..., checks: _Optional[_Iterable[_Union[ServiceCheck, _Mapping]]] = ..., started_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class VersionStatus(_message.Message):
    __slots__ = ("current", "latest", "state", "release_url", "checked_at", "error", "releases", "check_enabled", "release", "commits_ahead", "commit", "dirty")
    CURRENT_FIELD_NUMBER: _ClassVar[int]
    LATEST_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    RELEASE_URL_FIELD_NUMBER: _ClassVar[int]
    CHECKED_AT_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    RELEASES_FIELD_NUMBER: _ClassVar[int]
    CHECK_ENABLED_FIELD_NUMBER: _ClassVar[int]
    RELEASE_FIELD_NUMBER: _ClassVar[int]
    COMMITS_AHEAD_FIELD_NUMBER: _ClassVar[int]
    COMMIT_FIELD_NUMBER: _ClassVar[int]
    DIRTY_FIELD_NUMBER: _ClassVar[int]
    current: str
    latest: str
    state: UpdateState
    release_url: str
    checked_at: _timestamp_pb2.Timestamp
    error: str
    releases: _containers.RepeatedCompositeFieldContainer[Release]
    check_enabled: bool
    release: str
    commits_ahead: int
    commit: str
    dirty: bool
    def __init__(self, current: _Optional[str] = ..., latest: _Optional[str] = ..., state: _Optional[_Union[UpdateState, str]] = ..., release_url: _Optional[str] = ..., checked_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., error: _Optional[str] = ..., releases: _Optional[_Iterable[_Union[Release, _Mapping]]] = ..., check_enabled: _Optional[bool] = ..., release: _Optional[str] = ..., commits_ahead: _Optional[int] = ..., commit: _Optional[str] = ..., dirty: _Optional[bool] = ...) -> None: ...

class Release(_message.Message):
    __slots__ = ("version", "date", "sections")
    VERSION_FIELD_NUMBER: _ClassVar[int]
    DATE_FIELD_NUMBER: _ClassVar[int]
    SECTIONS_FIELD_NUMBER: _ClassVar[int]
    version: str
    date: str
    sections: _containers.RepeatedCompositeFieldContainer[ReleaseSection]
    def __init__(self, version: _Optional[str] = ..., date: _Optional[str] = ..., sections: _Optional[_Iterable[_Union[ReleaseSection, _Mapping]]] = ...) -> None: ...

class ReleaseSection(_message.Message):
    __slots__ = ("title", "items")
    TITLE_FIELD_NUMBER: _ClassVar[int]
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    title: str
    items: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, title: _Optional[str] = ..., items: _Optional[_Iterable[str]] = ...) -> None: ...

class ServiceCheck(_message.Message):
    __slots__ = ("name", "status", "detail")
    NAME_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    DETAIL_FIELD_NUMBER: _ClassVar[int]
    name: str
    status: CheckStatus
    detail: str
    def __init__(self, name: _Optional[str] = ..., status: _Optional[_Union[CheckStatus, str]] = ..., detail: _Optional[str] = ...) -> None: ...

class ListSettingsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListSettingsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: _containers.RepeatedCompositeFieldContainer[Setting]
    def __init__(self, settings: _Optional[_Iterable[_Union[Setting, _Mapping]]] = ...) -> None: ...

class SettingChoice(_message.Message):
    __slots__ = ("value", "label")
    VALUE_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    value: str
    label: str
    def __init__(self, value: _Optional[str] = ..., label: _Optional[str] = ...) -> None: ...

class Setting(_message.Message):
    __slots__ = ("key", "group", "label", "description", "kind", "choices", "default_value", "env_only", "source", "value", "is_set", "updated_at", "updated_by")
    KEY_FIELD_NUMBER: _ClassVar[int]
    GROUP_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    CHOICES_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_VALUE_FIELD_NUMBER: _ClassVar[int]
    ENV_ONLY_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    IS_SET_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_FIELD_NUMBER: _ClassVar[int]
    UPDATED_BY_FIELD_NUMBER: _ClassVar[int]
    key: str
    group: str
    label: str
    description: str
    kind: SettingKind
    choices: _containers.RepeatedCompositeFieldContainer[SettingChoice]
    default_value: str
    env_only: bool
    source: SettingSource
    value: str
    is_set: bool
    updated_at: _timestamp_pb2.Timestamp
    updated_by: str
    def __init__(self, key: _Optional[str] = ..., group: _Optional[str] = ..., label: _Optional[str] = ..., description: _Optional[str] = ..., kind: _Optional[_Union[SettingKind, str]] = ..., choices: _Optional[_Iterable[_Union[SettingChoice, _Mapping]]] = ..., default_value: _Optional[str] = ..., env_only: _Optional[bool] = ..., source: _Optional[_Union[SettingSource, str]] = ..., value: _Optional[str] = ..., is_set: _Optional[bool] = ..., updated_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., updated_by: _Optional[str] = ...) -> None: ...

class SettingChange(_message.Message):
    __slots__ = ("key", "value", "use_default")
    KEY_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    USE_DEFAULT_FIELD_NUMBER: _ClassVar[int]
    key: str
    value: str
    use_default: bool
    def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ..., use_default: _Optional[bool] = ...) -> None: ...

class UpdateSettingsRequest(_message.Message):
    __slots__ = ("changes",)
    CHANGES_FIELD_NUMBER: _ClassVar[int]
    changes: _containers.RepeatedCompositeFieldContainer[SettingChange]
    def __init__(self, changes: _Optional[_Iterable[_Union[SettingChange, _Mapping]]] = ...) -> None: ...

class UpdateSettingsResponse(_message.Message):
    __slots__ = ("settings",)
    SETTINGS_FIELD_NUMBER: _ClassVar[int]
    settings: _containers.RepeatedCompositeFieldContainer[Setting]
    def __init__(self, settings: _Optional[_Iterable[_Union[Setting, _Mapping]]] = ...) -> None: ...

class SettingError(_message.Message):
    __slots__ = ("key", "message")
    KEY_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    key: str
    message: str
    def __init__(self, key: _Optional[str] = ..., message: _Optional[str] = ...) -> None: ...

class SendTestEmailRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SendTestEmailResponse(_message.Message):
    __slots__ = ("to", "logged")
    TO_FIELD_NUMBER: _ClassVar[int]
    LOGGED_FIELD_NUMBER: _ClassVar[int]
    to: str
    logged: bool
    def __init__(self, to: _Optional[str] = ..., logged: _Optional[bool] = ...) -> None: ...

class GetComputeRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetComputeResponse(_message.Message):
    __slots__ = ("nodes", "queue", "sample_interval_seconds", "storage")
    NODES_FIELD_NUMBER: _ClassVar[int]
    QUEUE_FIELD_NUMBER: _ClassVar[int]
    SAMPLE_INTERVAL_SECONDS_FIELD_NUMBER: _ClassVar[int]
    STORAGE_FIELD_NUMBER: _ClassVar[int]
    nodes: _containers.RepeatedCompositeFieldContainer[ComputeNode]
    queue: GpuQueue
    sample_interval_seconds: int
    storage: ObjectStorage
    def __init__(self, nodes: _Optional[_Iterable[_Union[ComputeNode, _Mapping]]] = ..., queue: _Optional[_Union[GpuQueue, _Mapping]] = ..., sample_interval_seconds: _Optional[int] = ..., storage: _Optional[_Union[ObjectStorage, _Mapping]] = ...) -> None: ...

class ObjectStorage(_message.Message):
    __slots__ = ("used_bytes", "total_bytes", "error")
    USED_BYTES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_BYTES_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    used_bytes: int
    total_bytes: int
    error: str
    def __init__(self, used_bytes: _Optional[int] = ..., total_bytes: _Optional[int] = ..., error: _Optional[str] = ...) -> None: ...

class ComputeNode(_message.Message):
    __slots__ = ("id", "role", "hostname", "version", "online", "last_seen", "started_at", "model", "accelerator", "host", "gpus", "history", "task")
    ID_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    HOSTNAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    ONLINE_FIELD_NUMBER: _ClassVar[int]
    LAST_SEEN_FIELD_NUMBER: _ClassVar[int]
    STARTED_AT_FIELD_NUMBER: _ClassVar[int]
    MODEL_FIELD_NUMBER: _ClassVar[int]
    ACCELERATOR_FIELD_NUMBER: _ClassVar[int]
    HOST_FIELD_NUMBER: _ClassVar[int]
    GPUS_FIELD_NUMBER: _ClassVar[int]
    HISTORY_FIELD_NUMBER: _ClassVar[int]
    TASK_FIELD_NUMBER: _ClassVar[int]
    id: str
    role: NodeRole
    hostname: str
    version: str
    online: bool
    last_seen: _timestamp_pb2.Timestamp
    started_at: _timestamp_pb2.Timestamp
    model: str
    accelerator: str
    host: _system_pb2.HostStats
    gpus: _containers.RepeatedCompositeFieldContainer[_system_pb2.GpuStats]
    history: _containers.RepeatedCompositeFieldContainer[ComputeSample]
    task: RunningTask
    def __init__(self, id: _Optional[str] = ..., role: _Optional[_Union[NodeRole, str]] = ..., hostname: _Optional[str] = ..., version: _Optional[str] = ..., online: _Optional[bool] = ..., last_seen: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., started_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., model: _Optional[str] = ..., accelerator: _Optional[str] = ..., host: _Optional[_Union[_system_pb2.HostStats, _Mapping]] = ..., gpus: _Optional[_Iterable[_Union[_system_pb2.GpuStats, _Mapping]]] = ..., history: _Optional[_Iterable[_Union[ComputeSample, _Mapping]]] = ..., task: _Optional[_Union[RunningTask, _Mapping]] = ...) -> None: ...

class ComputeSample(_message.Message):
    __slots__ = ("at", "cpu_percent", "memory_used_bytes", "gpus")
    AT_FIELD_NUMBER: _ClassVar[int]
    CPU_PERCENT_FIELD_NUMBER: _ClassVar[int]
    MEMORY_USED_BYTES_FIELD_NUMBER: _ClassVar[int]
    GPUS_FIELD_NUMBER: _ClassVar[int]
    at: _timestamp_pb2.Timestamp
    cpu_percent: float
    memory_used_bytes: int
    gpus: _containers.RepeatedCompositeFieldContainer[GpuSample]
    def __init__(self, at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., cpu_percent: _Optional[float] = ..., memory_used_bytes: _Optional[int] = ..., gpus: _Optional[_Iterable[_Union[GpuSample, _Mapping]]] = ...) -> None: ...

class GpuSample(_message.Message):
    __slots__ = ("utilization_percent", "memory_used_bytes")
    UTILIZATION_PERCENT_FIELD_NUMBER: _ClassVar[int]
    MEMORY_USED_BYTES_FIELD_NUMBER: _ClassVar[int]
    utilization_percent: float
    memory_used_bytes: int
    def __init__(self, utilization_percent: _Optional[float] = ..., memory_used_bytes: _Optional[int] = ...) -> None: ...

class RunningTask(_message.Message):
    __slots__ = ("id", "kind", "frames", "started_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    FRAMES_FIELD_NUMBER: _ClassVar[int]
    STARTED_AT_FIELD_NUMBER: _ClassVar[int]
    id: int
    kind: str
    frames: int
    started_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[int] = ..., kind: _Optional[str] = ..., frames: _Optional[int] = ..., started_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GpuQueue(_message.Message):
    __slots__ = ("interactive", "background")
    INTERACTIVE_FIELD_NUMBER: _ClassVar[int]
    BACKGROUND_FIELD_NUMBER: _ClassVar[int]
    interactive: int
    background: int
    def __init__(self, interactive: _Optional[int] = ..., background: _Optional[int] = ...) -> None: ...
