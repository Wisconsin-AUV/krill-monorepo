import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from krill.v1 import user_pb2 as _user_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Period(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PERIOD_UNSPECIFIED: _ClassVar[Period]
    PERIOD_DAY: _ClassVar[Period]
    PERIOD_WEEK: _ClassVar[Period]
    PERIOD_MONTH: _ClassVar[Period]
    PERIOD_ALL_TIME: _ClassVar[Period]
PERIOD_UNSPECIFIED: Period
PERIOD_DAY: Period
PERIOD_WEEK: Period
PERIOD_MONTH: Period
PERIOD_ALL_TIME: Period

class Profile(_message.Message):
    __slots__ = ("id", "name", "username", "role", "created_at")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    username: str
    role: _user_pb2.Role
    created_at: _timestamp_pb2.Timestamp
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., username: _Optional[str] = ..., role: _Optional[_Union[_user_pb2.Role, str]] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class Contributions(_message.Message):
    __slots__ = ("boxes", "frames", "total")
    BOXES_FIELD_NUMBER: _ClassVar[int]
    FRAMES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    boxes: int
    frames: int
    total: int
    def __init__(self, boxes: _Optional[int] = ..., frames: _Optional[int] = ..., total: _Optional[int] = ...) -> None: ...

class LeaderboardEntry(_message.Message):
    __slots__ = ("rank", "user", "contributions")
    RANK_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    CONTRIBUTIONS_FIELD_NUMBER: _ClassVar[int]
    rank: int
    user: Profile
    contributions: Contributions
    def __init__(self, rank: _Optional[int] = ..., user: _Optional[_Union[Profile, _Mapping]] = ..., contributions: _Optional[_Union[Contributions, _Mapping]] = ...) -> None: ...

class GetLeaderboardRequest(_message.Message):
    __slots__ = ("period", "time_zone")
    PERIOD_FIELD_NUMBER: _ClassVar[int]
    TIME_ZONE_FIELD_NUMBER: _ClassVar[int]
    period: Period
    time_zone: str
    def __init__(self, period: _Optional[_Union[Period, str]] = ..., time_zone: _Optional[str] = ...) -> None: ...

class GetLeaderboardResponse(_message.Message):
    __slots__ = ("entries", "since", "team")
    ENTRIES_FIELD_NUMBER: _ClassVar[int]
    SINCE_FIELD_NUMBER: _ClassVar[int]
    TEAM_FIELD_NUMBER: _ClassVar[int]
    entries: _containers.RepeatedCompositeFieldContainer[LeaderboardEntry]
    since: _timestamp_pb2.Timestamp
    team: Contributions
    def __init__(self, entries: _Optional[_Iterable[_Union[LeaderboardEntry, _Mapping]]] = ..., since: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., team: _Optional[_Union[Contributions, _Mapping]] = ...) -> None: ...

class GetProfileRequest(_message.Message):
    __slots__ = ("username", "time_zone")
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    TIME_ZONE_FIELD_NUMBER: _ClassVar[int]
    username: str
    time_zone: str
    def __init__(self, username: _Optional[str] = ..., time_zone: _Optional[str] = ...) -> None: ...

class PeriodStats(_message.Message):
    __slots__ = ("period", "contributions", "rank", "contributors")
    PERIOD_FIELD_NUMBER: _ClassVar[int]
    CONTRIBUTIONS_FIELD_NUMBER: _ClassVar[int]
    RANK_FIELD_NUMBER: _ClassVar[int]
    CONTRIBUTORS_FIELD_NUMBER: _ClassVar[int]
    period: Period
    contributions: Contributions
    rank: int
    contributors: int
    def __init__(self, period: _Optional[_Union[Period, str]] = ..., contributions: _Optional[_Union[Contributions, _Mapping]] = ..., rank: _Optional[int] = ..., contributors: _Optional[int] = ...) -> None: ...

class ContributionDay(_message.Message):
    __slots__ = ("date", "count")
    DATE_FIELD_NUMBER: _ClassVar[int]
    COUNT_FIELD_NUMBER: _ClassVar[int]
    date: str
    count: int
    def __init__(self, date: _Optional[str] = ..., count: _Optional[int] = ...) -> None: ...

class LabelTypeCount(_message.Message):
    __slots__ = ("label_type_id", "name", "color", "boxes")
    LABEL_TYPE_ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    COLOR_FIELD_NUMBER: _ClassVar[int]
    BOXES_FIELD_NUMBER: _ClassVar[int]
    label_type_id: int
    name: str
    color: str
    boxes: int
    def __init__(self, label_type_id: _Optional[int] = ..., name: _Optional[str] = ..., color: _Optional[str] = ..., boxes: _Optional[int] = ...) -> None: ...

class GetProfileResponse(_message.Message):
    __slots__ = ("user", "periods", "days", "current_streak", "longest_streak", "active_days", "best_day", "clips", "label_types")
    USER_FIELD_NUMBER: _ClassVar[int]
    PERIODS_FIELD_NUMBER: _ClassVar[int]
    DAYS_FIELD_NUMBER: _ClassVar[int]
    CURRENT_STREAK_FIELD_NUMBER: _ClassVar[int]
    LONGEST_STREAK_FIELD_NUMBER: _ClassVar[int]
    ACTIVE_DAYS_FIELD_NUMBER: _ClassVar[int]
    BEST_DAY_FIELD_NUMBER: _ClassVar[int]
    CLIPS_FIELD_NUMBER: _ClassVar[int]
    LABEL_TYPES_FIELD_NUMBER: _ClassVar[int]
    user: Profile
    periods: _containers.RepeatedCompositeFieldContainer[PeriodStats]
    days: _containers.RepeatedCompositeFieldContainer[ContributionDay]
    current_streak: int
    longest_streak: int
    active_days: int
    best_day: ContributionDay
    clips: int
    label_types: _containers.RepeatedCompositeFieldContainer[LabelTypeCount]
    def __init__(self, user: _Optional[_Union[Profile, _Mapping]] = ..., periods: _Optional[_Iterable[_Union[PeriodStats, _Mapping]]] = ..., days: _Optional[_Iterable[_Union[ContributionDay, _Mapping]]] = ..., current_streak: _Optional[int] = ..., longest_streak: _Optional[int] = ..., active_days: _Optional[int] = ..., best_day: _Optional[_Union[ContributionDay, _Mapping]] = ..., clips: _Optional[int] = ..., label_types: _Optional[_Iterable[_Union[LabelTypeCount, _Mapping]]] = ...) -> None: ...
