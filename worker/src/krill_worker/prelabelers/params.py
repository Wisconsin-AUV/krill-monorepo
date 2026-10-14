"""Typed parameters for classic CV pre-labelers.

Each plugin declares its parameters as a frozen dataclass. Field types pick the
kind (int, float, bool, or HsvRange) and field metadata from param() gives the
label and bounds the admin panel shows.
"""

import dataclasses
import typing
from collections.abc import Iterable, Mapping
from dataclasses import dataclass
from typing import Any, TypeVar

import cv2
import numpy as np
import numpy.typing as npt
from krill.v1 import pipeline_pb2

Mask = npt.NDArray[np.uint8]

P = TypeVar("P")


@dataclass(frozen=True)
class HsvRange:
    """OpenCV HSV: hue 0 to 179, saturation and value 0 to 255.

    Hue wraps around when h_min is above h_max, which red needs.
    """

    h_min: int
    s_min: int
    v_min: int
    h_max: int
    s_max: int
    v_max: int

    def mask(self, hsv: npt.NDArray[np.uint8]) -> Mask:
        lo = (self.s_min, self.v_min)
        hi = (self.s_max, self.v_max)
        if self.h_min <= self.h_max:
            return np.asarray(
                cv2.inRange(hsv, (self.h_min, *lo), (self.h_max, *hi)), dtype=np.uint8
            )
        upper = cv2.inRange(hsv, (self.h_min, *lo), (179, *hi))
        lower = cv2.inRange(hsv, (0, *lo), (self.h_max, *hi))
        return np.asarray(cv2.bitwise_or(upper, lower), dtype=np.uint8)


def param(label: str, lo: float = 0, hi: float = 0, step: float = 0) -> dict[str, Any]:
    """Field metadata for a parameter. Bounds and step only apply to numbers."""
    return {"label": label, "min": lo, "max": hi, "step": step}


_KINDS: dict[Any, pipeline_pb2.ParamKind] = {
    int: pipeline_pb2.PARAM_KIND_INT,
    float: pipeline_pb2.PARAM_KIND_FLOAT,
    bool: pipeline_pb2.PARAM_KIND_BOOL,
    HsvRange: pipeline_pb2.PARAM_KIND_HSV_RANGE,
}


def _fields(cls: type[Any]) -> Iterable[tuple[dataclasses.Field[Any], Any]]:
    hints = typing.get_type_hints(cls)
    for f in dataclasses.fields(cls):
        yield f, hints[f.name]


def to_value(name: str, v: Any) -> pipeline_pb2.ParamValue:
    if isinstance(v, HsvRange):
        hsv = pipeline_pb2.HsvRange(**dataclasses.asdict(v))
        return pipeline_pb2.ParamValue(name=name, hsv=hsv)
    if isinstance(v, bool):
        return pipeline_pb2.ParamValue(name=name, flag=v)
    return pipeline_pb2.ParamValue(name=name, number=float(v))


def specs(cls: type[Any]) -> list[pipeline_pb2.ParamSpec]:
    """Describe a params dataclass for the admin panel."""
    defaults = cls()
    out = []
    for f, hint in _fields(cls):
        meta = f.metadata
        out.append(
            pipeline_pb2.ParamSpec(
                name=f.name,
                label=meta.get("label", f.name),
                kind=_KINDS[hint],
                min=meta.get("min", 0),
                max=meta.get("max", 0),
                step=meta.get("step", 0),
                default_value=to_value(f.name, getattr(defaults, f.name)),
            )
        )
    return out


def parse(cls: type[P], values: Iterable[pipeline_pb2.ParamValue]) -> P:
    """Build a params dataclass from the API's values. Missing ones keep their
    default, and values of the wrong kind are rejected."""
    given: Mapping[str, pipeline_pb2.ParamValue] = {v.name: v for v in values}
    kwargs: dict[str, Any] = {}
    for f, hint in _fields(cls):
        v = given.get(f.name)
        if v is None:
            continue
        which = v.WhichOneof("value")
        if hint is HsvRange and which == "hsv":
            h = v.hsv
            kwargs[f.name] = HsvRange(h.h_min, h.s_min, h.v_min, h.h_max, h.s_max, h.v_max)
        elif hint is bool and which == "flag":
            kwargs[f.name] = v.flag
        elif hint is int and which == "number":
            kwargs[f.name] = int(v.number)
        elif hint is float and which == "number":
            kwargs[f.name] = v.number
        else:
            raise ValueError(f"{f.name} must be a {hint.__name__}, not {which}")
    return cls(**kwargs)
