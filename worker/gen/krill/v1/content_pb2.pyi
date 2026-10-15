import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from krill.v1 import stats_pb2 as _stats_pb2
from krill.v1 import video_pb2 as _video_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class TrashKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    TRASH_KIND_UNSPECIFIED: _ClassVar[TrashKind]
    TRASH_KIND_VIDEO: _ClassVar[TrashKind]
    TRASH_KIND_CLIP: _ClassVar[TrashKind]

class ContentJobKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CONTENT_JOB_KIND_UNSPECIFIED: _ClassVar[ContentJobKind]
    CONTENT_JOB_KIND_REEXTRACT: _ClassVar[ContentJobKind]
    CONTENT_JOB_KIND_EMBED: _ClassVar[ContentJobKind]
    CONTENT_JOB_KIND_PURGE: _ClassVar[ContentJobKind]
TRASH_KIND_UNSPECIFIED: TrashKind
TRASH_KIND_VIDEO: TrashKind
TRASH_KIND_CLIP: TrashKind
CONTENT_JOB_KIND_UNSPECIFIED: ContentJobKind
CONTENT_JOB_KIND_REEXTRACT: ContentJobKind
CONTENT_JOB_KIND_EMBED: ContentJobKind
CONTENT_JOB_KIND_PURGE: ContentJobKind

class TrashItem(_message.Message):
    __slots__ = ("kind", "id", "video_id", "video_name", "clip_index", "deleted_at", "deleted_by", "purge_at", "size_bytes", "frame_count", "clip_count", "labeled_frame_count", "track_count", "box_count", "thumbnail_url")
    KIND_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    VIDEO_ID_FIELD_NUMBER: _ClassVar[int]
    VIDEO_NAME_FIELD_NUMBER: _ClassVar[int]
    CLIP_INDEX_FIELD_NUMBER: _ClassVar[int]
    DELETED_AT_FIELD_NUMBER: _ClassVar[int]
    DELETED_BY_FIELD_NUMBER: _ClassVar[int]
    PURGE_AT_FIELD_NUMBER: _ClassVar[int]
    SIZE_BYTES_FIELD_NUMBER: _ClassVar[int]
    FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    CLIP_COUNT_FIELD_NUMBER: _ClassVar[int]
    LABELED_FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    TRACK_COUNT_FIELD_NUMBER: _ClassVar[int]
    BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    THUMBNAIL_URL_FIELD_NUMBER: _ClassVar[int]
    kind: TrashKind
    id: int
    video_id: int
    video_name: str
    clip_index: int
    deleted_at: _timestamp_pb2.Timestamp
    deleted_by: _stats_pb2.Profile
    purge_at: _timestamp_pb2.Timestamp
    size_bytes: int
    frame_count: int
    clip_count: int
    labeled_frame_count: int
    track_count: int
    box_count: int
    thumbnail_url: str
    def __init__(self, kind: _Optional[_Union[TrashKind, str]] = ..., id: _Optional[int] = ..., video_id: _Optional[int] = ..., video_name: _Optional[str] = ..., clip_index: _Optional[int] = ..., deleted_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., deleted_by: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., purge_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., size_bytes: _Optional[int] = ..., frame_count: _Optional[int] = ..., clip_count: _Optional[int] = ..., labeled_frame_count: _Optional[int] = ..., track_count: _Optional[int] = ..., box_count: _Optional[int] = ..., thumbnail_url: _Optional[str] = ...) -> None: ...

class ListTrashRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListTrashResponse(_message.Message):
    __slots__ = ("items", "retention_days")
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    RETENTION_DAYS_FIELD_NUMBER: _ClassVar[int]
    items: _containers.RepeatedCompositeFieldContainer[TrashItem]
    retention_days: int
    def __init__(self, items: _Optional[_Iterable[_Union[TrashItem, _Mapping]]] = ..., retention_days: _Optional[int] = ...) -> None: ...

class RestoreTrashRequest(_message.Message):
    __slots__ = ("video_ids", "clip_ids")
    VIDEO_IDS_FIELD_NUMBER: _ClassVar[int]
    CLIP_IDS_FIELD_NUMBER: _ClassVar[int]
    video_ids: _containers.RepeatedScalarFieldContainer[int]
    clip_ids: _containers.RepeatedScalarFieldContainer[int]
    def __init__(self, video_ids: _Optional[_Iterable[int]] = ..., clip_ids: _Optional[_Iterable[int]] = ...) -> None: ...

class RestoreTrashResponse(_message.Message):
    __slots__ = ("videos", "clips")
    VIDEOS_FIELD_NUMBER: _ClassVar[int]
    CLIPS_FIELD_NUMBER: _ClassVar[int]
    videos: int
    clips: int
    def __init__(self, videos: _Optional[int] = ..., clips: _Optional[int] = ...) -> None: ...

class PurgeTrashRequest(_message.Message):
    __slots__ = ("video_ids", "clip_ids")
    VIDEO_IDS_FIELD_NUMBER: _ClassVar[int]
    CLIP_IDS_FIELD_NUMBER: _ClassVar[int]
    video_ids: _containers.RepeatedScalarFieldContainer[int]
    clip_ids: _containers.RepeatedScalarFieldContainer[int]
    def __init__(self, video_ids: _Optional[_Iterable[int]] = ..., clip_ids: _Optional[_Iterable[int]] = ...) -> None: ...

class PurgeTrashResponse(_message.Message):
    __slots__ = ("videos", "clips", "job")
    VIDEOS_FIELD_NUMBER: _ClassVar[int]
    CLIPS_FIELD_NUMBER: _ClassVar[int]
    JOB_FIELD_NUMBER: _ClassVar[int]
    videos: int
    clips: int
    job: ContentJob
    def __init__(self, videos: _Optional[int] = ..., clips: _Optional[int] = ..., job: _Optional[_Union[ContentJob, _Mapping]] = ...) -> None: ...

class EmptyTrashRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class EmptyTrashResponse(_message.Message):
    __slots__ = ("videos", "clips", "job")
    VIDEOS_FIELD_NUMBER: _ClassVar[int]
    CLIPS_FIELD_NUMBER: _ClassVar[int]
    JOB_FIELD_NUMBER: _ClassVar[int]
    videos: int
    clips: int
    job: ContentJob
    def __init__(self, videos: _Optional[int] = ..., clips: _Optional[int] = ..., job: _Optional[_Union[ContentJob, _Mapping]] = ...) -> None: ...

class ContentVideo(_message.Message):
    __slots__ = ("id", "name", "status", "error", "ingest_progress", "extract_fps", "fps", "frame_count", "clip_count", "embedded_clip_count", "current_clip_count", "labeled_frame_count", "box_count", "created_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    INGEST_PROGRESS_FIELD_NUMBER: _ClassVar[int]
    EXTRACT_FPS_FIELD_NUMBER: _ClassVar[int]
    FPS_FIELD_NUMBER: _ClassVar[int]
    FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    CLIP_COUNT_FIELD_NUMBER: _ClassVar[int]
    EMBEDDED_CLIP_COUNT_FIELD_NUMBER: _ClassVar[int]
    CURRENT_CLIP_COUNT_FIELD_NUMBER: _ClassVar[int]
    LABELED_FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    id: int
    name: str
    status: _video_pb2.VideoStatus
    error: str
    ingest_progress: float
    extract_fps: float
    fps: float
    frame_count: int
    clip_count: int
    embedded_clip_count: int
    current_clip_count: int
    labeled_frame_count: int
    box_count: int
    created_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[int] = ..., name: _Optional[str] = ..., status: _Optional[_Union[_video_pb2.VideoStatus, str]] = ..., error: _Optional[str] = ..., ingest_progress: _Optional[float] = ..., extract_fps: _Optional[float] = ..., fps: _Optional[float] = ..., frame_count: _Optional[int] = ..., clip_count: _Optional[int] = ..., embedded_clip_count: _Optional[int] = ..., current_clip_count: _Optional[int] = ..., labeled_frame_count: _Optional[int] = ..., box_count: _Optional[int] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListContentVideosRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListContentVideosResponse(_message.Message):
    __slots__ = ("videos", "current_model")
    VIDEOS_FIELD_NUMBER: _ClassVar[int]
    CURRENT_MODEL_FIELD_NUMBER: _ClassVar[int]
    videos: _containers.RepeatedCompositeFieldContainer[ContentVideo]
    current_model: str
    def __init__(self, videos: _Optional[_Iterable[_Union[ContentVideo, _Mapping]]] = ..., current_model: _Optional[str] = ...) -> None: ...

class ContentClip(_message.Message):
    __slots__ = ("id", "video_id", "video_name", "index", "frame_count", "labeled_frame_count", "box_count", "claimant", "claim_active_at", "claim_active", "embed_model")
    ID_FIELD_NUMBER: _ClassVar[int]
    VIDEO_ID_FIELD_NUMBER: _ClassVar[int]
    VIDEO_NAME_FIELD_NUMBER: _ClassVar[int]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    LABELED_FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    CLAIMANT_FIELD_NUMBER: _ClassVar[int]
    CLAIM_ACTIVE_AT_FIELD_NUMBER: _ClassVar[int]
    CLAIM_ACTIVE_FIELD_NUMBER: _ClassVar[int]
    EMBED_MODEL_FIELD_NUMBER: _ClassVar[int]
    id: int
    video_id: int
    video_name: str
    index: int
    frame_count: int
    labeled_frame_count: int
    box_count: int
    claimant: _stats_pb2.Profile
    claim_active_at: _timestamp_pb2.Timestamp
    claim_active: bool
    embed_model: str
    def __init__(self, id: _Optional[int] = ..., video_id: _Optional[int] = ..., video_name: _Optional[str] = ..., index: _Optional[int] = ..., frame_count: _Optional[int] = ..., labeled_frame_count: _Optional[int] = ..., box_count: _Optional[int] = ..., claimant: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., claim_active_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., claim_active: _Optional[bool] = ..., embed_model: _Optional[str] = ...) -> None: ...

class ListContentClipsRequest(_message.Message):
    __slots__ = ("video_id", "claimant_id")
    VIDEO_ID_FIELD_NUMBER: _ClassVar[int]
    CLAIMANT_ID_FIELD_NUMBER: _ClassVar[int]
    video_id: int
    claimant_id: str
    def __init__(self, video_id: _Optional[int] = ..., claimant_id: _Optional[str] = ...) -> None: ...

class ListContentClipsResponse(_message.Message):
    __slots__ = ("clips", "truncated", "current_model")
    CLIPS_FIELD_NUMBER: _ClassVar[int]
    TRUNCATED_FIELD_NUMBER: _ClassVar[int]
    CURRENT_MODEL_FIELD_NUMBER: _ClassVar[int]
    clips: _containers.RepeatedCompositeFieldContainer[ContentClip]
    truncated: bool
    current_model: str
    def __init__(self, clips: _Optional[_Iterable[_Union[ContentClip, _Mapping]]] = ..., truncated: _Optional[bool] = ..., current_model: _Optional[str] = ...) -> None: ...

class ReextractImpact(_message.Message):
    __slots__ = ("video_id", "name", "status", "extract_fps", "labeled_frame_count", "track_count", "box_count", "gold_frame_count", "open_review_count", "claim_count", "blocked")
    VIDEO_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    EXTRACT_FPS_FIELD_NUMBER: _ClassVar[int]
    LABELED_FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    TRACK_COUNT_FIELD_NUMBER: _ClassVar[int]
    BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    GOLD_FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    OPEN_REVIEW_COUNT_FIELD_NUMBER: _ClassVar[int]
    CLAIM_COUNT_FIELD_NUMBER: _ClassVar[int]
    BLOCKED_FIELD_NUMBER: _ClassVar[int]
    video_id: int
    name: str
    status: _video_pb2.VideoStatus
    extract_fps: float
    labeled_frame_count: int
    track_count: int
    box_count: int
    gold_frame_count: int
    open_review_count: int
    claim_count: int
    blocked: str
    def __init__(self, video_id: _Optional[int] = ..., name: _Optional[str] = ..., status: _Optional[_Union[_video_pb2.VideoStatus, str]] = ..., extract_fps: _Optional[float] = ..., labeled_frame_count: _Optional[int] = ..., track_count: _Optional[int] = ..., box_count: _Optional[int] = ..., gold_frame_count: _Optional[int] = ..., open_review_count: _Optional[int] = ..., claim_count: _Optional[int] = ..., blocked: _Optional[str] = ...) -> None: ...

class PreviewReextractRequest(_message.Message):
    __slots__ = ("video_ids",)
    VIDEO_IDS_FIELD_NUMBER: _ClassVar[int]
    video_ids: _containers.RepeatedScalarFieldContainer[int]
    def __init__(self, video_ids: _Optional[_Iterable[int]] = ...) -> None: ...

class PreviewReextractResponse(_message.Message):
    __slots__ = ("videos",)
    VIDEOS_FIELD_NUMBER: _ClassVar[int]
    videos: _containers.RepeatedCompositeFieldContainer[ReextractImpact]
    def __init__(self, videos: _Optional[_Iterable[_Union[ReextractImpact, _Mapping]]] = ...) -> None: ...

class ReextractVideosRequest(_message.Message):
    __slots__ = ("video_ids", "extract_fps", "discard_labels")
    VIDEO_IDS_FIELD_NUMBER: _ClassVar[int]
    EXTRACT_FPS_FIELD_NUMBER: _ClassVar[int]
    DISCARD_LABELS_FIELD_NUMBER: _ClassVar[int]
    video_ids: _containers.RepeatedScalarFieldContainer[int]
    extract_fps: float
    discard_labels: bool
    def __init__(self, video_ids: _Optional[_Iterable[int]] = ..., extract_fps: _Optional[float] = ..., discard_labels: _Optional[bool] = ...) -> None: ...

class ReextractVideosResponse(_message.Message):
    __slots__ = ("job",)
    JOB_FIELD_NUMBER: _ClassVar[int]
    job: ContentJob
    def __init__(self, job: _Optional[_Union[ContentJob, _Mapping]] = ...) -> None: ...

class RegenerateEmbeddingsRequest(_message.Message):
    __slots__ = ("video_ids", "clip_ids", "only_stale")
    VIDEO_IDS_FIELD_NUMBER: _ClassVar[int]
    CLIP_IDS_FIELD_NUMBER: _ClassVar[int]
    ONLY_STALE_FIELD_NUMBER: _ClassVar[int]
    video_ids: _containers.RepeatedScalarFieldContainer[int]
    clip_ids: _containers.RepeatedScalarFieldContainer[int]
    only_stale: bool
    def __init__(self, video_ids: _Optional[_Iterable[int]] = ..., clip_ids: _Optional[_Iterable[int]] = ..., only_stale: _Optional[bool] = ...) -> None: ...

class RegenerateEmbeddingsResponse(_message.Message):
    __slots__ = ("clips", "job")
    CLIPS_FIELD_NUMBER: _ClassVar[int]
    JOB_FIELD_NUMBER: _ClassVar[int]
    clips: int
    job: ContentJob
    def __init__(self, clips: _Optional[int] = ..., job: _Optional[_Union[ContentJob, _Mapping]] = ...) -> None: ...

class ReassignClipsRequest(_message.Message):
    __slots__ = ("clip_ids", "user_id")
    CLIP_IDS_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    clip_ids: _containers.RepeatedScalarFieldContainer[int]
    user_id: str
    def __init__(self, clip_ids: _Optional[_Iterable[int]] = ..., user_id: _Optional[str] = ...) -> None: ...

class ReassignClipsResponse(_message.Message):
    __slots__ = ("clips",)
    CLIPS_FIELD_NUMBER: _ClassVar[int]
    clips: int
    def __init__(self, clips: _Optional[int] = ...) -> None: ...

class ContentJob(_message.Message):
    __slots__ = ("id", "kind", "summary", "created_by", "created_at", "total", "done", "failed", "running")
    ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    SUMMARY_FIELD_NUMBER: _ClassVar[int]
    CREATED_BY_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    DONE_FIELD_NUMBER: _ClassVar[int]
    FAILED_FIELD_NUMBER: _ClassVar[int]
    RUNNING_FIELD_NUMBER: _ClassVar[int]
    id: int
    kind: ContentJobKind
    summary: str
    created_by: _stats_pb2.Profile
    created_at: _timestamp_pb2.Timestamp
    total: int
    done: int
    failed: int
    running: int
    def __init__(self, id: _Optional[int] = ..., kind: _Optional[_Union[ContentJobKind, str]] = ..., summary: _Optional[str] = ..., created_by: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., total: _Optional[int] = ..., done: _Optional[int] = ..., failed: _Optional[int] = ..., running: _Optional[int] = ...) -> None: ...

class ListContentJobsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListContentJobsResponse(_message.Message):
    __slots__ = ("jobs",)
    JOBS_FIELD_NUMBER: _ClassVar[int]
    jobs: _containers.RepeatedCompositeFieldContainer[ContentJob]
    def __init__(self, jobs: _Optional[_Iterable[_Union[ContentJob, _Mapping]]] = ...) -> None: ...
