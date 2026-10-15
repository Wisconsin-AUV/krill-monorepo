import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from krill.v1 import storage_pb2 as _storage_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetDatabaseRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class TableStats(_message.Message):
    __slots__ = ("schema", "name", "rows", "rows_estimated", "total_bytes", "index_bytes")
    SCHEMA_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    ROWS_FIELD_NUMBER: _ClassVar[int]
    ROWS_ESTIMATED_FIELD_NUMBER: _ClassVar[int]
    TOTAL_BYTES_FIELD_NUMBER: _ClassVar[int]
    INDEX_BYTES_FIELD_NUMBER: _ClassVar[int]
    schema: str
    name: str
    rows: int
    rows_estimated: bool
    total_bytes: int
    index_bytes: int
    def __init__(self, schema: _Optional[str] = ..., name: _Optional[str] = ..., rows: _Optional[int] = ..., rows_estimated: _Optional[bool] = ..., total_bytes: _Optional[int] = ..., index_bytes: _Optional[int] = ...) -> None: ...

class GetDatabaseResponse(_message.Message):
    __slots__ = ("size_bytes", "server_version", "schema_version", "latest_schema_version", "river_version", "latest_river_version", "tables")
    SIZE_BYTES_FIELD_NUMBER: _ClassVar[int]
    SERVER_VERSION_FIELD_NUMBER: _ClassVar[int]
    SCHEMA_VERSION_FIELD_NUMBER: _ClassVar[int]
    LATEST_SCHEMA_VERSION_FIELD_NUMBER: _ClassVar[int]
    RIVER_VERSION_FIELD_NUMBER: _ClassVar[int]
    LATEST_RIVER_VERSION_FIELD_NUMBER: _ClassVar[int]
    TABLES_FIELD_NUMBER: _ClassVar[int]
    size_bytes: int
    server_version: str
    schema_version: int
    latest_schema_version: int
    river_version: int
    latest_river_version: int
    tables: _containers.RepeatedCompositeFieldContainer[TableStats]
    def __init__(self, size_bytes: _Optional[int] = ..., server_version: _Optional[str] = ..., schema_version: _Optional[int] = ..., latest_schema_version: _Optional[int] = ..., river_version: _Optional[int] = ..., latest_river_version: _Optional[int] = ..., tables: _Optional[_Iterable[_Union[TableStats, _Mapping]]] = ...) -> None: ...

class Backup(_message.Message):
    __slots__ = ("id", "status", "scheduled", "created_by", "created_at", "finished_at", "size_bytes", "error", "filename")
    ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    SCHEDULED_FIELD_NUMBER: _ClassVar[int]
    CREATED_BY_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    FINISHED_AT_FIELD_NUMBER: _ClassVar[int]
    SIZE_BYTES_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    FILENAME_FIELD_NUMBER: _ClassVar[int]
    id: int
    status: _storage_pb2.JobStatus
    scheduled: bool
    created_by: str
    created_at: _timestamp_pb2.Timestamp
    finished_at: _timestamp_pb2.Timestamp
    size_bytes: int
    error: str
    filename: str
    def __init__(self, id: _Optional[int] = ..., status: _Optional[_Union[_storage_pb2.JobStatus, str]] = ..., scheduled: _Optional[bool] = ..., created_by: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., finished_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., size_bytes: _Optional[int] = ..., error: _Optional[str] = ..., filename: _Optional[str] = ...) -> None: ...

class BackupSchedule(_message.Message):
    __slots__ = ("hour", "keep", "next_at", "time_zone")
    HOUR_FIELD_NUMBER: _ClassVar[int]
    KEEP_FIELD_NUMBER: _ClassVar[int]
    NEXT_AT_FIELD_NUMBER: _ClassVar[int]
    TIME_ZONE_FIELD_NUMBER: _ClassVar[int]
    hour: int
    keep: int
    next_at: _timestamp_pb2.Timestamp
    time_zone: str
    def __init__(self, hour: _Optional[int] = ..., keep: _Optional[int] = ..., next_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., time_zone: _Optional[str] = ...) -> None: ...

class ListBackupsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListBackupsResponse(_message.Message):
    __slots__ = ("backups", "schedule", "unavailable")
    BACKUPS_FIELD_NUMBER: _ClassVar[int]
    SCHEDULE_FIELD_NUMBER: _ClassVar[int]
    UNAVAILABLE_FIELD_NUMBER: _ClassVar[int]
    backups: _containers.RepeatedCompositeFieldContainer[Backup]
    schedule: BackupSchedule
    unavailable: str
    def __init__(self, backups: _Optional[_Iterable[_Union[Backup, _Mapping]]] = ..., schedule: _Optional[_Union[BackupSchedule, _Mapping]] = ..., unavailable: _Optional[str] = ...) -> None: ...

class CreateBackupRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CreateBackupResponse(_message.Message):
    __slots__ = ("backup",)
    BACKUP_FIELD_NUMBER: _ClassVar[int]
    backup: Backup
    def __init__(self, backup: _Optional[_Union[Backup, _Mapping]] = ...) -> None: ...

class GetBackupDownloadRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class GetBackupDownloadResponse(_message.Message):
    __slots__ = ("url",)
    URL_FIELD_NUMBER: _ClassVar[int]
    url: str
    def __init__(self, url: _Optional[str] = ...) -> None: ...

class DeleteBackupRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class DeleteBackupResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
