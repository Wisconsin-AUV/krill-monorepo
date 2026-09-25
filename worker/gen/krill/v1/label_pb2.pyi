from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class LabelAttribute(_message.Message):
    __slots__ = ("name", "options")
    NAME_FIELD_NUMBER: _ClassVar[int]
    OPTIONS_FIELD_NUMBER: _ClassVar[int]
    name: str
    options: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, name: _Optional[str] = ..., options: _Optional[_Iterable[str]] = ...) -> None: ...

class LabelType(_message.Message):
    __slots__ = ("id", "name", "color", "description", "position", "attributes", "track_count", "box_count", "title", "guideline", "examples")
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
    def __init__(self, id: _Optional[int] = ..., name: _Optional[str] = ..., color: _Optional[str] = ..., description: _Optional[str] = ..., position: _Optional[int] = ..., attributes: _Optional[_Iterable[_Union[LabelAttribute, _Mapping]]] = ..., track_count: _Optional[int] = ..., box_count: _Optional[int] = ..., title: _Optional[str] = ..., guideline: _Optional[str] = ..., examples: _Optional[_Iterable[_Union[LabelExample, _Mapping]]] = ...) -> None: ...

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
    __slots__ = ("name", "color", "description", "attributes", "title", "guideline")
    NAME_FIELD_NUMBER: _ClassVar[int]
    COLOR_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    GUIDELINE_FIELD_NUMBER: _ClassVar[int]
    name: str
    color: str
    description: str
    attributes: _containers.RepeatedCompositeFieldContainer[LabelAttribute]
    title: str
    guideline: str
    def __init__(self, name: _Optional[str] = ..., color: _Optional[str] = ..., description: _Optional[str] = ..., attributes: _Optional[_Iterable[_Union[LabelAttribute, _Mapping]]] = ..., title: _Optional[str] = ..., guideline: _Optional[str] = ...) -> None: ...

class CreateLabelTypeResponse(_message.Message):
    __slots__ = ("label_type",)
    LABEL_TYPE_FIELD_NUMBER: _ClassVar[int]
    label_type: LabelType
    def __init__(self, label_type: _Optional[_Union[LabelType, _Mapping]] = ...) -> None: ...

class UpdateLabelTypeRequest(_message.Message):
    __slots__ = ("id", "name", "color", "description", "attributes", "title", "guideline")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    COLOR_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    GUIDELINE_FIELD_NUMBER: _ClassVar[int]
    id: int
    name: str
    color: str
    description: str
    attributes: _containers.RepeatedCompositeFieldContainer[LabelAttribute]
    title: str
    guideline: str
    def __init__(self, id: _Optional[int] = ..., name: _Optional[str] = ..., color: _Optional[str] = ..., description: _Optional[str] = ..., attributes: _Optional[_Iterable[_Union[LabelAttribute, _Mapping]]] = ..., title: _Optional[str] = ..., guideline: _Optional[str] = ...) -> None: ...

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
