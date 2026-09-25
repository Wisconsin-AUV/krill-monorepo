from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class FrameStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    FRAME_STATUS_UNSPECIFIED: _ClassVar[FrameStatus]
    FRAME_STATUS_UNLABELED: _ClassVar[FrameStatus]
    FRAME_STATUS_LABELED: _ClassVar[FrameStatus]
    FRAME_STATUS_EMPTY: _ClassVar[FrameStatus]

class AnnotationSource(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ANNOTATION_SOURCE_UNSPECIFIED: _ClassVar[AnnotationSource]
    ANNOTATION_SOURCE_HUMAN: _ClassVar[AnnotationSource]
    ANNOTATION_SOURCE_SAM: _ClassVar[AnnotationSource]
    ANNOTATION_SOURCE_YOLO: _ClassVar[AnnotationSource]
    ANNOTATION_SOURCE_CV: _ClassVar[AnnotationSource]

class AnnotationStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ANNOTATION_STATUS_UNSPECIFIED: _ClassVar[AnnotationStatus]
    ANNOTATION_STATUS_PROPOSED: _ClassVar[AnnotationStatus]
    ANNOTATION_STATUS_VERIFIED: _ClassVar[AnnotationStatus]
    ANNOTATION_STATUS_REJECTED: _ClassVar[AnnotationStatus]
FRAME_STATUS_UNSPECIFIED: FrameStatus
FRAME_STATUS_UNLABELED: FrameStatus
FRAME_STATUS_LABELED: FrameStatus
FRAME_STATUS_EMPTY: FrameStatus
ANNOTATION_SOURCE_UNSPECIFIED: AnnotationSource
ANNOTATION_SOURCE_HUMAN: AnnotationSource
ANNOTATION_SOURCE_SAM: AnnotationSource
ANNOTATION_SOURCE_YOLO: AnnotationSource
ANNOTATION_SOURCE_CV: AnnotationSource
ANNOTATION_STATUS_UNSPECIFIED: AnnotationStatus
ANNOTATION_STATUS_PROPOSED: AnnotationStatus
ANNOTATION_STATUS_VERIFIED: AnnotationStatus
ANNOTATION_STATUS_REJECTED: AnnotationStatus

class Box(_message.Message):
    __slots__ = ("x", "y", "width", "height")
    X_FIELD_NUMBER: _ClassVar[int]
    Y_FIELD_NUMBER: _ClassVar[int]
    WIDTH_FIELD_NUMBER: _ClassVar[int]
    HEIGHT_FIELD_NUMBER: _ClassVar[int]
    x: float
    y: float
    width: float
    height: float
    def __init__(self, x: _Optional[float] = ..., y: _Optional[float] = ..., width: _Optional[float] = ..., height: _Optional[float] = ...) -> None: ...

class Point(_message.Message):
    __slots__ = ("x", "y")
    X_FIELD_NUMBER: _ClassVar[int]
    Y_FIELD_NUMBER: _ClassVar[int]
    x: float
    y: float
    def __init__(self, x: _Optional[float] = ..., y: _Optional[float] = ...) -> None: ...

class Track(_message.Message):
    __slots__ = ("id", "clip_id", "label_type_id", "attributes")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    CLIP_ID_FIELD_NUMBER: _ClassVar[int]
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    id: int
    clip_id: int
    label_type_id: int
    attributes: _containers.ScalarMap[str, str]
    def __init__(self, id: _Optional[int] = ..., clip_id: _Optional[int] = ..., label_type_id: _Optional[int] = ..., attributes: _Optional[_Mapping[str, str]] = ...) -> None: ...

class Annotation(_message.Message):
    __slots__ = ("id", "track_id", "frame_id", "box", "source", "status")
    ID_FIELD_NUMBER: _ClassVar[int]
    TRACK_ID_FIELD_NUMBER: _ClassVar[int]
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    BOX_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    id: int
    track_id: int
    frame_id: int
    box: Box
    source: AnnotationSource
    status: AnnotationStatus
    def __init__(self, id: _Optional[int] = ..., track_id: _Optional[int] = ..., frame_id: _Optional[int] = ..., box: _Optional[_Union[Box, _Mapping]] = ..., source: _Optional[_Union[AnnotationSource, str]] = ..., status: _Optional[_Union[AnnotationStatus, str]] = ...) -> None: ...

class CreateTrackRequest(_message.Message):
    __slots__ = ("clip_id", "label_type_id", "attributes", "frame_id", "box")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    CLIP_ID_FIELD_NUMBER: _ClassVar[int]
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    BOX_FIELD_NUMBER: _ClassVar[int]
    clip_id: int
    label_type_id: int
    attributes: _containers.ScalarMap[str, str]
    frame_id: int
    box: Box
    def __init__(self, clip_id: _Optional[int] = ..., label_type_id: _Optional[int] = ..., attributes: _Optional[_Mapping[str, str]] = ..., frame_id: _Optional[int] = ..., box: _Optional[_Union[Box, _Mapping]] = ...) -> None: ...

class CreateTrackResponse(_message.Message):
    __slots__ = ("track", "annotation")
    TRACK_FIELD_NUMBER: _ClassVar[int]
    ANNOTATION_FIELD_NUMBER: _ClassVar[int]
    track: Track
    annotation: Annotation
    def __init__(self, track: _Optional[_Union[Track, _Mapping]] = ..., annotation: _Optional[_Union[Annotation, _Mapping]] = ...) -> None: ...

class UpdateTrackRequest(_message.Message):
    __slots__ = ("id", "label_type_id", "attributes", "set_attributes")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    SET_ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    id: int
    label_type_id: int
    attributes: _containers.ScalarMap[str, str]
    set_attributes: bool
    def __init__(self, id: _Optional[int] = ..., label_type_id: _Optional[int] = ..., attributes: _Optional[_Mapping[str, str]] = ..., set_attributes: _Optional[bool] = ...) -> None: ...

class UpdateTrackResponse(_message.Message):
    __slots__ = ("track",)
    TRACK_FIELD_NUMBER: _ClassVar[int]
    track: Track
    def __init__(self, track: _Optional[_Union[Track, _Mapping]] = ...) -> None: ...

class DeleteTrackRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class DeleteTrackResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SetBoxRequest(_message.Message):
    __slots__ = ("track_id", "frame_id", "box")
    TRACK_ID_FIELD_NUMBER: _ClassVar[int]
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    BOX_FIELD_NUMBER: _ClassVar[int]
    track_id: int
    frame_id: int
    box: Box
    def __init__(self, track_id: _Optional[int] = ..., frame_id: _Optional[int] = ..., box: _Optional[_Union[Box, _Mapping]] = ...) -> None: ...

class SetBoxResponse(_message.Message):
    __slots__ = ("annotation",)
    ANNOTATION_FIELD_NUMBER: _ClassVar[int]
    annotation: Annotation
    def __init__(self, annotation: _Optional[_Union[Annotation, _Mapping]] = ...) -> None: ...

class DeleteBoxRequest(_message.Message):
    __slots__ = ("track_id", "frame_id")
    TRACK_ID_FIELD_NUMBER: _ClassVar[int]
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    track_id: int
    frame_id: int
    def __init__(self, track_id: _Optional[int] = ..., frame_id: _Optional[int] = ...) -> None: ...

class DeleteBoxResponse(_message.Message):
    __slots__ = ("track_deleted",)
    TRACK_DELETED_FIELD_NUMBER: _ClassVar[int]
    track_deleted: bool
    def __init__(self, track_deleted: _Optional[bool] = ...) -> None: ...

class CopyBoxesRequest(_message.Message):
    __slots__ = ("from_frame_id", "to_frame_id", "track_ids")
    FROM_FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    TO_FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    TRACK_IDS_FIELD_NUMBER: _ClassVar[int]
    from_frame_id: int
    to_frame_id: int
    track_ids: _containers.RepeatedScalarFieldContainer[int]
    def __init__(self, from_frame_id: _Optional[int] = ..., to_frame_id: _Optional[int] = ..., track_ids: _Optional[_Iterable[int]] = ...) -> None: ...

class CopyBoxesResponse(_message.Message):
    __slots__ = ("annotations",)
    ANNOTATIONS_FIELD_NUMBER: _ClassVar[int]
    annotations: _containers.RepeatedCompositeFieldContainer[Annotation]
    def __init__(self, annotations: _Optional[_Iterable[_Union[Annotation, _Mapping]]] = ...) -> None: ...

class SetFrameStatusRequest(_message.Message):
    __slots__ = ("frame_id", "status")
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    frame_id: int
    status: FrameStatus
    def __init__(self, frame_id: _Optional[int] = ..., status: _Optional[_Union[FrameStatus, str]] = ...) -> None: ...

class SetFrameStatusResponse(_message.Message):
    __slots__ = ("status",)
    STATUS_FIELD_NUMBER: _ClassVar[int]
    status: FrameStatus
    def __init__(self, status: _Optional[_Union[FrameStatus, str]] = ...) -> None: ...

class TrackObjectRequest(_message.Message):
    __slots__ = ("frame_id", "track_id", "label_type_id", "attributes", "point", "box")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    TRACK_ID_FIELD_NUMBER: _ClassVar[int]
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    POINT_FIELD_NUMBER: _ClassVar[int]
    BOX_FIELD_NUMBER: _ClassVar[int]
    frame_id: int
    track_id: int
    label_type_id: int
    attributes: _containers.ScalarMap[str, str]
    point: Point
    box: Box
    def __init__(self, frame_id: _Optional[int] = ..., track_id: _Optional[int] = ..., label_type_id: _Optional[int] = ..., attributes: _Optional[_Mapping[str, str]] = ..., point: _Optional[_Union[Point, _Mapping]] = ..., box: _Optional[_Union[Box, _Mapping]] = ...) -> None: ...

class TrackObjectResponse(_message.Message):
    __slots__ = ("track",)
    TRACK_FIELD_NUMBER: _ClassVar[int]
    track: Track
    def __init__(self, track: _Optional[_Union[Track, _Mapping]] = ...) -> None: ...
