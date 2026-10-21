from krill.v1 import annotation_pb2 as _annotation_pb2
from krill.v1 import pipeline_pb2 as _pipeline_pb2
from krill.v1 import worker_pb2 as _worker_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class RegisterPrelabelersRequest(_message.Message):
    __slots__ = ("prelabelers",)
    PRELABELERS_FIELD_NUMBER: _ClassVar[int]
    prelabelers: _containers.RepeatedCompositeFieldContainer[_pipeline_pb2.PrelabelerSpec]
    def __init__(self, prelabelers: _Optional[_Iterable[_Union[_pipeline_pb2.PrelabelerSpec, _Mapping]]] = ...) -> None: ...

class RegisterPrelabelersResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CheckInRequest(_message.Message):
    __slots__ = ("task_id",)
    TASK_ID_FIELD_NUMBER: _ClassVar[int]
    task_id: int
    def __init__(self, task_id: _Optional[int] = ...) -> None: ...

class CheckInResponse(_message.Message):
    __slots__ = ("task",)
    TASK_FIELD_NUMBER: _ClassVar[int]
    task: _worker_pb2.NextTaskResponse
    def __init__(self, task: _Optional[_Union[_worker_pb2.NextTaskResponse, _Mapping]] = ...) -> None: ...

class ReportTrainProgressRequest(_message.Message):
    __slots__ = ("task_id", "epochs", "epoch", "evaluating", "log", "device")
    TASK_ID_FIELD_NUMBER: _ClassVar[int]
    EPOCHS_FIELD_NUMBER: _ClassVar[int]
    EPOCH_FIELD_NUMBER: _ClassVar[int]
    EVALUATING_FIELD_NUMBER: _ClassVar[int]
    LOG_FIELD_NUMBER: _ClassVar[int]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    task_id: int
    epochs: int
    epoch: _pipeline_pb2.EpochMetrics
    evaluating: bool
    log: str
    device: str
    def __init__(self, task_id: _Optional[int] = ..., epochs: _Optional[int] = ..., epoch: _Optional[_Union[_pipeline_pb2.EpochMetrics, _Mapping]] = ..., evaluating: _Optional[bool] = ..., log: _Optional[str] = ..., device: _Optional[str] = ...) -> None: ...

class ReportTrainProgressResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class FinishTrainRequest(_message.Message):
    __slots__ = ("task_id", "val", "test", "baseline_val", "baseline_test", "weights_bytes", "log")
    TASK_ID_FIELD_NUMBER: _ClassVar[int]
    VAL_FIELD_NUMBER: _ClassVar[int]
    TEST_FIELD_NUMBER: _ClassVar[int]
    BASELINE_VAL_FIELD_NUMBER: _ClassVar[int]
    BASELINE_TEST_FIELD_NUMBER: _ClassVar[int]
    WEIGHTS_BYTES_FIELD_NUMBER: _ClassVar[int]
    LOG_FIELD_NUMBER: _ClassVar[int]
    task_id: int
    val: _pipeline_pb2.EvalMetrics
    test: _pipeline_pb2.EvalMetrics
    baseline_val: _pipeline_pb2.EvalMetrics
    baseline_test: _pipeline_pb2.EvalMetrics
    weights_bytes: int
    log: str
    def __init__(self, task_id: _Optional[int] = ..., val: _Optional[_Union[_pipeline_pb2.EvalMetrics, _Mapping]] = ..., test: _Optional[_Union[_pipeline_pb2.EvalMetrics, _Mapping]] = ..., baseline_val: _Optional[_Union[_pipeline_pb2.EvalMetrics, _Mapping]] = ..., baseline_test: _Optional[_Union[_pipeline_pb2.EvalMetrics, _Mapping]] = ..., weights_bytes: _Optional[int] = ..., log: _Optional[str] = ...) -> None: ...

class FinishTrainResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class PrelabelBox(_message.Message):
    __slots__ = ("frame_id", "label", "box", "confidence", "plugin")
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    BOX_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    PLUGIN_FIELD_NUMBER: _ClassVar[int]
    frame_id: int
    label: str
    box: _annotation_pb2.Box
    confidence: float
    plugin: str
    def __init__(self, frame_id: _Optional[int] = ..., label: _Optional[str] = ..., box: _Optional[_Union[_annotation_pb2.Box, _Mapping]] = ..., confidence: _Optional[float] = ..., plugin: _Optional[str] = ...) -> None: ...

class ReportPrelabelsRequest(_message.Message):
    __slots__ = ("task_id", "boxes", "done", "frames_done")
    TASK_ID_FIELD_NUMBER: _ClassVar[int]
    BOXES_FIELD_NUMBER: _ClassVar[int]
    DONE_FIELD_NUMBER: _ClassVar[int]
    FRAMES_DONE_FIELD_NUMBER: _ClassVar[int]
    task_id: int
    boxes: _containers.RepeatedCompositeFieldContainer[PrelabelBox]
    done: bool
    frames_done: int
    def __init__(self, task_id: _Optional[int] = ..., boxes: _Optional[_Iterable[_Union[PrelabelBox, _Mapping]]] = ..., done: _Optional[bool] = ..., frames_done: _Optional[int] = ...) -> None: ...

class ReportPrelabelsResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ReportPreviewRequest(_message.Message):
    __slots__ = ("task_id", "boxes", "took_ms")
    TASK_ID_FIELD_NUMBER: _ClassVar[int]
    BOXES_FIELD_NUMBER: _ClassVar[int]
    TOOK_MS_FIELD_NUMBER: _ClassVar[int]
    task_id: int
    boxes: _containers.RepeatedCompositeFieldContainer[_pipeline_pb2.PreviewBox]
    took_ms: int
    def __init__(self, task_id: _Optional[int] = ..., boxes: _Optional[_Iterable[_Union[_pipeline_pb2.PreviewBox, _Mapping]]] = ..., took_ms: _Optional[int] = ...) -> None: ...

class ReportPreviewResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
