from krill.v1 import annotation_pb2 as _annotation_pb2
from krill.v1 import video_pb2 as _video_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Frame(_message.Message):
    __slots__ = ("id", "index", "timestamp_ms", "url", "status")
    ID_FIELD_NUMBER: _ClassVar[int]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    TIMESTAMP_MS_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    id: int
    index: int
    timestamp_ms: int
    url: str
    status: _annotation_pb2.FrameStatus
    def __init__(self, id: _Optional[int] = ..., index: _Optional[int] = ..., timestamp_ms: _Optional[int] = ..., url: _Optional[str] = ..., status: _Optional[_Union[_annotation_pb2.FrameStatus, str]] = ...) -> None: ...

class GetClipRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class GetClipResponse(_message.Message):
    __slots__ = ("video", "clip", "frames", "previous_clip_id", "next_clip_id", "tracks", "annotations")
    VIDEO_FIELD_NUMBER: _ClassVar[int]
    CLIP_FIELD_NUMBER: _ClassVar[int]
    FRAMES_FIELD_NUMBER: _ClassVar[int]
    PREVIOUS_CLIP_ID_FIELD_NUMBER: _ClassVar[int]
    NEXT_CLIP_ID_FIELD_NUMBER: _ClassVar[int]
    TRACKS_FIELD_NUMBER: _ClassVar[int]
    ANNOTATIONS_FIELD_NUMBER: _ClassVar[int]
    video: _video_pb2.Video
    clip: _video_pb2.Clip
    frames: _containers.RepeatedCompositeFieldContainer[Frame]
    previous_clip_id: int
    next_clip_id: int
    tracks: _containers.RepeatedCompositeFieldContainer[_annotation_pb2.Track]
    annotations: _containers.RepeatedCompositeFieldContainer[_annotation_pb2.Annotation]
    def __init__(self, video: _Optional[_Union[_video_pb2.Video, _Mapping]] = ..., clip: _Optional[_Union[_video_pb2.Clip, _Mapping]] = ..., frames: _Optional[_Iterable[_Union[Frame, _Mapping]]] = ..., previous_clip_id: _Optional[int] = ..., next_clip_id: _Optional[int] = ..., tracks: _Optional[_Iterable[_Union[_annotation_pb2.Track, _Mapping]]] = ..., annotations: _Optional[_Iterable[_Union[_annotation_pb2.Annotation, _Mapping]]] = ...) -> None: ...
