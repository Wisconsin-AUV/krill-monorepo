import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ExportStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    EXPORT_STATUS_UNSPECIFIED: _ClassVar[ExportStatus]
    EXPORT_STATUS_QUEUED: _ClassVar[ExportStatus]
    EXPORT_STATUS_RUNNING: _ClassVar[ExportStatus]
    EXPORT_STATUS_READY: _ClassVar[ExportStatus]
    EXPORT_STATUS_FAILED: _ClassVar[ExportStatus]
EXPORT_STATUS_UNSPECIFIED: ExportStatus
EXPORT_STATUS_QUEUED: ExportStatus
EXPORT_STATUS_RUNNING: ExportStatus
EXPORT_STATUS_READY: ExportStatus
EXPORT_STATUS_FAILED: ExportStatus

class ExportOptions(_message.Message):
    __slots__ = ("val_fraction", "stride", "dedup", "dedup_distance", "video_ids")
    VAL_FRACTION_FIELD_NUMBER: _ClassVar[int]
    STRIDE_FIELD_NUMBER: _ClassVar[int]
    DEDUP_FIELD_NUMBER: _ClassVar[int]
    DEDUP_DISTANCE_FIELD_NUMBER: _ClassVar[int]
    VIDEO_IDS_FIELD_NUMBER: _ClassVar[int]
    val_fraction: float
    stride: int
    dedup: bool
    dedup_distance: int
    video_ids: _containers.RepeatedScalarFieldContainer[int]
    def __init__(self, val_fraction: _Optional[float] = ..., stride: _Optional[int] = ..., dedup: _Optional[bool] = ..., dedup_distance: _Optional[int] = ..., video_ids: _Optional[_Iterable[int]] = ...) -> None: ...

class ExportClassStats(_message.Message):
    __slots__ = ("id", "name", "train_boxes", "val_boxes", "clips", "videos")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    TRAIN_BOXES_FIELD_NUMBER: _ClassVar[int]
    VAL_BOXES_FIELD_NUMBER: _ClassVar[int]
    CLIPS_FIELD_NUMBER: _ClassVar[int]
    VIDEOS_FIELD_NUMBER: _ClassVar[int]
    id: int
    name: str
    train_boxes: int
    val_boxes: int
    clips: int
    videos: int
    def __init__(self, id: _Optional[int] = ..., name: _Optional[str] = ..., train_boxes: _Optional[int] = ..., val_boxes: _Optional[int] = ..., clips: _Optional[int] = ..., videos: _Optional[int] = ...) -> None: ...

class ExportVideoStats(_message.Message):
    __slots__ = ("video_id", "name", "split", "pinned", "images", "boxes")
    VIDEO_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    SPLIT_FIELD_NUMBER: _ClassVar[int]
    PINNED_FIELD_NUMBER: _ClassVar[int]
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    BOXES_FIELD_NUMBER: _ClassVar[int]
    video_id: int
    name: str
    split: str
    pinned: bool
    images: int
    boxes: int
    def __init__(self, video_id: _Optional[int] = ..., name: _Optional[str] = ..., split: _Optional[str] = ..., pinned: _Optional[bool] = ..., images: _Optional[int] = ..., boxes: _Optional[int] = ...) -> None: ...

class ExportStats(_message.Message):
    __slots__ = ("train_images", "val_images", "train_boxes", "val_boxes", "negative_images", "unlabeled_frames", "incomplete_frames", "unverified_frames", "stride_skipped", "duplicate_skipped", "classes", "videos")
    TRAIN_IMAGES_FIELD_NUMBER: _ClassVar[int]
    VAL_IMAGES_FIELD_NUMBER: _ClassVar[int]
    TRAIN_BOXES_FIELD_NUMBER: _ClassVar[int]
    VAL_BOXES_FIELD_NUMBER: _ClassVar[int]
    NEGATIVE_IMAGES_FIELD_NUMBER: _ClassVar[int]
    UNLABELED_FRAMES_FIELD_NUMBER: _ClassVar[int]
    INCOMPLETE_FRAMES_FIELD_NUMBER: _ClassVar[int]
    UNVERIFIED_FRAMES_FIELD_NUMBER: _ClassVar[int]
    STRIDE_SKIPPED_FIELD_NUMBER: _ClassVar[int]
    DUPLICATE_SKIPPED_FIELD_NUMBER: _ClassVar[int]
    CLASSES_FIELD_NUMBER: _ClassVar[int]
    VIDEOS_FIELD_NUMBER: _ClassVar[int]
    train_images: int
    val_images: int
    train_boxes: int
    val_boxes: int
    negative_images: int
    unlabeled_frames: int
    incomplete_frames: int
    unverified_frames: int
    stride_skipped: int
    duplicate_skipped: int
    classes: _containers.RepeatedCompositeFieldContainer[ExportClassStats]
    videos: _containers.RepeatedCompositeFieldContainer[ExportVideoStats]
    def __init__(self, train_images: _Optional[int] = ..., val_images: _Optional[int] = ..., train_boxes: _Optional[int] = ..., val_boxes: _Optional[int] = ..., negative_images: _Optional[int] = ..., unlabeled_frames: _Optional[int] = ..., incomplete_frames: _Optional[int] = ..., unverified_frames: _Optional[int] = ..., stride_skipped: _Optional[int] = ..., duplicate_skipped: _Optional[int] = ..., classes: _Optional[_Iterable[_Union[ExportClassStats, _Mapping]]] = ..., videos: _Optional[_Iterable[_Union[ExportVideoStats, _Mapping]]] = ...) -> None: ...

class Export(_message.Message):
    __slots__ = ("id", "name", "status", "error", "progress", "options", "stats", "size_bytes", "download_url", "created_at", "finished_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    PROGRESS_FIELD_NUMBER: _ClassVar[int]
    OPTIONS_FIELD_NUMBER: _ClassVar[int]
    STATS_FIELD_NUMBER: _ClassVar[int]
    SIZE_BYTES_FIELD_NUMBER: _ClassVar[int]
    DOWNLOAD_URL_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    FINISHED_AT_FIELD_NUMBER: _ClassVar[int]
    id: int
    name: str
    status: ExportStatus
    error: str
    progress: float
    options: ExportOptions
    stats: ExportStats
    size_bytes: int
    download_url: str
    created_at: _timestamp_pb2.Timestamp
    finished_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[int] = ..., name: _Optional[str] = ..., status: _Optional[_Union[ExportStatus, str]] = ..., error: _Optional[str] = ..., progress: _Optional[float] = ..., options: _Optional[_Union[ExportOptions, _Mapping]] = ..., stats: _Optional[_Union[ExportStats, _Mapping]] = ..., size_bytes: _Optional[int] = ..., download_url: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., finished_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class PreviewExportRequest(_message.Message):
    __slots__ = ("options",)
    OPTIONS_FIELD_NUMBER: _ClassVar[int]
    options: ExportOptions
    def __init__(self, options: _Optional[_Union[ExportOptions, _Mapping]] = ...) -> None: ...

class PreviewExportResponse(_message.Message):
    __slots__ = ("stats",)
    STATS_FIELD_NUMBER: _ClassVar[int]
    stats: ExportStats
    def __init__(self, stats: _Optional[_Union[ExportStats, _Mapping]] = ...) -> None: ...

class CreateExportRequest(_message.Message):
    __slots__ = ("name", "options")
    NAME_FIELD_NUMBER: _ClassVar[int]
    OPTIONS_FIELD_NUMBER: _ClassVar[int]
    name: str
    options: ExportOptions
    def __init__(self, name: _Optional[str] = ..., options: _Optional[_Union[ExportOptions, _Mapping]] = ...) -> None: ...

class CreateExportResponse(_message.Message):
    __slots__ = ("export",)
    EXPORT_FIELD_NUMBER: _ClassVar[int]
    export: Export
    def __init__(self, export: _Optional[_Union[Export, _Mapping]] = ...) -> None: ...

class ListExportsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListExportsResponse(_message.Message):
    __slots__ = ("exports",)
    EXPORTS_FIELD_NUMBER: _ClassVar[int]
    exports: _containers.RepeatedCompositeFieldContainer[Export]
    def __init__(self, exports: _Optional[_Iterable[_Union[Export, _Mapping]]] = ...) -> None: ...

class DeleteExportRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class DeleteExportResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
