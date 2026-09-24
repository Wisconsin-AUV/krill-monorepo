from krill.v1 import video_pb2 as _video_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetQueueRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class OpenClip(_message.Message):
    __slots__ = ("clip", "video_name")
    CLIP_FIELD_NUMBER: _ClassVar[int]
    VIDEO_NAME_FIELD_NUMBER: _ClassVar[int]
    clip: _video_pb2.Clip
    video_name: str
    def __init__(self, clip: _Optional[_Union[_video_pb2.Clip, _Mapping]] = ..., video_name: _Optional[str] = ...) -> None: ...

class ClipProgress(_message.Message):
    __slots__ = ("id", "index", "frame_count", "labeled_frame_count", "claim")
    ID_FIELD_NUMBER: _ClassVar[int]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    LABELED_FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    CLAIM_FIELD_NUMBER: _ClassVar[int]
    id: int
    index: int
    frame_count: int
    labeled_frame_count: int
    claim: _video_pb2.ClipClaim
    def __init__(self, id: _Optional[int] = ..., index: _Optional[int] = ..., frame_count: _Optional[int] = ..., labeled_frame_count: _Optional[int] = ..., claim: _Optional[_Union[_video_pb2.ClipClaim, _Mapping]] = ...) -> None: ...

class VideoProgress(_message.Message):
    __slots__ = ("id", "name", "clips")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    CLIPS_FIELD_NUMBER: _ClassVar[int]
    id: int
    name: str
    clips: _containers.RepeatedCompositeFieldContainer[ClipProgress]
    def __init__(self, id: _Optional[int] = ..., name: _Optional[str] = ..., clips: _Optional[_Iterable[_Union[ClipProgress, _Mapping]]] = ...) -> None: ...

class GetQueueResponse(_message.Message):
    __slots__ = ("open_clips", "videos", "available_clips")
    OPEN_CLIPS_FIELD_NUMBER: _ClassVar[int]
    VIDEOS_FIELD_NUMBER: _ClassVar[int]
    AVAILABLE_CLIPS_FIELD_NUMBER: _ClassVar[int]
    open_clips: _containers.RepeatedCompositeFieldContainer[OpenClip]
    videos: _containers.RepeatedCompositeFieldContainer[VideoProgress]
    available_clips: int
    def __init__(self, open_clips: _Optional[_Iterable[_Union[OpenClip, _Mapping]]] = ..., videos: _Optional[_Iterable[_Union[VideoProgress, _Mapping]]] = ..., available_clips: _Optional[int] = ...) -> None: ...

class ClaimNextClipRequest(_message.Message):
    __slots__ = ("release_clip_id",)
    RELEASE_CLIP_ID_FIELD_NUMBER: _ClassVar[int]
    release_clip_id: int
    def __init__(self, release_clip_id: _Optional[int] = ...) -> None: ...

class ClaimNextClipResponse(_message.Message):
    __slots__ = ("clip_id",)
    CLIP_ID_FIELD_NUMBER: _ClassVar[int]
    clip_id: int
    def __init__(self, clip_id: _Optional[int] = ...) -> None: ...
