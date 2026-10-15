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

class TrendBucket(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    TREND_BUCKET_UNSPECIFIED: _ClassVar[TrendBucket]
    TREND_BUCKET_HOUR: _ClassVar[TrendBucket]
    TREND_BUCKET_DAY: _ClassVar[TrendBucket]
    TREND_BUCKET_WEEK: _ClassVar[TrendBucket]
TREND_BUCKET_UNSPECIFIED: TrendBucket
TREND_BUCKET_HOUR: TrendBucket
TREND_BUCKET_DAY: TrendBucket
TREND_BUCKET_WEEK: TrendBucket

class GetLabelerStatsRequest(_message.Message):
    __slots__ = ("period", "time_zone")
    PERIOD_FIELD_NUMBER: _ClassVar[int]
    TIME_ZONE_FIELD_NUMBER: _ClassVar[int]
    period: _stats_pb2.Period
    time_zone: str
    def __init__(self, period: _Optional[_Union[_stats_pb2.Period, str]] = ..., time_zone: _Optional[str] = ...) -> None: ...

class ProposalCounts(_message.Message):
    __slots__ = ("source", "accepted", "edited", "rejected")
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    ACCEPTED_FIELD_NUMBER: _ClassVar[int]
    EDITED_FIELD_NUMBER: _ClassVar[int]
    REJECTED_FIELD_NUMBER: _ClassVar[int]
    source: str
    accepted: int
    edited: int
    rejected: int
    def __init__(self, source: _Optional[str] = ..., accepted: _Optional[int] = ..., edited: _Optional[int] = ..., rejected: _Optional[int] = ...) -> None: ...

class LabelerStats(_message.Message):
    __slots__ = ("user", "frames_labeled", "frames_empty", "boxes_drawn", "proposals", "frames_marked", "frames_corrected", "boxes_corrected", "active_seconds")
    USER_FIELD_NUMBER: _ClassVar[int]
    FRAMES_LABELED_FIELD_NUMBER: _ClassVar[int]
    FRAMES_EMPTY_FIELD_NUMBER: _ClassVar[int]
    BOXES_DRAWN_FIELD_NUMBER: _ClassVar[int]
    PROPOSALS_FIELD_NUMBER: _ClassVar[int]
    FRAMES_MARKED_FIELD_NUMBER: _ClassVar[int]
    FRAMES_CORRECTED_FIELD_NUMBER: _ClassVar[int]
    BOXES_CORRECTED_FIELD_NUMBER: _ClassVar[int]
    ACTIVE_SECONDS_FIELD_NUMBER: _ClassVar[int]
    user: _stats_pb2.Profile
    frames_labeled: int
    frames_empty: int
    boxes_drawn: int
    proposals: _containers.RepeatedCompositeFieldContainer[ProposalCounts]
    frames_marked: int
    frames_corrected: int
    boxes_corrected: int
    active_seconds: float
    def __init__(self, user: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., frames_labeled: _Optional[int] = ..., frames_empty: _Optional[int] = ..., boxes_drawn: _Optional[int] = ..., proposals: _Optional[_Iterable[_Union[ProposalCounts, _Mapping]]] = ..., frames_marked: _Optional[int] = ..., frames_corrected: _Optional[int] = ..., boxes_corrected: _Optional[int] = ..., active_seconds: _Optional[float] = ...) -> None: ...

class GetLabelerStatsResponse(_message.Message):
    __slots__ = ("labelers", "since")
    LABELERS_FIELD_NUMBER: _ClassVar[int]
    SINCE_FIELD_NUMBER: _ClassVar[int]
    labelers: _containers.RepeatedCompositeFieldContainer[LabelerStats]
    since: _timestamp_pb2.Timestamp
    def __init__(self, labelers: _Optional[_Iterable[_Union[LabelerStats, _Mapping]]] = ..., since: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetLabelerTrendRequest(_message.Message):
    __slots__ = ("user_id", "period", "time_zone")
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    PERIOD_FIELD_NUMBER: _ClassVar[int]
    TIME_ZONE_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    period: _stats_pb2.Period
    time_zone: str
    def __init__(self, user_id: _Optional[str] = ..., period: _Optional[_Union[_stats_pb2.Period, str]] = ..., time_zone: _Optional[str] = ...) -> None: ...

class TrendPoint(_message.Message):
    __slots__ = ("start", "frames", "boxes_drawn", "proposals_kept", "frames_corrected", "active_seconds")
    START_FIELD_NUMBER: _ClassVar[int]
    FRAMES_FIELD_NUMBER: _ClassVar[int]
    BOXES_DRAWN_FIELD_NUMBER: _ClassVar[int]
    PROPOSALS_KEPT_FIELD_NUMBER: _ClassVar[int]
    FRAMES_CORRECTED_FIELD_NUMBER: _ClassVar[int]
    ACTIVE_SECONDS_FIELD_NUMBER: _ClassVar[int]
    start: _timestamp_pb2.Timestamp
    frames: int
    boxes_drawn: int
    proposals_kept: int
    frames_corrected: int
    active_seconds: float
    def __init__(self, start: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., frames: _Optional[int] = ..., boxes_drawn: _Optional[int] = ..., proposals_kept: _Optional[int] = ..., frames_corrected: _Optional[int] = ..., active_seconds: _Optional[float] = ...) -> None: ...

class CorrectionCount(_message.Message):
    __slots__ = ("kind", "boxes")
    KIND_FIELD_NUMBER: _ClassVar[int]
    BOXES_FIELD_NUMBER: _ClassVar[int]
    kind: str
    boxes: int
    def __init__(self, kind: _Optional[str] = ..., boxes: _Optional[int] = ...) -> None: ...

class GetLabelerTrendResponse(_message.Message):
    __slots__ = ("user", "bucket", "points", "corrections")
    USER_FIELD_NUMBER: _ClassVar[int]
    BUCKET_FIELD_NUMBER: _ClassVar[int]
    POINTS_FIELD_NUMBER: _ClassVar[int]
    CORRECTIONS_FIELD_NUMBER: _ClassVar[int]
    user: _stats_pb2.Profile
    bucket: TrendBucket
    points: _containers.RepeatedCompositeFieldContainer[TrendPoint]
    corrections: _containers.RepeatedCompositeFieldContainer[CorrectionCount]
    def __init__(self, user: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., bucket: _Optional[_Union[TrendBucket, str]] = ..., points: _Optional[_Iterable[_Union[TrendPoint, _Mapping]]] = ..., corrections: _Optional[_Iterable[_Union[CorrectionCount, _Mapping]]] = ...) -> None: ...

class GetCoverageRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CoverageVideo(_message.Message):
    __slots__ = ("id", "name", "split", "held_out", "frames", "labeled", "empty", "unlabeled")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    SPLIT_FIELD_NUMBER: _ClassVar[int]
    HELD_OUT_FIELD_NUMBER: _ClassVar[int]
    FRAMES_FIELD_NUMBER: _ClassVar[int]
    LABELED_FIELD_NUMBER: _ClassVar[int]
    EMPTY_FIELD_NUMBER: _ClassVar[int]
    UNLABELED_FIELD_NUMBER: _ClassVar[int]
    id: int
    name: str
    split: str
    held_out: bool
    frames: int
    labeled: int
    empty: int
    unlabeled: int
    def __init__(self, id: _Optional[int] = ..., name: _Optional[str] = ..., split: _Optional[str] = ..., held_out: _Optional[bool] = ..., frames: _Optional[int] = ..., labeled: _Optional[int] = ..., empty: _Optional[int] = ..., unlabeled: _Optional[int] = ...) -> None: ...

class CoverageCell(_message.Message):
    __slots__ = ("video_id", "label_type_id", "verified", "proposed", "frames")
    VIDEO_ID_FIELD_NUMBER: _ClassVar[int]
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    VERIFIED_FIELD_NUMBER: _ClassVar[int]
    PROPOSED_FIELD_NUMBER: _ClassVar[int]
    FRAMES_FIELD_NUMBER: _ClassVar[int]
    video_id: int
    label_type_id: int
    verified: int
    proposed: int
    frames: int
    def __init__(self, video_id: _Optional[int] = ..., label_type_id: _Optional[int] = ..., verified: _Optional[int] = ..., proposed: _Optional[int] = ..., frames: _Optional[int] = ...) -> None: ...

class CoverageLabelType(_message.Message):
    __slots__ = ("id", "name", "color", "verified", "proposed", "frames", "videos", "held_out_verified", "held_out_videos", "few_boxes", "few_videos")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    COLOR_FIELD_NUMBER: _ClassVar[int]
    VERIFIED_FIELD_NUMBER: _ClassVar[int]
    PROPOSED_FIELD_NUMBER: _ClassVar[int]
    FRAMES_FIELD_NUMBER: _ClassVar[int]
    VIDEOS_FIELD_NUMBER: _ClassVar[int]
    HELD_OUT_VERIFIED_FIELD_NUMBER: _ClassVar[int]
    HELD_OUT_VIDEOS_FIELD_NUMBER: _ClassVar[int]
    FEW_BOXES_FIELD_NUMBER: _ClassVar[int]
    FEW_VIDEOS_FIELD_NUMBER: _ClassVar[int]
    id: int
    name: str
    color: str
    verified: int
    proposed: int
    frames: int
    videos: int
    held_out_verified: int
    held_out_videos: int
    few_boxes: bool
    few_videos: bool
    def __init__(self, id: _Optional[int] = ..., name: _Optional[str] = ..., color: _Optional[str] = ..., verified: _Optional[int] = ..., proposed: _Optional[int] = ..., frames: _Optional[int] = ..., videos: _Optional[int] = ..., held_out_verified: _Optional[int] = ..., held_out_videos: _Optional[int] = ..., few_boxes: _Optional[bool] = ..., few_videos: _Optional[bool] = ...) -> None: ...

class GetCoverageResponse(_message.Message):
    __slots__ = ("label_types", "videos", "cells", "min_boxes", "min_videos")
    LABEL_TYPES_FIELD_NUMBER: _ClassVar[int]
    VIDEOS_FIELD_NUMBER: _ClassVar[int]
    CELLS_FIELD_NUMBER: _ClassVar[int]
    MIN_BOXES_FIELD_NUMBER: _ClassVar[int]
    MIN_VIDEOS_FIELD_NUMBER: _ClassVar[int]
    label_types: _containers.RepeatedCompositeFieldContainer[CoverageLabelType]
    videos: _containers.RepeatedCompositeFieldContainer[CoverageVideo]
    cells: _containers.RepeatedCompositeFieldContainer[CoverageCell]
    min_boxes: int
    min_videos: int
    def __init__(self, label_types: _Optional[_Iterable[_Union[CoverageLabelType, _Mapping]]] = ..., videos: _Optional[_Iterable[_Union[CoverageVideo, _Mapping]]] = ..., cells: _Optional[_Iterable[_Union[CoverageCell, _Mapping]]] = ..., min_boxes: _Optional[int] = ..., min_videos: _Optional[int] = ...) -> None: ...

class ListStuckClipsRequest(_message.Message):
    __slots__ = ("idle_days",)
    IDLE_DAYS_FIELD_NUMBER: _ClassVar[int]
    idle_days: int
    def __init__(self, idle_days: _Optional[int] = ...) -> None: ...

class StuckClip(_message.Message):
    __slots__ = ("clip_id", "clip_index", "video_id", "video_name", "claimant", "active_at", "frame_count", "labeled_frames", "boxes")
    CLIP_ID_FIELD_NUMBER: _ClassVar[int]
    CLIP_INDEX_FIELD_NUMBER: _ClassVar[int]
    VIDEO_ID_FIELD_NUMBER: _ClassVar[int]
    VIDEO_NAME_FIELD_NUMBER: _ClassVar[int]
    CLAIMANT_FIELD_NUMBER: _ClassVar[int]
    ACTIVE_AT_FIELD_NUMBER: _ClassVar[int]
    FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    LABELED_FRAMES_FIELD_NUMBER: _ClassVar[int]
    BOXES_FIELD_NUMBER: _ClassVar[int]
    clip_id: int
    clip_index: int
    video_id: int
    video_name: str
    claimant: _stats_pb2.Profile
    active_at: _timestamp_pb2.Timestamp
    frame_count: int
    labeled_frames: int
    boxes: int
    def __init__(self, clip_id: _Optional[int] = ..., clip_index: _Optional[int] = ..., video_id: _Optional[int] = ..., video_name: _Optional[str] = ..., claimant: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., active_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., frame_count: _Optional[int] = ..., labeled_frames: _Optional[int] = ..., boxes: _Optional[int] = ...) -> None: ...

class ListStuckClipsResponse(_message.Message):
    __slots__ = ("clips", "idle_days")
    CLIPS_FIELD_NUMBER: _ClassVar[int]
    IDLE_DAYS_FIELD_NUMBER: _ClassVar[int]
    clips: _containers.RepeatedCompositeFieldContainer[StuckClip]
    idle_days: int
    def __init__(self, clips: _Optional[_Iterable[_Union[StuckClip, _Mapping]]] = ..., idle_days: _Optional[int] = ...) -> None: ...

class ReleaseStuckClipsRequest(_message.Message):
    __slots__ = ("clip_ids", "idle_days")
    CLIP_IDS_FIELD_NUMBER: _ClassVar[int]
    IDLE_DAYS_FIELD_NUMBER: _ClassVar[int]
    clip_ids: _containers.RepeatedScalarFieldContainer[int]
    idle_days: int
    def __init__(self, clip_ids: _Optional[_Iterable[int]] = ..., idle_days: _Optional[int] = ...) -> None: ...

class ReleaseStuckClipsResponse(_message.Message):
    __slots__ = ("released",)
    RELEASED_FIELD_NUMBER: _ClassVar[int]
    released: int
    def __init__(self, released: _Optional[int] = ...) -> None: ...
