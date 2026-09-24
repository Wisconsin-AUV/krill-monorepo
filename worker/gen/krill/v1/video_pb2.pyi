import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from krill.v1 import stats_pb2 as _stats_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class VideoStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    VIDEO_STATUS_UNSPECIFIED: _ClassVar[VideoStatus]
    VIDEO_STATUS_UPLOADING: _ClassVar[VideoStatus]
    VIDEO_STATUS_QUEUED: _ClassVar[VideoStatus]
    VIDEO_STATUS_PROCESSING: _ClassVar[VideoStatus]
    VIDEO_STATUS_READY: _ClassVar[VideoStatus]
    VIDEO_STATUS_FAILED: _ClassVar[VideoStatus]

class SplitAssignment(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SPLIT_ASSIGNMENT_UNSPECIFIED: _ClassVar[SplitAssignment]
    SPLIT_ASSIGNMENT_AUTO: _ClassVar[SplitAssignment]
    SPLIT_ASSIGNMENT_TRAIN: _ClassVar[SplitAssignment]
    SPLIT_ASSIGNMENT_VAL: _ClassVar[SplitAssignment]
VIDEO_STATUS_UNSPECIFIED: VideoStatus
VIDEO_STATUS_UPLOADING: VideoStatus
VIDEO_STATUS_QUEUED: VideoStatus
VIDEO_STATUS_PROCESSING: VideoStatus
VIDEO_STATUS_READY: VideoStatus
VIDEO_STATUS_FAILED: VideoStatus
SPLIT_ASSIGNMENT_UNSPECIFIED: SplitAssignment
SPLIT_ASSIGNMENT_AUTO: SplitAssignment
SPLIT_ASSIGNMENT_TRAIN: SplitAssignment
SPLIT_ASSIGNMENT_VAL: SplitAssignment

class Video(_message.Message):
    __slots__ = ("id", "name", "filename", "notes", "status", "error", "ingest_progress", "split", "extract_fps", "width", "height", "fps", "duration_ms", "frame_count", "clip_count", "thumbnail_url", "created_at", "labeled_frame_count", "box_count")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    FILENAME_FIELD_NUMBER: _ClassVar[int]
    NOTES_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    INGEST_PROGRESS_FIELD_NUMBER: _ClassVar[int]
    SPLIT_FIELD_NUMBER: _ClassVar[int]
    EXTRACT_FPS_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    FPS_FIELD_NUMBER: _ClassVar[int]
    DURATION_MS_FIELD_NUMBER: _ClassVar[int]
    FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    CLIP_COUNT_FIELD_NUMBER: _ClassVar[int]
    THUMBNAIL_URL_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    LABELED_FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    id: int
    name: str
    filename: str
    notes: str
    status: VideoStatus
    error: str
    ingest_progress: float
    split: SplitAssignment
    extract_fps: float
    width: int
    height: int
    fps: float
    duration_ms: int
    frame_count: int
    clip_count: int
    thumbnail_url: str
    created_at: _timestamp_pb2.Timestamp
    labeled_frame_count: int
    box_count: int
    def __init__(self, id: _Optional[int] = ..., name: _Optional[str] = ..., filename: _Optional[str] = ..., notes: _Optional[str] = ..., status: _Optional[_Union[VideoStatus, str]] = ..., error: _Optional[str] = ..., ingest_progress: _Optional[float] = ..., split: _Optional[_Union[SplitAssignment, str]] = ..., extract_fps: _Optional[float] = ..., width: _Optional[int] = ..., height: _Optional[int] = ..., fps: _Optional[float] = ..., duration_ms: _Optional[int] = ..., frame_count: _Optional[int] = ..., clip_count: _Optional[int] = ..., thumbnail_url: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., labeled_frame_count: _Optional[int] = ..., box_count: _Optional[int] = ...) -> None: ...

class Clip(_message.Message):
    __slots__ = ("id", "video_id", "index", "start_frame", "frame_count", "start_ms", "duration_ms", "thumbnail_url", "labeled_frame_count", "box_count", "claim")
    ID_FIELD_NUMBER: _ClassVar[int]
    VIDEO_ID_FIELD_NUMBER: _ClassVar[int]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    START_FRAME_FIELD_NUMBER: _ClassVar[int]
    FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    START_MS_FIELD_NUMBER: _ClassVar[int]
    DURATION_MS_FIELD_NUMBER: _ClassVar[int]
    THUMBNAIL_URL_FIELD_NUMBER: _ClassVar[int]
    LABELED_FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    CLAIM_FIELD_NUMBER: _ClassVar[int]
    id: int
    video_id: int
    index: int
    start_frame: int
    frame_count: int
    start_ms: int
    duration_ms: int
    thumbnail_url: str
    labeled_frame_count: int
    box_count: int
    claim: ClipClaim
    def __init__(self, id: _Optional[int] = ..., video_id: _Optional[int] = ..., index: _Optional[int] = ..., start_frame: _Optional[int] = ..., frame_count: _Optional[int] = ..., start_ms: _Optional[int] = ..., duration_ms: _Optional[int] = ..., thumbnail_url: _Optional[str] = ..., labeled_frame_count: _Optional[int] = ..., box_count: _Optional[int] = ..., claim: _Optional[_Union[ClipClaim, _Mapping]] = ...) -> None: ...

class ClipClaim(_message.Message):
    __slots__ = ("user", "active_at", "active")
    USER_FIELD_NUMBER: _ClassVar[int]
    ACTIVE_AT_FIELD_NUMBER: _ClassVar[int]
    ACTIVE_FIELD_NUMBER: _ClassVar[int]
    user: _stats_pb2.Profile
    active_at: _timestamp_pb2.Timestamp
    active: bool
    def __init__(self, user: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., active_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., active: _Optional[bool] = ...) -> None: ...

class CreateVideoRequest(_message.Message):
    __slots__ = ("name", "filename", "extract_fps")
    NAME_FIELD_NUMBER: _ClassVar[int]
    FILENAME_FIELD_NUMBER: _ClassVar[int]
    EXTRACT_FPS_FIELD_NUMBER: _ClassVar[int]
    name: str
    filename: str
    extract_fps: float
    def __init__(self, name: _Optional[str] = ..., filename: _Optional[str] = ..., extract_fps: _Optional[float] = ...) -> None: ...

class CreateVideoResponse(_message.Message):
    __slots__ = ("video", "upload_url")
    VIDEO_FIELD_NUMBER: _ClassVar[int]
    UPLOAD_URL_FIELD_NUMBER: _ClassVar[int]
    video: Video
    upload_url: str
    def __init__(self, video: _Optional[_Union[Video, _Mapping]] = ..., upload_url: _Optional[str] = ...) -> None: ...

class StartIngestRequest(_message.Message):
    __slots__ = ("video_id",)
    VIDEO_ID_FIELD_NUMBER: _ClassVar[int]
    video_id: int
    def __init__(self, video_id: _Optional[int] = ...) -> None: ...

class StartIngestResponse(_message.Message):
    __slots__ = ("video",)
    VIDEO_FIELD_NUMBER: _ClassVar[int]
    video: Video
    def __init__(self, video: _Optional[_Union[Video, _Mapping]] = ...) -> None: ...

class ListVideosRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListVideosResponse(_message.Message):
    __slots__ = ("videos",)
    VIDEOS_FIELD_NUMBER: _ClassVar[int]
    videos: _containers.RepeatedCompositeFieldContainer[Video]
    def __init__(self, videos: _Optional[_Iterable[_Union[Video, _Mapping]]] = ...) -> None: ...

class GetVideoRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class GetVideoResponse(_message.Message):
    __slots__ = ("video", "clips")
    VIDEO_FIELD_NUMBER: _ClassVar[int]
    CLIPS_FIELD_NUMBER: _ClassVar[int]
    video: Video
    clips: _containers.RepeatedCompositeFieldContainer[Clip]
    def __init__(self, video: _Optional[_Union[Video, _Mapping]] = ..., clips: _Optional[_Iterable[_Union[Clip, _Mapping]]] = ...) -> None: ...

class UpdateVideoRequest(_message.Message):
    __slots__ = ("id", "name", "notes", "split")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    NOTES_FIELD_NUMBER: _ClassVar[int]
    SPLIT_FIELD_NUMBER: _ClassVar[int]
    id: int
    name: str
    notes: str
    split: SplitAssignment
    def __init__(self, id: _Optional[int] = ..., name: _Optional[str] = ..., notes: _Optional[str] = ..., split: _Optional[_Union[SplitAssignment, str]] = ...) -> None: ...

class UpdateVideoResponse(_message.Message):
    __slots__ = ("video",)
    VIDEO_FIELD_NUMBER: _ClassVar[int]
    video: Video
    def __init__(self, video: _Optional[_Union[Video, _Mapping]] = ...) -> None: ...

class DeleteVideoRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class DeleteVideoResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
