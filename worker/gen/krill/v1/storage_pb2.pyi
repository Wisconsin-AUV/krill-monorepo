import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ObjectKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    OBJECT_KIND_UNSPECIFIED: _ClassVar[ObjectKind]
    OBJECT_KIND_SOURCE: _ClassVar[ObjectKind]
    OBJECT_KIND_FRAME: _ClassVar[ObjectKind]
    OBJECT_KIND_EMBEDDING: _ClassVar[ObjectKind]
    OBJECT_KIND_EXPORT: _ClassVar[ObjectKind]
    OBJECT_KIND_LABEL_EXAMPLE: _ClassVar[ObjectKind]
    OBJECT_KIND_BACKUP: _ClassVar[ObjectKind]
    OBJECT_KIND_UPLOAD: _ClassVar[ObjectKind]
    OBJECT_KIND_OTHER: _ClassVar[ObjectKind]
    OBJECT_KIND_MODEL: _ClassVar[ObjectKind]

class JobStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    JOB_STATUS_UNSPECIFIED: _ClassVar[JobStatus]
    JOB_STATUS_QUEUED: _ClassVar[JobStatus]
    JOB_STATUS_RUNNING: _ClassVar[JobStatus]
    JOB_STATUS_DONE: _ClassVar[JobStatus]
    JOB_STATUS_FAILED: _ClassVar[JobStatus]
OBJECT_KIND_UNSPECIFIED: ObjectKind
OBJECT_KIND_SOURCE: ObjectKind
OBJECT_KIND_FRAME: ObjectKind
OBJECT_KIND_EMBEDDING: ObjectKind
OBJECT_KIND_EXPORT: ObjectKind
OBJECT_KIND_LABEL_EXAMPLE: ObjectKind
OBJECT_KIND_BACKUP: ObjectKind
OBJECT_KIND_UPLOAD: ObjectKind
OBJECT_KIND_OTHER: ObjectKind
OBJECT_KIND_MODEL: ObjectKind
JOB_STATUS_UNSPECIFIED: JobStatus
JOB_STATUS_QUEUED: JobStatus
JOB_STATUS_RUNNING: JobStatus
JOB_STATUS_DONE: JobStatus
JOB_STATUS_FAILED: JobStatus

class ObjectCategory(_message.Message):
    __slots__ = ("kind", "bytes", "objects", "orphan_bytes", "orphan_objects", "missing_objects", "missing_samples")
    KIND_FIELD_NUMBER: _ClassVar[int]
    BYTES_FIELD_NUMBER: _ClassVar[int]
    OBJECTS_FIELD_NUMBER: _ClassVar[int]
    ORPHAN_BYTES_FIELD_NUMBER: _ClassVar[int]
    ORPHAN_OBJECTS_FIELD_NUMBER: _ClassVar[int]
    MISSING_OBJECTS_FIELD_NUMBER: _ClassVar[int]
    MISSING_SAMPLES_FIELD_NUMBER: _ClassVar[int]
    kind: ObjectKind
    bytes: int
    objects: int
    orphan_bytes: int
    orphan_objects: int
    missing_objects: int
    missing_samples: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, kind: _Optional[_Union[ObjectKind, str]] = ..., bytes: _Optional[int] = ..., objects: _Optional[int] = ..., orphan_bytes: _Optional[int] = ..., orphan_objects: _Optional[int] = ..., missing_objects: _Optional[int] = ..., missing_samples: _Optional[_Iterable[str]] = ...) -> None: ...

class VideoUsage(_message.Message):
    __slots__ = ("video_id", "name", "frame_bytes", "frames", "embedding_bytes", "embeddings", "source_bytes", "orphan_bytes")
    VIDEO_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    FRAME_BYTES_FIELD_NUMBER: _ClassVar[int]
    FRAMES_FIELD_NUMBER: _ClassVar[int]
    EMBEDDING_BYTES_FIELD_NUMBER: _ClassVar[int]
    EMBEDDINGS_FIELD_NUMBER: _ClassVar[int]
    SOURCE_BYTES_FIELD_NUMBER: _ClassVar[int]
    ORPHAN_BYTES_FIELD_NUMBER: _ClassVar[int]
    video_id: int
    name: str
    frame_bytes: int
    frames: int
    embedding_bytes: int
    embeddings: int
    source_bytes: int
    orphan_bytes: int
    def __init__(self, video_id: _Optional[int] = ..., name: _Optional[str] = ..., frame_bytes: _Optional[int] = ..., frames: _Optional[int] = ..., embedding_bytes: _Optional[int] = ..., embeddings: _Optional[int] = ..., source_bytes: _Optional[int] = ..., orphan_bytes: _Optional[int] = ...) -> None: ...

class Orphan(_message.Message):
    __slots__ = ("key", "kind", "bytes", "modified_at")
    KEY_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    BYTES_FIELD_NUMBER: _ClassVar[int]
    MODIFIED_AT_FIELD_NUMBER: _ClassVar[int]
    key: str
    kind: ObjectKind
    bytes: int
    modified_at: _timestamp_pb2.Timestamp
    def __init__(self, key: _Optional[str] = ..., kind: _Optional[_Union[ObjectKind, str]] = ..., bytes: _Optional[int] = ..., modified_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class StorageCleanup(_message.Message):
    __slots__ = ("status", "requested_by", "finished_at", "deleted_objects", "deleted_bytes", "skipped_objects", "error")
    STATUS_FIELD_NUMBER: _ClassVar[int]
    REQUESTED_BY_FIELD_NUMBER: _ClassVar[int]
    FINISHED_AT_FIELD_NUMBER: _ClassVar[int]
    DELETED_OBJECTS_FIELD_NUMBER: _ClassVar[int]
    DELETED_BYTES_FIELD_NUMBER: _ClassVar[int]
    SKIPPED_OBJECTS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    status: JobStatus
    requested_by: str
    finished_at: _timestamp_pb2.Timestamp
    deleted_objects: int
    deleted_bytes: int
    skipped_objects: int
    error: str
    def __init__(self, status: _Optional[_Union[JobStatus, str]] = ..., requested_by: _Optional[str] = ..., finished_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., deleted_objects: _Optional[int] = ..., deleted_bytes: _Optional[int] = ..., skipped_objects: _Optional[int] = ..., error: _Optional[str] = ...) -> None: ...

class StorageScan(_message.Message):
    __slots__ = ("id", "status", "error", "requested_by", "created_at", "finished_at", "categories", "videos", "cleanup")
    ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    REQUESTED_BY_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    FINISHED_AT_FIELD_NUMBER: _ClassVar[int]
    CATEGORIES_FIELD_NUMBER: _ClassVar[int]
    VIDEOS_FIELD_NUMBER: _ClassVar[int]
    CLEANUP_FIELD_NUMBER: _ClassVar[int]
    id: int
    status: JobStatus
    error: str
    requested_by: str
    created_at: _timestamp_pb2.Timestamp
    finished_at: _timestamp_pb2.Timestamp
    categories: _containers.RepeatedCompositeFieldContainer[ObjectCategory]
    videos: _containers.RepeatedCompositeFieldContainer[VideoUsage]
    cleanup: StorageCleanup
    def __init__(self, id: _Optional[int] = ..., status: _Optional[_Union[JobStatus, str]] = ..., error: _Optional[str] = ..., requested_by: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., finished_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., categories: _Optional[_Iterable[_Union[ObjectCategory, _Mapping]]] = ..., videos: _Optional[_Iterable[_Union[VideoUsage, _Mapping]]] = ..., cleanup: _Optional[_Union[StorageCleanup, _Mapping]] = ...) -> None: ...

class GetStorageReportRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetStorageReportResponse(_message.Message):
    __slots__ = ("scan", "previous", "orphans")
    SCAN_FIELD_NUMBER: _ClassVar[int]
    PREVIOUS_FIELD_NUMBER: _ClassVar[int]
    ORPHANS_FIELD_NUMBER: _ClassVar[int]
    scan: StorageScan
    previous: StorageScan
    orphans: _containers.RepeatedCompositeFieldContainer[Orphan]
    def __init__(self, scan: _Optional[_Union[StorageScan, _Mapping]] = ..., previous: _Optional[_Union[StorageScan, _Mapping]] = ..., orphans: _Optional[_Iterable[_Union[Orphan, _Mapping]]] = ...) -> None: ...

class ScanStorageRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ScanStorageResponse(_message.Message):
    __slots__ = ("scan",)
    SCAN_FIELD_NUMBER: _ClassVar[int]
    scan: StorageScan
    def __init__(self, scan: _Optional[_Union[StorageScan, _Mapping]] = ...) -> None: ...

class CleanUpStorageRequest(_message.Message):
    __slots__ = ("scan_id",)
    SCAN_ID_FIELD_NUMBER: _ClassVar[int]
    scan_id: int
    def __init__(self, scan_id: _Optional[int] = ...) -> None: ...

class CleanUpStorageResponse(_message.Message):
    __slots__ = ("scan",)
    SCAN_FIELD_NUMBER: _ClassVar[int]
    scan: StorageScan
    def __init__(self, scan: _Optional[_Union[StorageScan, _Mapping]] = ...) -> None: ...
