import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from krill.v1 import annotation_pb2 as _annotation_pb2
from krill.v1 import export_pb2 as _export_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ModelSize(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MODEL_SIZE_UNSPECIFIED: _ClassVar[ModelSize]
    MODEL_SIZE_NANO: _ClassVar[ModelSize]
    MODEL_SIZE_SMALL: _ClassVar[ModelSize]

class TrainingStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    TRAINING_STATUS_UNSPECIFIED: _ClassVar[TrainingStatus]
    TRAINING_STATUS_QUEUED: _ClassVar[TrainingStatus]
    TRAINING_STATUS_PREPARING: _ClassVar[TrainingStatus]
    TRAINING_STATUS_TRAINING: _ClassVar[TrainingStatus]
    TRAINING_STATUS_EVALUATING: _ClassVar[TrainingStatus]
    TRAINING_STATUS_SUCCEEDED: _ClassVar[TrainingStatus]
    TRAINING_STATUS_FAILED: _ClassVar[TrainingStatus]
    TRAINING_STATUS_CANCELLED: _ClassVar[TrainingStatus]

class TrainingTrigger(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    TRAINING_TRIGGER_UNSPECIFIED: _ClassVar[TrainingTrigger]
    TRAINING_TRIGGER_MANUAL: _ClassVar[TrainingTrigger]
    TRAINING_TRIGGER_SCHEDULE: _ClassVar[TrainingTrigger]

class PrelabelSource(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PRELABEL_SOURCE_UNSPECIFIED: _ClassVar[PrelabelSource]
    PRELABEL_SOURCE_YOLO: _ClassVar[PrelabelSource]
    PRELABEL_SOURCE_CV: _ClassVar[PrelabelSource]

class PrelabelStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PRELABEL_STATUS_UNSPECIFIED: _ClassVar[PrelabelStatus]
    PRELABEL_STATUS_QUEUED: _ClassVar[PrelabelStatus]
    PRELABEL_STATUS_RUNNING: _ClassVar[PrelabelStatus]
    PRELABEL_STATUS_SUCCEEDED: _ClassVar[PrelabelStatus]
    PRELABEL_STATUS_FAILED: _ClassVar[PrelabelStatus]
    PRELABEL_STATUS_CANCELLED: _ClassVar[PrelabelStatus]

class ParamKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PARAM_KIND_UNSPECIFIED: _ClassVar[ParamKind]
    PARAM_KIND_INT: _ClassVar[ParamKind]
    PARAM_KIND_FLOAT: _ClassVar[ParamKind]
    PARAM_KIND_BOOL: _ClassVar[ParamKind]
    PARAM_KIND_HSV_RANGE: _ClassVar[ParamKind]

class ModelTrialStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MODEL_TRIAL_STATUS_UNSPECIFIED: _ClassVar[ModelTrialStatus]
    MODEL_TRIAL_STATUS_QUEUED: _ClassVar[ModelTrialStatus]
    MODEL_TRIAL_STATUS_RUNNING: _ClassVar[ModelTrialStatus]
    MODEL_TRIAL_STATUS_SUCCEEDED: _ClassVar[ModelTrialStatus]
    MODEL_TRIAL_STATUS_FAILED: _ClassVar[ModelTrialStatus]
    MODEL_TRIAL_STATUS_CANCELLED: _ClassVar[ModelTrialStatus]
MODEL_SIZE_UNSPECIFIED: ModelSize
MODEL_SIZE_NANO: ModelSize
MODEL_SIZE_SMALL: ModelSize
TRAINING_STATUS_UNSPECIFIED: TrainingStatus
TRAINING_STATUS_QUEUED: TrainingStatus
TRAINING_STATUS_PREPARING: TrainingStatus
TRAINING_STATUS_TRAINING: TrainingStatus
TRAINING_STATUS_EVALUATING: TrainingStatus
TRAINING_STATUS_SUCCEEDED: TrainingStatus
TRAINING_STATUS_FAILED: TrainingStatus
TRAINING_STATUS_CANCELLED: TrainingStatus
TRAINING_TRIGGER_UNSPECIFIED: TrainingTrigger
TRAINING_TRIGGER_MANUAL: TrainingTrigger
TRAINING_TRIGGER_SCHEDULE: TrainingTrigger
PRELABEL_SOURCE_UNSPECIFIED: PrelabelSource
PRELABEL_SOURCE_YOLO: PrelabelSource
PRELABEL_SOURCE_CV: PrelabelSource
PRELABEL_STATUS_UNSPECIFIED: PrelabelStatus
PRELABEL_STATUS_QUEUED: PrelabelStatus
PRELABEL_STATUS_RUNNING: PrelabelStatus
PRELABEL_STATUS_SUCCEEDED: PrelabelStatus
PRELABEL_STATUS_FAILED: PrelabelStatus
PRELABEL_STATUS_CANCELLED: PrelabelStatus
PARAM_KIND_UNSPECIFIED: ParamKind
PARAM_KIND_INT: ParamKind
PARAM_KIND_FLOAT: ParamKind
PARAM_KIND_BOOL: ParamKind
PARAM_KIND_HSV_RANGE: ParamKind
MODEL_TRIAL_STATUS_UNSPECIFIED: ModelTrialStatus
MODEL_TRIAL_STATUS_QUEUED: ModelTrialStatus
MODEL_TRIAL_STATUS_RUNNING: ModelTrialStatus
MODEL_TRIAL_STATUS_SUCCEEDED: ModelTrialStatus
MODEL_TRIAL_STATUS_FAILED: ModelTrialStatus
MODEL_TRIAL_STATUS_CANCELLED: ModelTrialStatus

class TrainConfig(_message.Message):
    __slots__ = ("size", "epochs", "image_size", "batch", "base_run_id", "dataset")
    SIZE_FIELD_NUMBER: _ClassVar[int]
    EPOCHS_FIELD_NUMBER: _ClassVar[int]
    IMAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    BATCH_FIELD_NUMBER: _ClassVar[int]
    BASE_RUN_ID_FIELD_NUMBER: _ClassVar[int]
    DATASET_FIELD_NUMBER: _ClassVar[int]
    size: ModelSize
    epochs: int
    image_size: int
    batch: int
    base_run_id: int
    dataset: _export_pb2.ExportOptions
    def __init__(self, size: _Optional[_Union[ModelSize, str]] = ..., epochs: _Optional[int] = ..., image_size: _Optional[int] = ..., batch: _Optional[int] = ..., base_run_id: _Optional[int] = ..., dataset: _Optional[_Union[_export_pb2.ExportOptions, _Mapping]] = ...) -> None: ...

class EpochMetrics(_message.Message):
    __slots__ = ("epoch", "box_loss", "cls_loss", "precision", "recall", "map50", "map50_95")
    EPOCH_FIELD_NUMBER: _ClassVar[int]
    BOX_LOSS_FIELD_NUMBER: _ClassVar[int]
    CLS_LOSS_FIELD_NUMBER: _ClassVar[int]
    PRECISION_FIELD_NUMBER: _ClassVar[int]
    RECALL_FIELD_NUMBER: _ClassVar[int]
    MAP50_FIELD_NUMBER: _ClassVar[int]
    MAP50_95_FIELD_NUMBER: _ClassVar[int]
    epoch: int
    box_loss: float
    cls_loss: float
    precision: float
    recall: float
    map50: float
    map50_95: float
    def __init__(self, epoch: _Optional[int] = ..., box_loss: _Optional[float] = ..., cls_loss: _Optional[float] = ..., precision: _Optional[float] = ..., recall: _Optional[float] = ..., map50: _Optional[float] = ..., map50_95: _Optional[float] = ...) -> None: ...

class ClassMetrics(_message.Message):
    __slots__ = ("name", "images", "instances", "precision", "recall", "map50", "map50_95")
    NAME_FIELD_NUMBER: _ClassVar[int]
    IMAGES_FIELD_NUMBER: _ClassVar[int]
    INSTANCES_FIELD_NUMBER: _ClassVar[int]
    PRECISION_FIELD_NUMBER: _ClassVar[int]
    RECALL_FIELD_NUMBER: _ClassVar[int]
    MAP50_FIELD_NUMBER: _ClassVar[int]
    MAP50_95_FIELD_NUMBER: _ClassVar[int]
    name: str
    images: int
    instances: int
    precision: float
    recall: float
    map50: float
    map50_95: float
    def __init__(self, name: _Optional[str] = ..., images: _Optional[int] = ..., instances: _Optional[int] = ..., precision: _Optional[float] = ..., recall: _Optional[float] = ..., map50: _Optional[float] = ..., map50_95: _Optional[float] = ...) -> None: ...

class EvalMetrics(_message.Message):
    __slots__ = ("all", "classes")
    ALL_FIELD_NUMBER: _ClassVar[int]
    CLASSES_FIELD_NUMBER: _ClassVar[int]
    all: ClassMetrics
    classes: _containers.RepeatedCompositeFieldContainer[ClassMetrics]
    def __init__(self, all: _Optional[_Union[ClassMetrics, _Mapping]] = ..., classes: _Optional[_Iterable[_Union[ClassMetrics, _Mapping]]] = ...) -> None: ...

class TrainingRun(_message.Message):
    __slots__ = ("id", "status", "trigger", "created_by", "config", "dataset", "epoch", "history", "val", "test", "baseline_run_id", "baseline_val", "baseline_test", "pinned", "error", "log", "created_at", "started_at", "finished_at", "weights_url", "weights_bytes", "device", "verified_boxes")
    ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    TRIGGER_FIELD_NUMBER: _ClassVar[int]
    CREATED_BY_FIELD_NUMBER: _ClassVar[int]
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    DATASET_FIELD_NUMBER: _ClassVar[int]
    EPOCH_FIELD_NUMBER: _ClassVar[int]
    HISTORY_FIELD_NUMBER: _ClassVar[int]
    VAL_FIELD_NUMBER: _ClassVar[int]
    TEST_FIELD_NUMBER: _ClassVar[int]
    BASELINE_RUN_ID_FIELD_NUMBER: _ClassVar[int]
    BASELINE_VAL_FIELD_NUMBER: _ClassVar[int]
    BASELINE_TEST_FIELD_NUMBER: _ClassVar[int]
    PINNED_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    LOG_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    STARTED_AT_FIELD_NUMBER: _ClassVar[int]
    FINISHED_AT_FIELD_NUMBER: _ClassVar[int]
    WEIGHTS_URL_FIELD_NUMBER: _ClassVar[int]
    WEIGHTS_BYTES_FIELD_NUMBER: _ClassVar[int]
    DEVICE_FIELD_NUMBER: _ClassVar[int]
    VERIFIED_BOXES_FIELD_NUMBER: _ClassVar[int]
    id: int
    status: TrainingStatus
    trigger: TrainingTrigger
    created_by: str
    config: TrainConfig
    dataset: _export_pb2.ExportStats
    epoch: int
    history: _containers.RepeatedCompositeFieldContainer[EpochMetrics]
    val: EvalMetrics
    test: EvalMetrics
    baseline_run_id: int
    baseline_val: EvalMetrics
    baseline_test: EvalMetrics
    pinned: bool
    error: str
    log: str
    created_at: _timestamp_pb2.Timestamp
    started_at: _timestamp_pb2.Timestamp
    finished_at: _timestamp_pb2.Timestamp
    weights_url: str
    weights_bytes: int
    device: str
    verified_boxes: int
    def __init__(self, id: _Optional[int] = ..., status: _Optional[_Union[TrainingStatus, str]] = ..., trigger: _Optional[_Union[TrainingTrigger, str]] = ..., created_by: _Optional[str] = ..., config: _Optional[_Union[TrainConfig, _Mapping]] = ..., dataset: _Optional[_Union[_export_pb2.ExportStats, _Mapping]] = ..., epoch: _Optional[int] = ..., history: _Optional[_Iterable[_Union[EpochMetrics, _Mapping]]] = ..., val: _Optional[_Union[EvalMetrics, _Mapping]] = ..., test: _Optional[_Union[EvalMetrics, _Mapping]] = ..., baseline_run_id: _Optional[int] = ..., baseline_val: _Optional[_Union[EvalMetrics, _Mapping]] = ..., baseline_test: _Optional[_Union[EvalMetrics, _Mapping]] = ..., pinned: _Optional[bool] = ..., error: _Optional[str] = ..., log: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., started_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., finished_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., weights_url: _Optional[str] = ..., weights_bytes: _Optional[int] = ..., device: _Optional[str] = ..., verified_boxes: _Optional[int] = ...) -> None: ...

class RetrainSchedule(_message.Message):
    __slots__ = ("hour", "time_zone", "min_new_labels", "new_labels", "auto_pin", "next_check")
    HOUR_FIELD_NUMBER: _ClassVar[int]
    TIME_ZONE_FIELD_NUMBER: _ClassVar[int]
    MIN_NEW_LABELS_FIELD_NUMBER: _ClassVar[int]
    NEW_LABELS_FIELD_NUMBER: _ClassVar[int]
    AUTO_PIN_FIELD_NUMBER: _ClassVar[int]
    NEXT_CHECK_FIELD_NUMBER: _ClassVar[int]
    hour: int
    time_zone: str
    min_new_labels: int
    new_labels: int
    auto_pin: bool
    next_check: _timestamp_pb2.Timestamp
    def __init__(self, hour: _Optional[int] = ..., time_zone: _Optional[str] = ..., min_new_labels: _Optional[int] = ..., new_labels: _Optional[int] = ..., auto_pin: _Optional[bool] = ..., next_check: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListTrainingRunsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListTrainingRunsResponse(_message.Message):
    __slots__ = ("runs", "pinned_run_id", "defaults", "schedule")
    RUNS_FIELD_NUMBER: _ClassVar[int]
    PINNED_RUN_ID_FIELD_NUMBER: _ClassVar[int]
    DEFAULTS_FIELD_NUMBER: _ClassVar[int]
    SCHEDULE_FIELD_NUMBER: _ClassVar[int]
    runs: _containers.RepeatedCompositeFieldContainer[TrainingRun]
    pinned_run_id: int
    defaults: TrainConfig
    schedule: RetrainSchedule
    def __init__(self, runs: _Optional[_Iterable[_Union[TrainingRun, _Mapping]]] = ..., pinned_run_id: _Optional[int] = ..., defaults: _Optional[_Union[TrainConfig, _Mapping]] = ..., schedule: _Optional[_Union[RetrainSchedule, _Mapping]] = ...) -> None: ...

class GetTrainingRunRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class GetTrainingRunResponse(_message.Message):
    __slots__ = ("run", "pinned")
    RUN_FIELD_NUMBER: _ClassVar[int]
    PINNED_FIELD_NUMBER: _ClassVar[int]
    run: TrainingRun
    pinned: TrainingRun
    def __init__(self, run: _Optional[_Union[TrainingRun, _Mapping]] = ..., pinned: _Optional[_Union[TrainingRun, _Mapping]] = ...) -> None: ...

class StartTrainingRequest(_message.Message):
    __slots__ = ("config",)
    CONFIG_FIELD_NUMBER: _ClassVar[int]
    config: TrainConfig
    def __init__(self, config: _Optional[_Union[TrainConfig, _Mapping]] = ...) -> None: ...

class StartTrainingResponse(_message.Message):
    __slots__ = ("run",)
    RUN_FIELD_NUMBER: _ClassVar[int]
    run: TrainingRun
    def __init__(self, run: _Optional[_Union[TrainingRun, _Mapping]] = ...) -> None: ...

class CancelTrainingRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class CancelTrainingResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class PinModelRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class PinModelResponse(_message.Message):
    __slots__ = ("prelabel",)
    PRELABEL_FIELD_NUMBER: _ClassVar[int]
    prelabel: PrelabelRun
    def __init__(self, prelabel: _Optional[_Union[PrelabelRun, _Mapping]] = ...) -> None: ...

class PrelabelRun(_message.Message):
    __slots__ = ("id", "source", "training_run_id", "status", "clips_total", "clips_done", "frames", "proposals", "error", "created_by", "created_at", "finished_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    TRAINING_RUN_ID_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    CLIPS_TOTAL_FIELD_NUMBER: _ClassVar[int]
    CLIPS_DONE_FIELD_NUMBER: _ClassVar[int]
    FRAMES_FIELD_NUMBER: _ClassVar[int]
    PROPOSALS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    CREATED_BY_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    FINISHED_AT_FIELD_NUMBER: _ClassVar[int]
    id: int
    source: PrelabelSource
    training_run_id: int
    status: PrelabelStatus
    clips_total: int
    clips_done: int
    frames: int
    proposals: int
    error: str
    created_by: str
    created_at: _timestamp_pb2.Timestamp
    finished_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[int] = ..., source: _Optional[_Union[PrelabelSource, str]] = ..., training_run_id: _Optional[int] = ..., status: _Optional[_Union[PrelabelStatus, str]] = ..., clips_total: _Optional[int] = ..., clips_done: _Optional[int] = ..., frames: _Optional[int] = ..., proposals: _Optional[int] = ..., error: _Optional[str] = ..., created_by: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., finished_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListPrelabelRunsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListPrelabelRunsResponse(_message.Message):
    __slots__ = ("runs", "open_yolo_proposals", "open_cv_proposals")
    RUNS_FIELD_NUMBER: _ClassVar[int]
    OPEN_YOLO_PROPOSALS_FIELD_NUMBER: _ClassVar[int]
    OPEN_CV_PROPOSALS_FIELD_NUMBER: _ClassVar[int]
    runs: _containers.RepeatedCompositeFieldContainer[PrelabelRun]
    open_yolo_proposals: int
    open_cv_proposals: int
    def __init__(self, runs: _Optional[_Iterable[_Union[PrelabelRun, _Mapping]]] = ..., open_yolo_proposals: _Optional[int] = ..., open_cv_proposals: _Optional[int] = ...) -> None: ...

class StartPrelabelRequest(_message.Message):
    __slots__ = ("source",)
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    source: PrelabelSource
    def __init__(self, source: _Optional[_Union[PrelabelSource, str]] = ...) -> None: ...

class StartPrelabelResponse(_message.Message):
    __slots__ = ("run",)
    RUN_FIELD_NUMBER: _ClassVar[int]
    run: PrelabelRun
    def __init__(self, run: _Optional[_Union[PrelabelRun, _Mapping]] = ...) -> None: ...

class CancelPrelabelRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class CancelPrelabelResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class HsvRange(_message.Message):
    __slots__ = ("h_min", "s_min", "v_min", "h_max", "s_max", "v_max")
    H_MIN_FIELD_NUMBER: _ClassVar[int]
    S_MIN_FIELD_NUMBER: _ClassVar[int]
    V_MIN_FIELD_NUMBER: _ClassVar[int]
    H_MAX_FIELD_NUMBER: _ClassVar[int]
    S_MAX_FIELD_NUMBER: _ClassVar[int]
    V_MAX_FIELD_NUMBER: _ClassVar[int]
    h_min: int
    s_min: int
    v_min: int
    h_max: int
    s_max: int
    v_max: int
    def __init__(self, h_min: _Optional[int] = ..., s_min: _Optional[int] = ..., v_min: _Optional[int] = ..., h_max: _Optional[int] = ..., s_max: _Optional[int] = ..., v_max: _Optional[int] = ...) -> None: ...

class ParamValue(_message.Message):
    __slots__ = ("name", "number", "flag", "hsv")
    NAME_FIELD_NUMBER: _ClassVar[int]
    NUMBER_FIELD_NUMBER: _ClassVar[int]
    FLAG_FIELD_NUMBER: _ClassVar[int]
    HSV_FIELD_NUMBER: _ClassVar[int]
    name: str
    number: float
    flag: bool
    hsv: HsvRange
    def __init__(self, name: _Optional[str] = ..., number: _Optional[float] = ..., flag: _Optional[bool] = ..., hsv: _Optional[_Union[HsvRange, _Mapping]] = ...) -> None: ...

class ParamSpec(_message.Message):
    __slots__ = ("name", "label", "kind", "min", "max", "step", "default_value")
    NAME_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    MIN_FIELD_NUMBER: _ClassVar[int]
    MAX_FIELD_NUMBER: _ClassVar[int]
    STEP_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_VALUE_FIELD_NUMBER: _ClassVar[int]
    name: str
    label: str
    kind: ParamKind
    min: float
    max: float
    step: float
    default_value: ParamValue
    def __init__(self, name: _Optional[str] = ..., label: _Optional[str] = ..., kind: _Optional[_Union[ParamKind, str]] = ..., min: _Optional[float] = ..., max: _Optional[float] = ..., step: _Optional[float] = ..., default_value: _Optional[_Union[ParamValue, _Mapping]] = ...) -> None: ...

class PrelabelerSpec(_message.Message):
    __slots__ = ("name", "title", "outputs", "params", "oriented")
    NAME_FIELD_NUMBER: _ClassVar[int]
    TITLE_FIELD_NUMBER: _ClassVar[int]
    OUTPUTS_FIELD_NUMBER: _ClassVar[int]
    PARAMS_FIELD_NUMBER: _ClassVar[int]
    ORIENTED_FIELD_NUMBER: _ClassVar[int]
    name: str
    title: str
    outputs: _containers.RepeatedScalarFieldContainer[str]
    params: _containers.RepeatedCompositeFieldContainer[ParamSpec]
    oriented: bool
    def __init__(self, name: _Optional[str] = ..., title: _Optional[str] = ..., outputs: _Optional[_Iterable[str]] = ..., params: _Optional[_Iterable[_Union[ParamSpec, _Mapping]]] = ..., oriented: _Optional[bool] = ...) -> None: ...

class PrelabelerOutput(_message.Message):
    __slots__ = ("name", "label_type_id", "attributes")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    name: str
    label_type_id: int
    attributes: _containers.ScalarMap[str, str]
    def __init__(self, name: _Optional[str] = ..., label_type_id: _Optional[int] = ..., attributes: _Optional[_Mapping[str, str]] = ...) -> None: ...

class Prelabeler(_message.Message):
    __slots__ = ("spec", "enabled", "params", "outputs", "updated_at", "updated_by")
    SPEC_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    PARAMS_FIELD_NUMBER: _ClassVar[int]
    OUTPUTS_FIELD_NUMBER: _ClassVar[int]
    UPDATED_AT_FIELD_NUMBER: _ClassVar[int]
    UPDATED_BY_FIELD_NUMBER: _ClassVar[int]
    spec: PrelabelerSpec
    enabled: bool
    params: _containers.RepeatedCompositeFieldContainer[ParamValue]
    outputs: _containers.RepeatedCompositeFieldContainer[PrelabelerOutput]
    updated_at: _timestamp_pb2.Timestamp
    updated_by: str
    def __init__(self, spec: _Optional[_Union[PrelabelerSpec, _Mapping]] = ..., enabled: _Optional[bool] = ..., params: _Optional[_Iterable[_Union[ParamValue, _Mapping]]] = ..., outputs: _Optional[_Iterable[_Union[PrelabelerOutput, _Mapping]]] = ..., updated_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., updated_by: _Optional[str] = ...) -> None: ...

class ListPrelabelersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListPrelabelersResponse(_message.Message):
    __slots__ = ("prelabelers",)
    PRELABELERS_FIELD_NUMBER: _ClassVar[int]
    prelabelers: _containers.RepeatedCompositeFieldContainer[Prelabeler]
    def __init__(self, prelabelers: _Optional[_Iterable[_Union[Prelabeler, _Mapping]]] = ...) -> None: ...

class UpdatePrelabelerRequest(_message.Message):
    __slots__ = ("name", "enabled", "params", "outputs")
    NAME_FIELD_NUMBER: _ClassVar[int]
    ENABLED_FIELD_NUMBER: _ClassVar[int]
    PARAMS_FIELD_NUMBER: _ClassVar[int]
    OUTPUTS_FIELD_NUMBER: _ClassVar[int]
    name: str
    enabled: bool
    params: _containers.RepeatedCompositeFieldContainer[ParamValue]
    outputs: _containers.RepeatedCompositeFieldContainer[PrelabelerOutput]
    def __init__(self, name: _Optional[str] = ..., enabled: _Optional[bool] = ..., params: _Optional[_Iterable[_Union[ParamValue, _Mapping]]] = ..., outputs: _Optional[_Iterable[_Union[PrelabelerOutput, _Mapping]]] = ...) -> None: ...

class UpdatePrelabelerResponse(_message.Message):
    __slots__ = ("prelabeler",)
    PRELABELER_FIELD_NUMBER: _ClassVar[int]
    prelabeler: Prelabeler
    def __init__(self, prelabeler: _Optional[_Union[Prelabeler, _Mapping]] = ...) -> None: ...

class PreviewPrelabelerRequest(_message.Message):
    __slots__ = ("name", "params", "frame_id")
    NAME_FIELD_NUMBER: _ClassVar[int]
    PARAMS_FIELD_NUMBER: _ClassVar[int]
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    name: str
    params: _containers.RepeatedCompositeFieldContainer[ParamValue]
    frame_id: int
    def __init__(self, name: _Optional[str] = ..., params: _Optional[_Iterable[_Union[ParamValue, _Mapping]]] = ..., frame_id: _Optional[int] = ...) -> None: ...

class PreviewBox(_message.Message):
    __slots__ = ("output", "box", "confidence", "corners")
    OUTPUT_FIELD_NUMBER: _ClassVar[int]
    BOX_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    CORNERS_FIELD_NUMBER: _ClassVar[int]
    output: str
    box: _annotation_pb2.Box
    confidence: float
    corners: _containers.RepeatedCompositeFieldContainer[_annotation_pb2.Point]
    def __init__(self, output: _Optional[str] = ..., box: _Optional[_Union[_annotation_pb2.Box, _Mapping]] = ..., confidence: _Optional[float] = ..., corners: _Optional[_Iterable[_Union[_annotation_pb2.Point, _Mapping]]] = ...) -> None: ...

class PreviewPrelabelerResponse(_message.Message):
    __slots__ = ("boxes", "mask_url", "frame_url", "took_ms")
    BOXES_FIELD_NUMBER: _ClassVar[int]
    MASK_URL_FIELD_NUMBER: _ClassVar[int]
    FRAME_URL_FIELD_NUMBER: _ClassVar[int]
    TOOK_MS_FIELD_NUMBER: _ClassVar[int]
    boxes: _containers.RepeatedCompositeFieldContainer[PreviewBox]
    mask_url: str
    frame_url: str
    took_ms: int
    def __init__(self, boxes: _Optional[_Iterable[_Union[PreviewBox, _Mapping]]] = ..., mask_url: _Optional[str] = ..., frame_url: _Optional[str] = ..., took_ms: _Optional[int] = ...) -> None: ...

class ModelTrialBox(_message.Message):
    __slots__ = ("frame_id", "label_type_id", "attributes", "box", "confidence", "track")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    FRAME_ID_FIELD_NUMBER: _ClassVar[int]
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    BOX_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    TRACK_FIELD_NUMBER: _ClassVar[int]
    frame_id: int
    label_type_id: int
    attributes: _containers.ScalarMap[str, str]
    box: _annotation_pb2.Box
    confidence: float
    track: int
    def __init__(self, frame_id: _Optional[int] = ..., label_type_id: _Optional[int] = ..., attributes: _Optional[_Mapping[str, str]] = ..., box: _Optional[_Union[_annotation_pb2.Box, _Mapping]] = ..., confidence: _Optional[float] = ..., track: _Optional[int] = ...) -> None: ...

class ModelTrialClass(_message.Message):
    __slots__ = ("label_type_id", "name", "matched", "correct", "missed", "extra", "precision", "recall")
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    MATCHED_FIELD_NUMBER: _ClassVar[int]
    CORRECT_FIELD_NUMBER: _ClassVar[int]
    MISSED_FIELD_NUMBER: _ClassVar[int]
    EXTRA_FIELD_NUMBER: _ClassVar[int]
    PRECISION_FIELD_NUMBER: _ClassVar[int]
    RECALL_FIELD_NUMBER: _ClassVar[int]
    label_type_id: int
    name: str
    matched: int
    correct: int
    missed: int
    extra: int
    precision: float
    recall: float
    def __init__(self, label_type_id: _Optional[int] = ..., name: _Optional[str] = ..., matched: _Optional[int] = ..., correct: _Optional[int] = ..., missed: _Optional[int] = ..., extra: _Optional[int] = ..., precision: _Optional[float] = ..., recall: _Optional[float] = ...) -> None: ...

class ModelTrialSummary(_message.Message):
    __slots__ = ("min_confidence", "labeled_frames", "all", "classes")
    MIN_CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    LABELED_FRAMES_FIELD_NUMBER: _ClassVar[int]
    ALL_FIELD_NUMBER: _ClassVar[int]
    CLASSES_FIELD_NUMBER: _ClassVar[int]
    min_confidence: float
    labeled_frames: int
    all: ModelTrialClass
    classes: _containers.RepeatedCompositeFieldContainer[ModelTrialClass]
    def __init__(self, min_confidence: _Optional[float] = ..., labeled_frames: _Optional[int] = ..., all: _Optional[_Union[ModelTrialClass, _Mapping]] = ..., classes: _Optional[_Iterable[_Union[ModelTrialClass, _Mapping]]] = ...) -> None: ...

class ModelTrial(_message.Message):
    __slots__ = ("id", "training_run_id", "clip_id", "video_id", "video_name", "clip_index", "status", "frames_total", "frames_done", "confidence_floor", "error", "created_by", "created_at", "finished_at", "summary")
    ID_FIELD_NUMBER: _ClassVar[int]
    TRAINING_RUN_ID_FIELD_NUMBER: _ClassVar[int]
    CLIP_ID_FIELD_NUMBER: _ClassVar[int]
    VIDEO_ID_FIELD_NUMBER: _ClassVar[int]
    VIDEO_NAME_FIELD_NUMBER: _ClassVar[int]
    CLIP_INDEX_FIELD_NUMBER: _ClassVar[int]
    STATUS_FIELD_NUMBER: _ClassVar[int]
    FRAMES_TOTAL_FIELD_NUMBER: _ClassVar[int]
    FRAMES_DONE_FIELD_NUMBER: _ClassVar[int]
    CONFIDENCE_FLOOR_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    CREATED_BY_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    FINISHED_AT_FIELD_NUMBER: _ClassVar[int]
    SUMMARY_FIELD_NUMBER: _ClassVar[int]
    id: int
    training_run_id: int
    clip_id: int
    video_id: int
    video_name: str
    clip_index: int
    status: ModelTrialStatus
    frames_total: int
    frames_done: int
    confidence_floor: float
    error: str
    created_by: str
    created_at: _timestamp_pb2.Timestamp
    finished_at: _timestamp_pb2.Timestamp
    summary: ModelTrialSummary
    def __init__(self, id: _Optional[int] = ..., training_run_id: _Optional[int] = ..., clip_id: _Optional[int] = ..., video_id: _Optional[int] = ..., video_name: _Optional[str] = ..., clip_index: _Optional[int] = ..., status: _Optional[_Union[ModelTrialStatus, str]] = ..., frames_total: _Optional[int] = ..., frames_done: _Optional[int] = ..., confidence_floor: _Optional[float] = ..., error: _Optional[str] = ..., created_by: _Optional[str] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., finished_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., summary: _Optional[_Union[ModelTrialSummary, _Mapping]] = ...) -> None: ...

class StartModelTrialRequest(_message.Message):
    __slots__ = ("training_run_id", "clip_id")
    TRAINING_RUN_ID_FIELD_NUMBER: _ClassVar[int]
    CLIP_ID_FIELD_NUMBER: _ClassVar[int]
    training_run_id: int
    clip_id: int
    def __init__(self, training_run_id: _Optional[int] = ..., clip_id: _Optional[int] = ...) -> None: ...

class StartModelTrialResponse(_message.Message):
    __slots__ = ("trial",)
    TRIAL_FIELD_NUMBER: _ClassVar[int]
    trial: ModelTrial
    def __init__(self, trial: _Optional[_Union[ModelTrial, _Mapping]] = ...) -> None: ...

class GetModelTrialRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class GetModelTrialResponse(_message.Message):
    __slots__ = ("trial", "boxes")
    TRIAL_FIELD_NUMBER: _ClassVar[int]
    BOXES_FIELD_NUMBER: _ClassVar[int]
    trial: ModelTrial
    boxes: _containers.RepeatedCompositeFieldContainer[ModelTrialBox]
    def __init__(self, trial: _Optional[_Union[ModelTrial, _Mapping]] = ..., boxes: _Optional[_Iterable[_Union[ModelTrialBox, _Mapping]]] = ...) -> None: ...

class ListModelTrialsRequest(_message.Message):
    __slots__ = ("training_run_id", "clip_id")
    TRAINING_RUN_ID_FIELD_NUMBER: _ClassVar[int]
    CLIP_ID_FIELD_NUMBER: _ClassVar[int]
    training_run_id: int
    clip_id: int
    def __init__(self, training_run_id: _Optional[int] = ..., clip_id: _Optional[int] = ...) -> None: ...

class ListModelTrialsResponse(_message.Message):
    __slots__ = ("trials",)
    TRIALS_FIELD_NUMBER: _ClassVar[int]
    trials: _containers.RepeatedCompositeFieldContainer[ModelTrial]
    def __init__(self, trials: _Optional[_Iterable[_Union[ModelTrial, _Mapping]]] = ...) -> None: ...

class CancelModelTrialRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class CancelModelTrialResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class PrelabelFrame(_message.Message):
    __slots__ = ("id", "url")
    ID_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    id: int
    url: str
    def __init__(self, id: _Optional[int] = ..., url: _Optional[str] = ...) -> None: ...

class TrainTask(_message.Message):
    __slots__ = ("id", "run_id", "dataset_url", "classes", "size", "epochs", "image_size", "batch", "base_weights_url", "baseline_weights_url", "weights_upload_url", "has_test")
    ID_FIELD_NUMBER: _ClassVar[int]
    RUN_ID_FIELD_NUMBER: _ClassVar[int]
    DATASET_URL_FIELD_NUMBER: _ClassVar[int]
    CLASSES_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    EPOCHS_FIELD_NUMBER: _ClassVar[int]
    IMAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    BATCH_FIELD_NUMBER: _ClassVar[int]
    BASE_WEIGHTS_URL_FIELD_NUMBER: _ClassVar[int]
    BASELINE_WEIGHTS_URL_FIELD_NUMBER: _ClassVar[int]
    WEIGHTS_UPLOAD_URL_FIELD_NUMBER: _ClassVar[int]
    HAS_TEST_FIELD_NUMBER: _ClassVar[int]
    id: int
    run_id: int
    dataset_url: str
    classes: _containers.RepeatedScalarFieldContainer[str]
    size: ModelSize
    epochs: int
    image_size: int
    batch: int
    base_weights_url: str
    baseline_weights_url: str
    weights_upload_url: str
    has_test: bool
    def __init__(self, id: _Optional[int] = ..., run_id: _Optional[int] = ..., dataset_url: _Optional[str] = ..., classes: _Optional[_Iterable[str]] = ..., size: _Optional[_Union[ModelSize, str]] = ..., epochs: _Optional[int] = ..., image_size: _Optional[int] = ..., batch: _Optional[int] = ..., base_weights_url: _Optional[str] = ..., baseline_weights_url: _Optional[str] = ..., weights_upload_url: _Optional[str] = ..., has_test: _Optional[bool] = ...) -> None: ...

class PluginConfig(_message.Message):
    __slots__ = ("name", "params")
    NAME_FIELD_NUMBER: _ClassVar[int]
    PARAMS_FIELD_NUMBER: _ClassVar[int]
    name: str
    params: _containers.RepeatedCompositeFieldContainer[ParamValue]
    def __init__(self, name: _Optional[str] = ..., params: _Optional[_Iterable[_Union[ParamValue, _Mapping]]] = ...) -> None: ...

class YoloModel(_message.Message):
    __slots__ = ("version", "weights_url", "min_confidence", "image_size")
    VERSION_FIELD_NUMBER: _ClassVar[int]
    WEIGHTS_URL_FIELD_NUMBER: _ClassVar[int]
    MIN_CONFIDENCE_FIELD_NUMBER: _ClassVar[int]
    IMAGE_SIZE_FIELD_NUMBER: _ClassVar[int]
    version: str
    weights_url: str
    min_confidence: float
    image_size: int
    def __init__(self, version: _Optional[str] = ..., weights_url: _Optional[str] = ..., min_confidence: _Optional[float] = ..., image_size: _Optional[int] = ...) -> None: ...

class CvPlugins(_message.Message):
    __slots__ = ("plugins",)
    PLUGINS_FIELD_NUMBER: _ClassVar[int]
    plugins: _containers.RepeatedCompositeFieldContainer[PluginConfig]
    def __init__(self, plugins: _Optional[_Iterable[_Union[PluginConfig, _Mapping]]] = ...) -> None: ...

class PrelabelTask(_message.Message):
    __slots__ = ("id", "frames", "yolo", "cv")
    ID_FIELD_NUMBER: _ClassVar[int]
    FRAMES_FIELD_NUMBER: _ClassVar[int]
    YOLO_FIELD_NUMBER: _ClassVar[int]
    CV_FIELD_NUMBER: _ClassVar[int]
    id: int
    frames: _containers.RepeatedCompositeFieldContainer[PrelabelFrame]
    yolo: YoloModel
    cv: CvPlugins
    def __init__(self, id: _Optional[int] = ..., frames: _Optional[_Iterable[_Union[PrelabelFrame, _Mapping]]] = ..., yolo: _Optional[_Union[YoloModel, _Mapping]] = ..., cv: _Optional[_Union[CvPlugins, _Mapping]] = ...) -> None: ...

class PreviewTask(_message.Message):
    __slots__ = ("id", "frame", "plugin", "mask_upload_url")
    ID_FIELD_NUMBER: _ClassVar[int]
    FRAME_FIELD_NUMBER: _ClassVar[int]
    PLUGIN_FIELD_NUMBER: _ClassVar[int]
    MASK_UPLOAD_URL_FIELD_NUMBER: _ClassVar[int]
    id: int
    frame: PrelabelFrame
    plugin: PluginConfig
    mask_upload_url: str
    def __init__(self, id: _Optional[int] = ..., frame: _Optional[_Union[PrelabelFrame, _Mapping]] = ..., plugin: _Optional[_Union[PluginConfig, _Mapping]] = ..., mask_upload_url: _Optional[str] = ...) -> None: ...
