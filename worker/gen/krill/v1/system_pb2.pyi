from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class HostStats(_message.Message):
    __slots__ = ("cpu_percent", "cpu_count", "memory_used_bytes", "memory_total_bytes", "disk_used_bytes", "disk_total_bytes", "platform")
    CPU_PERCENT_FIELD_NUMBER: _ClassVar[int]
    CPU_COUNT_FIELD_NUMBER: _ClassVar[int]
    MEMORY_USED_BYTES_FIELD_NUMBER: _ClassVar[int]
    MEMORY_TOTAL_BYTES_FIELD_NUMBER: _ClassVar[int]
    DISK_USED_BYTES_FIELD_NUMBER: _ClassVar[int]
    DISK_TOTAL_BYTES_FIELD_NUMBER: _ClassVar[int]
    PLATFORM_FIELD_NUMBER: _ClassVar[int]
    cpu_percent: float
    cpu_count: int
    memory_used_bytes: int
    memory_total_bytes: int
    disk_used_bytes: int
    disk_total_bytes: int
    platform: str
    def __init__(self, cpu_percent: _Optional[float] = ..., cpu_count: _Optional[int] = ..., memory_used_bytes: _Optional[int] = ..., memory_total_bytes: _Optional[int] = ..., disk_used_bytes: _Optional[int] = ..., disk_total_bytes: _Optional[int] = ..., platform: _Optional[str] = ...) -> None: ...

class GpuStats(_message.Message):
    __slots__ = ("index", "name", "memory_used_bytes", "memory_total_bytes", "utilization_percent", "temperature_celsius", "power_watts")
    INDEX_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    MEMORY_USED_BYTES_FIELD_NUMBER: _ClassVar[int]
    MEMORY_TOTAL_BYTES_FIELD_NUMBER: _ClassVar[int]
    UTILIZATION_PERCENT_FIELD_NUMBER: _ClassVar[int]
    TEMPERATURE_CELSIUS_FIELD_NUMBER: _ClassVar[int]
    POWER_WATTS_FIELD_NUMBER: _ClassVar[int]
    index: int
    name: str
    memory_used_bytes: int
    memory_total_bytes: int
    utilization_percent: float
    temperature_celsius: float
    power_watts: float
    def __init__(self, index: _Optional[int] = ..., name: _Optional[str] = ..., memory_used_bytes: _Optional[int] = ..., memory_total_bytes: _Optional[int] = ..., utilization_percent: _Optional[float] = ..., temperature_celsius: _Optional[float] = ..., power_watts: _Optional[float] = ...) -> None: ...
