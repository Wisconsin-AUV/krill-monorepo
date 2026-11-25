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

class KeypointCheckKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    KEYPOINT_CHECK_KIND_UNSPECIFIED: _ClassVar[KeypointCheckKind]
    KEYPOINT_CHECK_KIND_FIT: _ClassVar[KeypointCheckKind]
    KEYPOINT_CHECK_KIND_TRACKING: _ClassVar[KeypointCheckKind]

class KeypointCheckStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    KEYPOINT_CHECK_STATUS_UNSPECIFIED: _ClassVar[KeypointCheckStatus]
    KEYPOINT_CHECK_STATUS_RUNNING: _ClassVar[KeypointCheckStatus]
    KEYPOINT_CHECK_STATUS_SUCCEEDED: _ClassVar[KeypointCheckStatus]
    KEYPOINT_CHECK_STATUS_FAILED: _ClassVar[KeypointCheckStatus]
KEYPOINT_CHECK_KIND_UNSPECIFIED: KeypointCheckKind
KEYPOINT_CHECK_KIND_FIT: KeypointCheckKind
KEYPOINT_CHECK_KIND_TRACKING: KeypointCheckKind
KEYPOINT_CHECK_STATUS_UNSPECIFIED: KeypointCheckStatus
KEYPOINT_CHECK_STATUS_RUNNING: KeypointCheckStatus
KEYPOINT_CHECK_STATUS_SUCCEEDED: KeypointCheckStatus
KEYPOINT_CHECK_STATUS_FAILED: KeypointCheckStatus

class LabelAttribute(_message.Message):
    __slots__ = ("name", "options")
    NAME_FIELD_NUMBER: _ClassVar[int]
    OPTIONS_FIELD_NUMBER: _ClassVar[int]
    name: str
    options: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, name: _Optional[str] = ..., options: _Optional[_Iterable[str]] = ...) -> None: ...

class LabelType(_message.Message):
    __slots__ = ("id", "name", "color", "description", "position", "attributes", "track_count", "box_count", "title", "guideline", "examples", "oriented", "reference_url", "unrotated_box_count", "keypoints", "quads", "edges", "unposed_box_count", "posed_box_count")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    COLOR_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    POSITION_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    TRACK_COUNT_FIELD_NUMBER: _ClassVar[int]
    BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    GUIDELINE_FIELD_NUMBER: _ClassVar[int]
    EXAMPLES_FIELD_NUMBER: _ClassVar[int]
    ORIENTED_FIELD_NUMBER: _ClassVar[int]
    REFERENCE_URL_FIELD_NUMBER: _ClassVar[int]
    UNROTATED_BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    KEYPOINTS_FIELD_NUMBER: _ClassVar[int]
    QUADS_FIELD_NUMBER: _ClassVar[int]
    EDGES_FIELD_NUMBER: _ClassVar[int]
    UNPOSED_BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    POSED_BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    id: int
    name: str
    color: str
    description: str
    position: int
    attributes: _containers.RepeatedCompositeFieldContainer[LabelAttribute]
    track_count: int
    box_count: int
    title: str
    guideline: str
    examples: _containers.RepeatedCompositeFieldContainer[LabelExample]
    oriented: bool
    reference_url: str
    unrotated_box_count: int
    keypoints: _containers.RepeatedCompositeFieldContainer[KeypointDef]
    quads: _containers.RepeatedCompositeFieldContainer[KeypointQuad]
    edges: _containers.RepeatedCompositeFieldContainer[KeypointEdge]
    unposed_box_count: int
    posed_box_count: int
    def __init__(self, id: _Optional[int] = ..., name: _Optional[str] = ..., color: _Optional[str] = ..., description: _Optional[str] = ..., position: _Optional[int] = ..., attributes: _Optional[_Iterable[_Union[LabelAttribute, _Mapping]]] = ..., track_count: _Optional[int] = ..., box_count: _Optional[int] = ..., title: _Optional[str] = ..., guideline: _Optional[str] = ..., examples: _Optional[_Iterable[_Union[LabelExample, _Mapping]]] = ..., oriented: _Optional[bool] = ..., reference_url: _Optional[str] = ..., unrotated_box_count: _Optional[int] = ..., keypoints: _Optional[_Iterable[_Union[KeypointDef, _Mapping]]] = ..., quads: _Optional[_Iterable[_Union[KeypointQuad, _Mapping]]] = ..., edges: _Optional[_Iterable[_Union[KeypointEdge, _Mapping]]] = ..., unposed_box_count: _Optional[int] = ..., posed_box_count: _Optional[int] = ...) -> None: ...

class KeypointDef(_message.Message):
    __slots__ = ("name", "mirror")
    NAME_FIELD_NUMBER: _ClassVar[int]
    MIRROR_FIELD_NUMBER: _ClassVar[int]
    name: str
    mirror: int
    def __init__(self, name: _Optional[str] = ..., mirror: _Optional[int] = ...) -> None: ...

class KeypointQuad(_message.Message):
    __slots__ = ("corners", "reference", "center")
    CORNERS_FIELD_NUMBER: _ClassVar[int]
    REFERENCE_FIELD_NUMBER: _ClassVar[int]
    CENTER_FIELD_NUMBER: _ClassVar[int]
    corners: _containers.RepeatedScalarFieldContainer[int]
    reference: bool
    center: int
    def __init__(self, corners: _Optional[_Iterable[int]] = ..., reference: _Optional[bool] = ..., center: _Optional[int] = ...) -> None: ...

class KeypointEdge(_message.Message):
    __slots__ = ("to",)
    FROM_FIELD_NUMBER: _ClassVar[int]
    TO_FIELD_NUMBER: _ClassVar[int]
    to: int
    def __init__(self, to: _Optional[int] = ..., **kwargs) -> None: ...

class LabelExample(_message.Message):
    __slots__ = ("id", "url", "caption")
    ID_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    CAPTION_FIELD_NUMBER: _ClassVar[int]
    id: int
    url: str
    caption: str
    def __init__(self, id: _Optional[int] = ..., url: _Optional[str] = ..., caption: _Optional[str] = ...) -> None: ...

class ListLabelTypesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListLabelTypesResponse(_message.Message):
    __slots__ = ("label_types",)
    LABEL_TYPES_FIELD_NUMBER: _ClassVar[int]
    label_types: _containers.RepeatedCompositeFieldContainer[LabelType]
    def __init__(self, label_types: _Optional[_Iterable[_Union[LabelType, _Mapping]]] = ...) -> None: ...

class CreateLabelTypeRequest(_message.Message):
    __slots__ = ("name", "color", "description", "attributes", "title", "guideline", "oriented", "keypoints", "quads", "edges")
    NAME_FIELD_NUMBER: _ClassVar[int]
    COLOR_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    GUIDELINE_FIELD_NUMBER: _ClassVar[int]
    ORIENTED_FIELD_NUMBER: _ClassVar[int]
    KEYPOINTS_FIELD_NUMBER: _ClassVar[int]
    QUADS_FIELD_NUMBER: _ClassVar[int]
    EDGES_FIELD_NUMBER: _ClassVar[int]
    name: str
    color: str
    description: str
    attributes: _containers.RepeatedCompositeFieldContainer[LabelAttribute]
    title: str
    guideline: str
    oriented: bool
    keypoints: _containers.RepeatedCompositeFieldContainer[KeypointDef]
    quads: _containers.RepeatedCompositeFieldContainer[KeypointQuad]
    edges: _containers.RepeatedCompositeFieldContainer[KeypointEdge]
    def __init__(self, name: _Optional[str] = ..., color: _Optional[str] = ..., description: _Optional[str] = ..., attributes: _Optional[_Iterable[_Union[LabelAttribute, _Mapping]]] = ..., title: _Optional[str] = ..., guideline: _Optional[str] = ..., oriented: _Optional[bool] = ..., keypoints: _Optional[_Iterable[_Union[KeypointDef, _Mapping]]] = ..., quads: _Optional[_Iterable[_Union[KeypointQuad, _Mapping]]] = ..., edges: _Optional[_Iterable[_Union[KeypointEdge, _Mapping]]] = ...) -> None: ...

class CreateLabelTypeResponse(_message.Message):
    __slots__ = ("label_type",)
    LABEL_TYPE_FIELD_NUMBER: _ClassVar[int]
    label_type: LabelType
    def __init__(self, label_type: _Optional[_Union[LabelType, _Mapping]] = ...) -> None: ...

class UpdateLabelTypeRequest(_message.Message):
    __slots__ = ("id", "name", "color", "description", "attributes", "title", "guideline", "oriented", "keypoints", "quads", "edges", "keypoint_origins")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    COLOR_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    GUIDELINE_FIELD_NUMBER: _ClassVar[int]
    ORIENTED_FIELD_NUMBER: _ClassVar[int]
    KEYPOINTS_FIELD_NUMBER: _ClassVar[int]
    QUADS_FIELD_NUMBER: _ClassVar[int]
    EDGES_FIELD_NUMBER: _ClassVar[int]
    KEYPOINT_ORIGINS_FIELD_NUMBER: _ClassVar[int]
    id: int
    name: str
    color: str
    description: str
    attributes: _containers.RepeatedCompositeFieldContainer[LabelAttribute]
    title: str
    guideline: str
    oriented: bool
    keypoints: _containers.RepeatedCompositeFieldContainer[KeypointDef]
    quads: _containers.RepeatedCompositeFieldContainer[KeypointQuad]
    edges: _containers.RepeatedCompositeFieldContainer[KeypointEdge]
    keypoint_origins: _containers.RepeatedScalarFieldContainer[int]
    def __init__(self, id: _Optional[int] = ..., name: _Optional[str] = ..., color: _Optional[str] = ..., description: _Optional[str] = ..., attributes: _Optional[_Iterable[_Union[LabelAttribute, _Mapping]]] = ..., title: _Optional[str] = ..., guideline: _Optional[str] = ..., oriented: _Optional[bool] = ..., keypoints: _Optional[_Iterable[_Union[KeypointDef, _Mapping]]] = ..., quads: _Optional[_Iterable[_Union[KeypointQuad, _Mapping]]] = ..., edges: _Optional[_Iterable[_Union[KeypointEdge, _Mapping]]] = ..., keypoint_origins: _Optional[_Iterable[int]] = ...) -> None: ...

class UpdateLabelTypeResponse(_message.Message):
    __slots__ = ("label_type",)
    LABEL_TYPE_FIELD_NUMBER: _ClassVar[int]
    label_type: LabelType
    def __init__(self, label_type: _Optional[_Union[LabelType, _Mapping]] = ...) -> None: ...

class DeleteLabelTypeRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class DeleteLabelTypeResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ReorderLabelTypesRequest(_message.Message):
    __slots__ = ("ids",)
    IDS_FIELD_NUMBER: _ClassVar[int]
    ids: _containers.RepeatedScalarFieldContainer[int]
    def __init__(self, ids: _Optional[_Iterable[int]] = ...) -> None: ...

class ReorderLabelTypesResponse(_message.Message):
    __slots__ = ("label_types",)
    LABEL_TYPES_FIELD_NUMBER: _ClassVar[int]
    label_types: _containers.RepeatedCompositeFieldContainer[LabelType]
    def __init__(self, label_types: _Optional[_Iterable[_Union[LabelType, _Mapping]]] = ...) -> None: ...

class CreateLabelExampleUploadRequest(_message.Message):
    __slots__ = ("label_type_id",)
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    label_type_id: int
    def __init__(self, label_type_id: _Optional[int] = ...) -> None: ...

class CreateLabelExampleUploadResponse(_message.Message):
    __slots__ = ("key", "upload_url")
    KEY_FIELD_NUMBER: _ClassVar[int]
    UPLOAD_URL_FIELD_NUMBER: _ClassVar[int]
    key: str
    upload_url: str
    def __init__(self, key: _Optional[str] = ..., upload_url: _Optional[str] = ...) -> None: ...

class AddLabelExampleRequest(_message.Message):
    __slots__ = ("label_type_id", "key", "caption")
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    KEY_FIELD_NUMBER: _ClassVar[int]
    CAPTION_FIELD_NUMBER: _ClassVar[int]
    label_type_id: int
    key: str
    caption: str
    def __init__(self, label_type_id: _Optional[int] = ..., key: _Optional[str] = ..., caption: _Optional[str] = ...) -> None: ...

class AddLabelExampleResponse(_message.Message):
    __slots__ = ("example",)
    EXAMPLE_FIELD_NUMBER: _ClassVar[int]
    example: LabelExample
    def __init__(self, example: _Optional[_Union[LabelExample, _Mapping]] = ...) -> None: ...

class DeleteLabelExampleRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class DeleteLabelExampleResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SetLabelReferenceRequest(_message.Message):
    __slots__ = ("label_type_id", "key")
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    KEY_FIELD_NUMBER: _ClassVar[int]
    label_type_id: int
    key: str
    def __init__(self, label_type_id: _Optional[int] = ..., key: _Optional[str] = ...) -> None: ...

class SetLabelReferenceResponse(_message.Message):
    __slots__ = ("label_type",)
    LABEL_TYPE_FIELD_NUMBER: _ClassVar[int]
    label_type: LabelType
    def __init__(self, label_type: _Optional[_Union[LabelType, _Mapping]] = ...) -> None: ...

class FitRotationsRequest(_message.Message):
    __slots__ = ("label_type_id",)
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    label_type_id: int
    def __init__(self, label_type_id: _Optional[int] = ...) -> None: ...

class FitRotationsResponse(_message.Message):
    __slots__ = ("box_count",)
    BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    box_count: int
    def __init__(self, box_count: _Optional[int] = ...) -> None: ...

class FitKeypointsRequest(_message.Message):
    __slots__ = ("label_type_id",)
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    label_type_id: int
    def __init__(self, label_type_id: _Optional[int] = ...) -> None: ...

class FitKeypointsResponse(_message.Message):
    __slots__ = ("box_count",)
    BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    box_count: int
    def __init__(self, box_count: _Optional[int] = ...) -> None: ...

class CheckKeypointsRequest(_message.Message):
    __slots__ = ("label_type_id", "kind")
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    label_type_id: int
    kind: KeypointCheckKind
    def __init__(self, label_type_id: _Optional[int] = ..., kind: _Optional[_Union[KeypointCheckKind, str]] = ...) -> None: ...

class CheckKeypointsResponse(_message.Message):
    __slots__ = ("box_count",)
    BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    box_count: int
    def __init__(self, box_count: _Optional[int] = ...) -> None: ...

class ListKeypointChecksRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListKeypointChecksResponse(_message.Message):
    __slots__ = ("types", "max_boxes", "max_tracked_frames")
    TYPES_FIELD_NUMBER: _ClassVar[int]
    MAX_BOXES_FIELD_NUMBER: _ClassVar[int]
    MAX_TRACKED_FRAMES_FIELD_NUMBER: _ClassVar[int]
    types: _containers.RepeatedCompositeFieldContainer[KeypointCheckType]
    max_boxes: int
    max_tracked_frames: int
    def __init__(self, types: _Optional[_Iterable[_Union[KeypointCheckType, _Mapping]]] = ..., max_boxes: _Optional[int] = ..., max_tracked_frames: _Optional[int] = ...) -> None: ...

class KeypointCheckType(_message.Message):
    __slots__ = ("label_type_id", "name", "title", "color", "posed_box_count", "last", "running", "posed_track_count", "fittable", "last_tracking", "running_tracking")
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    COLOR_FIELD_NUMBER: _ClassVar[int]
    POSED_BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    LAST_FIELD_NUMBER: _ClassVar[int]
    RUNNING_FIELD_NUMBER: _ClassVar[int]
    POSED_TRACK_COUNT_FIELD_NUMBER: _ClassVar[int]
    FITTABLE_FIELD_NUMBER: _ClassVar[int]
    LAST_TRACKING_FIELD_NUMBER: _ClassVar[int]
    RUNNING_TRACKING_FIELD_NUMBER: _ClassVar[int]
    label_type_id: int
    name: str
    title: str
    color: str
    posed_box_count: int
    last: KeypointCheck
    running: KeypointCheck
    posed_track_count: int
    fittable: bool
    last_tracking: KeypointCheck
    running_tracking: KeypointCheck
    def __init__(self, label_type_id: _Optional[int] = ..., name: _Optional[str] = ..., title: _Optional[str] = ..., color: _Optional[str] = ..., posed_box_count: _Optional[int] = ..., last: _Optional[_Union[KeypointCheck, _Mapping]] = ..., running: _Optional[_Union[KeypointCheck, _Mapping]] = ..., posed_track_count: _Optional[int] = ..., fittable: _Optional[bool] = ..., last_tracking: _Optional[_Union[KeypointCheck, _Mapping]] = ..., running_tracking: _Optional[_Union[KeypointCheck, _Mapping]] = ...) -> None: ...

class KeypointCheck(_message.Message):
    __slots__ = ("id", "status", "error", "created_by", "created_at", "finished_at", "box_count", "found_count", "turned_count", "compared_count", "median_error", "p90_error", "keypoints", "worst", "kind", "track_count", "frame_count", "missed_count", "extra_count")
    ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    CREATED_BY_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    FINISHED_AT_FIELD_NUMBER: _ClassVar[int]
    BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    FOUND_COUNT_FIELD_NUMBER: _ClassVar[int]
    TURNED_COUNT_FIELD_NUMBER: _ClassVar[int]
    COMPARED_COUNT_FIELD_NUMBER: _ClassVar[int]
    MEDIAN_ERROR_FIELD_NUMBER: _ClassVar[int]
    P90_ERROR_FIELD_NUMBER: _ClassVar[int]
    KEYPOINTS_FIELD_NUMBER: _ClassVar[int]
    WORST_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    TRACK_COUNT_FIELD_NUMBER: _ClassVar[int]
    FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    MISSED_COUNT_FIELD_NUMBER: _ClassVar[int]
    EXTRA_COUNT_FIELD_NUMBER: _ClassVar[int]
    id: int
    status: KeypointCheckStatus
    error: str
    created_by: _stats_pb2.Profile
    created_at: _timestamp_pb2.Timestamp
    finished_at: _timestamp_pb2.Timestamp
    box_count: int
    found_count: int
    turned_count: int
    compared_count: int
    median_error: float
    p90_error: float
    keypoints: _containers.RepeatedCompositeFieldContainer[KeypointError]
    worst: _containers.RepeatedCompositeFieldContainer[KeypointCheckBox]
    kind: KeypointCheckKind
    track_count: int
    frame_count: int
    missed_count: int
    extra_count: int
    def __init__(self, id: _Optional[int] = ..., status: _Optional[_Union[KeypointCheckStatus, str]] = ..., error: _Optional[str] = ..., created_by: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., finished_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., box_count: _Optional[int] = ..., found_count: _Optional[int] = ..., turned_count: _Optional[int] = ..., compared_count: _Optional[int] = ..., median_error: _Optional[float] = ..., p90_error: _Optional[float] = ..., keypoints: _Optional[_Iterable[_Union[KeypointError, _Mapping]]] = ..., worst: _Optional[_Iterable[_Union[KeypointCheckBox, _Mapping]]] = ..., kind: _Optional[_Union[KeypointCheckKind, str]] = ..., track_count: _Optional[int] = ..., frame_count: _Optional[int] = ..., missed_count: _Optional[int] = ..., extra_count: _Optional[int] = ...) -> None: ...

class KeypointError(_message.Message):
    __slots__ = ("name", "count", "median", "p90")
    NAME_FIELD_NUMBER: _ClassVar[int]
    COUNT_FIELD_NUMBER: _ClassVar[int]
    MEDIAN_FIELD_NUMBER: _ClassVar[int]
    P90_FIELD_NUMBER: _ClassVar[int]
    name: str
    count: int
    median: float
    p90: float
    def __init__(self, name: _Optional[str] = ..., count: _Optional[int] = ..., median: _Optional[float] = ..., p90: _Optional[float] = ...) -> None: ...

class KeypointCheckBox(_message.Message):
    __slots__ = ("annotation_id", "clip_id", "frame_index", "error", "turned", "video_name", "clip_index", "frames_tracked")
    ANNOTATION_ID_FIELD_NUMBER: _ClassVar[int]
    CLIP_ID_FIELD_NUMBER: _ClassVar[int]
    FRAME_INDEX_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    TURNED_FIELD_NUMBER: _ClassVar[int]
    VIDEO_NAME_FIELD_NUMBER: _ClassVar[int]
    CLIP_INDEX_FIELD_NUMBER: _ClassVar[int]
    FRAMES_TRACKED_FIELD_NUMBER: _ClassVar[int]
    annotation_id: int
    clip_id: int
    frame_index: int
    error: float
    turned: bool
    video_name: str
    clip_index: int
    frames_tracked: int
    def __init__(self, annotation_id: _Optional[int] = ..., clip_id: _Optional[int] = ..., frame_index: _Optional[int] = ..., error: _Optional[float] = ..., turned: _Optional[bool] = ..., video_name: _Optional[str] = ..., clip_index: _Optional[int] = ..., frames_tracked: _Optional[int] = ...) -> None: ...

class AttributeValueMapping(_message.Message):
    __slots__ = ("source_attribute", "source_option", "target_attribute", "target_option", "track_count")
    SOURCE_ATTRIBUTE_FIELD_NUMBER: _ClassVar[int]
    SOURCE_OPTION_FIELD_NUMBER: _ClassVar[int]
    TARGET_ATTRIBUTE_FIELD_NUMBER: _ClassVar[int]
    TARGET_OPTION_FIELD_NUMBER: _ClassVar[int]
    TRACK_COUNT_FIELD_NUMBER: _ClassVar[int]
    source_attribute: str
    source_option: str
    target_attribute: str
    target_option: str
    track_count: int
    def __init__(self, source_attribute: _Optional[str] = ..., source_option: _Optional[str] = ..., target_attribute: _Optional[str] = ..., target_option: _Optional[str] = ..., track_count: _Optional[int] = ...) -> None: ...

class ExportClassChange(_message.Message):
    __slots__ = ("name", "old_id", "new_id", "becomes")
    NAME_FIELD_NUMBER: _ClassVar[int]
    OLD_ID_FIELD_NUMBER: _ClassVar[int]
    NEW_ID_FIELD_NUMBER: _ClassVar[int]
    BECOMES_FIELD_NUMBER: _ClassVar[int]
    name: str
    old_id: int
    new_id: int
    becomes: str
    def __init__(self, name: _Optional[str] = ..., old_id: _Optional[int] = ..., new_id: _Optional[int] = ..., becomes: _Optional[str] = ...) -> None: ...

class PreviewMergeLabelTypesRequest(_message.Message):
    __slots__ = ("source_id", "target_id", "mappings")
    SOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    TARGET_ID_FIELD_NUMBER: _ClassVar[int]
    MAPPINGS_FIELD_NUMBER: _ClassVar[int]
    source_id: int
    target_id: int
    mappings: _containers.RepeatedCompositeFieldContainer[AttributeValueMapping]
    def __init__(self, source_id: _Optional[int] = ..., target_id: _Optional[int] = ..., mappings: _Optional[_Iterable[_Union[AttributeValueMapping, _Mapping]]] = ...) -> None: ...

class PreviewMergeLabelTypesResponse(_message.Message):
    __slots__ = ("track_count", "box_count", "clip_count", "video_count", "trashed_track_count", "gold_frame_count", "gold_attempt_count", "open_review_count", "example_count", "mappings", "class_changes", "unfilled_attributes")
    TRACK_COUNT_FIELD_NUMBER: _ClassVar[int]
    BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    CLIP_COUNT_FIELD_NUMBER: _ClassVar[int]
    VIDEO_COUNT_FIELD_NUMBER: _ClassVar[int]
    TRASHED_TRACK_COUNT_FIELD_NUMBER: _ClassVar[int]
    GOLD_FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    GOLD_ATTEMPT_COUNT_FIELD_NUMBER: _ClassVar[int]
    OPEN_REVIEW_COUNT_FIELD_NUMBER: _ClassVar[int]
    EXAMPLE_COUNT_FIELD_NUMBER: _ClassVar[int]
    MAPPINGS_FIELD_NUMBER: _ClassVar[int]
    CLASS_CHANGES_FIELD_NUMBER: _ClassVar[int]
    UNFILLED_ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    track_count: int
    box_count: int
    clip_count: int
    video_count: int
    trashed_track_count: int
    gold_frame_count: int
    gold_attempt_count: int
    open_review_count: int
    example_count: int
    mappings: _containers.RepeatedCompositeFieldContainer[AttributeValueMapping]
    class_changes: _containers.RepeatedCompositeFieldContainer[ExportClassChange]
    unfilled_attributes: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, track_count: _Optional[int] = ..., box_count: _Optional[int] = ..., clip_count: _Optional[int] = ..., video_count: _Optional[int] = ..., trashed_track_count: _Optional[int] = ..., gold_frame_count: _Optional[int] = ..., gold_attempt_count: _Optional[int] = ..., open_review_count: _Optional[int] = ..., example_count: _Optional[int] = ..., mappings: _Optional[_Iterable[_Union[AttributeValueMapping, _Mapping]]] = ..., class_changes: _Optional[_Iterable[_Union[ExportClassChange, _Mapping]]] = ..., unfilled_attributes: _Optional[_Iterable[str]] = ...) -> None: ...

class MergeLabelTypesRequest(_message.Message):
    __slots__ = ("source_id", "target_id", "mappings")
    SOURCE_ID_FIELD_NUMBER: _ClassVar[int]
    TARGET_ID_FIELD_NUMBER: _ClassVar[int]
    MAPPINGS_FIELD_NUMBER: _ClassVar[int]
    source_id: int
    target_id: int
    mappings: _containers.RepeatedCompositeFieldContainer[AttributeValueMapping]
    def __init__(self, source_id: _Optional[int] = ..., target_id: _Optional[int] = ..., mappings: _Optional[_Iterable[_Union[AttributeValueMapping, _Mapping]]] = ...) -> None: ...

class MergeLabelTypesResponse(_message.Message):
    __slots__ = ("track_count", "box_count", "gold_frame_count", "dropped_value_count")
    TRACK_COUNT_FIELD_NUMBER: _ClassVar[int]
    BOX_COUNT_FIELD_NUMBER: _ClassVar[int]
    GOLD_FRAME_COUNT_FIELD_NUMBER: _ClassVar[int]
    DROPPED_VALUE_COUNT_FIELD_NUMBER: _ClassVar[int]
    track_count: int
    box_count: int
    gold_frame_count: int
    dropped_value_count: int
    def __init__(self, track_count: _Optional[int] = ..., box_count: _Optional[int] = ..., gold_frame_count: _Optional[int] = ..., dropped_value_count: _Optional[int] = ...) -> None: ...
