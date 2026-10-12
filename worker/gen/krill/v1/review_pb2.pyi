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

class ReviewReason(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    REVIEW_REASON_UNSPECIFIED: _ClassVar[ReviewReason]
    REVIEW_REASON_FLAGGED: _ClassVar[ReviewReason]
    REVIEW_REASON_LOW_ACCURACY: _ClassVar[ReviewReason]
    REVIEW_REASON_DRIFT: _ClassVar[ReviewReason]

class ReviewResolution(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    REVIEW_RESOLUTION_UNSPECIFIED: _ClassVar[ReviewResolution]
    REVIEW_RESOLUTION_ACCEPTED: _ClassVar[ReviewResolution]
    REVIEW_RESOLUTION_FIXED: _ClassVar[ReviewResolution]
    REVIEW_RESOLUTION_DISMISSED: _ClassVar[ReviewResolution]
REVIEW_REASON_UNSPECIFIED: ReviewReason
REVIEW_REASON_FLAGGED: ReviewReason
REVIEW_REASON_LOW_ACCURACY: ReviewReason
REVIEW_REASON_DRIFT: ReviewReason
REVIEW_RESOLUTION_UNSPECIFIED: ReviewResolution
REVIEW_RESOLUTION_ACCEPTED: ReviewResolution
REVIEW_RESOLUTION_FIXED: ReviewResolution
REVIEW_RESOLUTION_DISMISSED: ReviewResolution

class Review(_message.Message):
    __slots__ = ("id", "frame_id", "reason", "created_by", "created_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    REASON_FIELD_NUMBER: _ClassVar[int]
    CREATED_BY_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    id: int
    frame_id: int
    reason: ReviewReason
    created_by: _stats_pb2.Profile
    created_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[int] = ..., frame_id: _Optional[int] = ..., reason: _Optional[_Union[ReviewReason, str]] = ..., created_by: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ReviewItem(_message.Message):
    __slots__ = ("review", "clip_id", "clip_index", "frame_index", "video_name", "thumbnail_url", "labeler")
    REVIEW_FIELD_NUMBER: _ClassVar[int]
    CLIP_ID_FIELD_NUMBER: _ClassVar[int]
    CLIP_INDEX_FIELD_NUMBER: _ClassVar[int]
    FRAME_INDEX_FIELD_NUMBER: _ClassVar[int]
    VIDEO_NAME_FIELD_NUMBER: _ClassVar[int]
    THUMBNAIL_URL_FIELD_NUMBER: _ClassVar[int]
    LABELER_FIELD_NUMBER: _ClassVar[int]
    review: Review
    clip_id: int
    clip_index: int
    frame_index: int
    video_name: str
    thumbnail_url: str
    labeler: _stats_pb2.Profile
    def __init__(self, review: _Optional[_Union[Review, _Mapping]] = ..., clip_id: _Optional[int] = ..., clip_index: _Optional[int] = ..., frame_index: _Optional[int] = ..., video_name: _Optional[str] = ..., thumbnail_url: _Optional[str] = ..., labeler: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ...) -> None: ...

class FlagFrameRequest(_message.Message):
    __slots__ = ("frame_id", "flagged")
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    FLAGGED_FIELD_NUMBER: _ClassVar[int]
    frame_id: int
    flagged: bool
    def __init__(self, frame_id: _Optional[int] = ..., flagged: _Optional[bool] = ...) -> None: ...

class FlagFrameResponse(_message.Message):
    __slots__ = ("review",)
    REVIEW_FIELD_NUMBER: _ClassVar[int]
    review: Review
    def __init__(self, review: _Optional[_Union[Review, _Mapping]] = ...) -> None: ...

class ListReviewsRequest(_message.Message):
    __slots__ = ("reason",)
    REASON_FIELD_NUMBER: _ClassVar[int]
    reason: ReviewReason
    def __init__(self, reason: _Optional[_Union[ReviewReason, str]] = ...) -> None: ...

class ListReviewsResponse(_message.Message):
    __slots__ = ("reviews",)
    REVIEWS_FIELD_NUMBER: _ClassVar[int]
    reviews: _containers.RepeatedCompositeFieldContainer[ReviewItem]
    def __init__(self, reviews: _Optional[_Iterable[_Union[ReviewItem, _Mapping]]] = ...) -> None: ...

class CountReviewsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CountReviewsResponse(_message.Message):
    __slots__ = ("count",)
    COUNT_FIELD_NUMBER: _ClassVar[int]
    count: int
    def __init__(self, count: _Optional[int] = ...) -> None: ...

class ResolveReviewRequest(_message.Message):
    __slots__ = ("id", "resolution")
    ID_FIELD_NUMBER: _ClassVar[int]
    RESOLUTION_FIELD_NUMBER: _ClassVar[int]
    id: int
    resolution: ReviewResolution
    def __init__(self, id: _Optional[int] = ..., resolution: _Optional[_Union[ReviewResolution, str]] = ...) -> None: ...

class ResolveReviewResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
