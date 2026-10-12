import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from krill.v1 import annotation_pb2 as _annotation_pb2
from krill.v1 import stats_pb2 as _stats_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GoldBox(_message.Message):
    __slots__ = ("label_type_id", "attributes", "box")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    BOX_FIELD_NUMBER: _ClassVar[int]
    label_type_id: int
    attributes: _containers.ScalarMap[str, str]
    box: _annotation_pb2.Box
    def __init__(self, label_type_id: _Optional[int] = ..., attributes: _Optional[_Mapping[str, str]] = ..., box: _Optional[_Union[_annotation_pb2.Box, _Mapping]] = ...) -> None: ...

class GoldMatch(_message.Message):
    __slots__ = ("reference", "answer", "iou", "correct")
    REFERENCE_FIELD_NUMBER: _ClassVar[int]
    ANSWER_FIELD_NUMBER: _ClassVar[int]
    IOU_FIELD_NUMBER: _ClassVar[int]
    CORRECT_FIELD_NUMBER: _ClassVar[int]
    reference: int
    answer: int
    iou: float
    correct: bool
    def __init__(self, reference: _Optional[int] = ..., answer: _Optional[int] = ..., iou: _Optional[float] = ..., correct: _Optional[bool] = ...) -> None: ...

class GoldResult(_message.Message):
    __slots__ = ("reference", "answer", "matches", "score")
    REFERENCE_FIELD_NUMBER: _ClassVar[int]
    ANSWER_FIELD_NUMBER: _ClassVar[int]
    MATCHES_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    reference: _containers.RepeatedCompositeFieldContainer[GoldBox]
    answer: _containers.RepeatedCompositeFieldContainer[GoldBox]
    matches: _containers.RepeatedCompositeFieldContainer[GoldMatch]
    score: float
    def __init__(self, reference: _Optional[_Iterable[_Union[GoldBox, _Mapping]]] = ..., answer: _Optional[_Iterable[_Union[GoldBox, _Mapping]]] = ..., matches: _Optional[_Iterable[_Union[GoldMatch, _Mapping]]] = ..., score: _Optional[float] = ...) -> None: ...

class SetGoldFrameRequest(_message.Message):
    __slots__ = ("frame_id", "gold")
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    GOLD_FIELD_NUMBER: _ClassVar[int]
    frame_id: int
    gold: bool
    def __init__(self, frame_id: _Optional[int] = ..., gold: _Optional[bool] = ...) -> None: ...

class SetGoldFrameResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetGoldCheckRequest(_message.Message):
    __slots__ = ("frame_id",)
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    frame_id: int
    def __init__(self, frame_id: _Optional[int] = ...) -> None: ...

class GetGoldCheckResponse(_message.Message):
    __slots__ = ("frame_id", "url", "width", "height")
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    frame_id: int
    url: str
    width: int
    height: int
    def __init__(self, frame_id: _Optional[int] = ..., url: _Optional[str] = ..., width: _Optional[int] = ..., height: _Optional[int] = ...) -> None: ...

class SubmitGoldCheckRequest(_message.Message):
    __slots__ = ("frame_id", "boxes")
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    BOXES_FIELD_NUMBER: _ClassVar[int]
    frame_id: int
    boxes: _containers.RepeatedCompositeFieldContainer[GoldBox]
    def __init__(self, frame_id: _Optional[int] = ..., boxes: _Optional[_Iterable[_Union[GoldBox, _Mapping]]] = ...) -> None: ...

class SubmitGoldCheckResponse(_message.Message):
    __slots__ = ("result",)
    RESULT_FIELD_NUMBER: _ClassVar[int]
    result: GoldResult
    def __init__(self, result: _Optional[_Union[GoldResult, _Mapping]] = ...) -> None: ...

class GetGoldStatsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GoldLabeler(_message.Message):
    __slots__ = ("user", "attempts", "accuracy", "mean_iou", "needs_review")
    USER_FIELD_NUMBER: _ClassVar[int]
    ATTEMPTS_FIELD_NUMBER: _ClassVar[int]
    ACCURACY_FIELD_NUMBER: _ClassVar[int]
    MEAN_IOU_FIELD_NUMBER: _ClassVar[int]
    NEEDS_REVIEW_FIELD_NUMBER: _ClassVar[int]
    user: _stats_pb2.Profile
    attempts: int
    accuracy: float
    mean_iou: float
    needs_review: bool
    def __init__(self, user: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., attempts: _Optional[int] = ..., accuracy: _Optional[float] = ..., mean_iou: _Optional[float] = ..., needs_review: _Optional[bool] = ...) -> None: ...

class GoldFrame(_message.Message):
    __slots__ = ("frame_id", "clip_id", "index", "video_name", "thumbnail_url", "box_count", "attempts", "mean_score")
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    CLIP_ID_FIELD_NUMBER: _ClassVar[int]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    VIDEO_NAME_FIELD_NUMBER: _ClassVar[int]
    THUMBNAIL_URL_FIELD_NUMBER: _ClassVar[int]
    BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    ATTEMPTS_FIELD_NUMBER: _ClassVar[int]
    MEAN_SCORE_FIELD_NUMBER: _ClassVar[int]
    frame_id: int
    clip_id: int
    index: int
    video_name: str
    thumbnail_url: str
    box_count: int
    attempts: int
    mean_score: float
    def __init__(self, frame_id: _Optional[int] = ..., clip_id: _Optional[int] = ..., index: _Optional[int] = ..., video_name: _Optional[str] = ..., thumbnail_url: _Optional[str] = ..., box_count: _Optional[int] = ..., attempts: _Optional[int] = ..., mean_score: _Optional[float] = ...) -> None: ...

class GetGoldStatsResponse(_message.Message):
    __slots__ = ("labelers", "frames")
    LABELERS_FIELD_NUMBER: _ClassVar[int]
    FRAMES_FIELD_NUMBER: _ClassVar[int]
    labelers: _containers.RepeatedCompositeFieldContainer[GoldLabeler]
    frames: _containers.RepeatedCompositeFieldContainer[GoldFrame]
    def __init__(self, labelers: _Optional[_Iterable[_Union[GoldLabeler, _Mapping]]] = ..., frames: _Optional[_Iterable[_Union[GoldFrame, _Mapping]]] = ...) -> None: ...

class GoldAttempt(_message.Message):
    __slots__ = ("frame_id", "user", "clip_id", "index", "video_name", "score", "mean_iou", "created_at")
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    CLIP_ID_FIELD_NUMBER: _ClassVar[int]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    VIDEO_NAME_FIELD_NUMBER: _ClassVar[int]
    SCORE_FIELD_NUMBER: _ClassVar[int]
    MEAN_IOU_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    frame_id: int
    user: _stats_pb2.Profile
    clip_id: int
    index: int
    video_name: str
    score: float
    mean_iou: float
    created_at: _timestamp_pb2.Timestamp
    def __init__(self, frame_id: _Optional[int] = ..., user: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., clip_id: _Optional[int] = ..., index: _Optional[int] = ..., video_name: _Optional[str] = ..., score: _Optional[float] = ..., mean_iou: _Optional[float] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class GetGoldAttemptRequest(_message.Message):
    __slots__ = ("frame_id", "user_id")
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    frame_id: int
    user_id: str
    def __init__(self, frame_id: _Optional[int] = ..., user_id: _Optional[str] = ...) -> None: ...

class GetGoldAttemptResponse(_message.Message):
    __slots__ = ("attempt", "result", "url", "width", "height")
    ATTEMPT_FIELD_NUMBER: _ClassVar[int]
    RESULT_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    attempt: GoldAttempt
    result: GoldResult
    url: str
    width: int
    height: int
    def __init__(self, attempt: _Optional[_Union[GoldAttempt, _Mapping]] = ..., result: _Optional[_Union[GoldResult, _Mapping]] = ..., url: _Optional[str] = ..., width: _Optional[int] = ..., height: _Optional[int] = ...) -> None: ...

class ListGoldAttemptsRequest(_message.Message):
    __slots__ = ("user_id", "frame_id", "page_size", "page_token")
    USER_ID_FIELD_NUMBER: _ClassVar[int]
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    PAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    user_id: str
    frame_id: int
    page_size: int
    page_token: str
    def __init__(self, user_id: _Optional[str] = ..., frame_id: _Optional[int] = ..., page_size: _Optional[int] = ..., page_token: _Optional[str] = ...) -> None: ...

class ListGoldAttemptsResponse(_message.Message):
    __slots__ = ("attempts", "next_page_token")
    ATTEMPTS_FIELD_NUMBER: _ClassVar[int]
    NEXT_PAGE_TOKEN_FIELD_NUMBER: _ClassVar[int]
    attempts: _containers.RepeatedCompositeFieldContainer[GoldAttempt]
    next_page_token: str
    def __init__(self, attempts: _Optional[_Iterable[_Union[GoldAttempt, _Mapping]]] = ..., next_page_token: _Optional[str] = ...) -> None: ...
