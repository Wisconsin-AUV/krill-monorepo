import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from krill.v1 import stats_pb2 as _stats_pb2
from krill.v1 import user_pb2 as _user_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class InviteState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    INVITE_STATE_UNSPECIFIED: _ClassVar[InviteState]
    INVITE_STATE_ACTIVE: _ClassVar[InviteState]
    INVITE_STATE_EXPIRED: _ClassVar[InviteState]
    INVITE_STATE_REVOKED: _ClassVar[InviteState]
    INVITE_STATE_USED_UP: _ClassVar[InviteState]
INVITE_STATE_UNSPECIFIED: InviteState
INVITE_STATE_ACTIVE: InviteState
INVITE_STATE_EXPIRED: InviteState
INVITE_STATE_REVOKED: InviteState
INVITE_STATE_USED_UP: InviteState

class InviteUse(_message.Message):
    __slots__ = ("user", "used_at")
    USER_FIELD_NUMBER: _ClassVar[int]
    USED_AT_FIELD_NUMBER: _ClassVar[int]
    user: _stats_pb2.Profile
    used_at: _timestamp_pb2.Timestamp
    def __init__(self, user: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., used_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class Invite(_message.Message):
    __slots__ = ("id", "role", "max_uses", "uses", "note", "email", "expires_at", "revoked_at", "created_at", "created_by", "used_by", "state")
    ID_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    MAX_USES_FIELD_NUMBER: _ClassVar[int]
    USES_FIELD_NUMBER: _ClassVar[int]
    NOTE_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_AT_FIELD_NUMBER: _ClassVar[int]
    REVOKED_AT_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    CREATED_BY_FIELD_NUMBER: _ClassVar[int]
    USED_BY_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    id: int
    role: _user_pb2.Role
    max_uses: int
    uses: int
    note: str
    email: str
    expires_at: _timestamp_pb2.Timestamp
    revoked_at: _timestamp_pb2.Timestamp
    created_at: _timestamp_pb2.Timestamp
    created_by: _stats_pb2.Profile
    used_by: _containers.RepeatedCompositeFieldContainer[InviteUse]
    state: InviteState
    def __init__(self, id: _Optional[int] = ..., role: _Optional[_Union[_user_pb2.Role, str]] = ..., max_uses: _Optional[int] = ..., uses: _Optional[int] = ..., note: _Optional[str] = ..., email: _Optional[str] = ..., expires_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., revoked_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., created_by: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., used_by: _Optional[_Iterable[_Union[InviteUse, _Mapping]]] = ..., state: _Optional[_Union[InviteState, str]] = ...) -> None: ...

class ListInvitesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListInvitesResponse(_message.Message):
    __slots__ = ("invites",)
    INVITES_FIELD_NUMBER: _ClassVar[int]
    invites: _containers.RepeatedCompositeFieldContainer[Invite]
    def __init__(self, invites: _Optional[_Iterable[_Union[Invite, _Mapping]]] = ...) -> None: ...

class CreateInviteRequest(_message.Message):
    __slots__ = ("role", "max_uses", "expires_in_hours", "note", "email")
    ROLE_FIELD_NUMBER: _ClassVar[int]
    MAX_USES_FIELD_NUMBER: _ClassVar[int]
    EXPIRES_IN_HOURS_FIELD_NUMBER: _ClassVar[int]
    NOTE_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    role: _user_pb2.Role
    max_uses: int
    expires_in_hours: int
    note: str
    email: str
    def __init__(self, role: _Optional[_Union[_user_pb2.Role, str]] = ..., max_uses: _Optional[int] = ..., expires_in_hours: _Optional[int] = ..., note: _Optional[str] = ..., email: _Optional[str] = ...) -> None: ...

class CreateInviteResponse(_message.Message):
    __slots__ = ("invite", "url")
    INVITE_FIELD_NUMBER: _ClassVar[int]
    URL_FIELD_NUMBER: _ClassVar[int]
    invite: Invite
    url: str
    def __init__(self, invite: _Optional[_Union[Invite, _Mapping]] = ..., url: _Optional[str] = ...) -> None: ...

class RevokeInviteRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: int
    def __init__(self, id: _Optional[int] = ...) -> None: ...

class RevokeInviteResponse(_message.Message):
    __slots__ = ("invite",)
    INVITE_FIELD_NUMBER: _ClassVar[int]
    invite: Invite
    def __init__(self, invite: _Optional[_Union[Invite, _Mapping]] = ...) -> None: ...

class ApprovedEmail(_message.Message):
    __slots__ = ("email", "role", "created_by", "created_at", "used_by", "used_at")
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    CREATED_BY_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    USED_BY_FIELD_NUMBER: _ClassVar[int]
    USED_AT_FIELD_NUMBER: _ClassVar[int]
    email: str
    role: _user_pb2.Role
    created_by: _stats_pb2.Profile
    created_at: _timestamp_pb2.Timestamp
    used_by: _stats_pb2.Profile
    used_at: _timestamp_pb2.Timestamp
    def __init__(self, email: _Optional[str] = ..., role: _Optional[_Union[_user_pb2.Role, str]] = ..., created_by: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., used_by: _Optional[_Union[_stats_pb2.Profile, _Mapping]] = ..., used_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ...) -> None: ...

class ListApprovedEmailsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListApprovedEmailsResponse(_message.Message):
    __slots__ = ("approved_emails",)
    APPROVED_EMAILS_FIELD_NUMBER: _ClassVar[int]
    approved_emails: _containers.RepeatedCompositeFieldContainer[ApprovedEmail]
    def __init__(self, approved_emails: _Optional[_Iterable[_Union[ApprovedEmail, _Mapping]]] = ...) -> None: ...

class AddApprovedEmailsRequest(_message.Message):
    __slots__ = ("emails", "role")
    EMAILS_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    emails: _containers.RepeatedScalarFieldContainer[str]
    role: _user_pb2.Role
    def __init__(self, emails: _Optional[_Iterable[str]] = ..., role: _Optional[_Union[_user_pb2.Role, str]] = ...) -> None: ...

class AddApprovedEmailsResponse(_message.Message):
    __slots__ = ("approved_emails",)
    APPROVED_EMAILS_FIELD_NUMBER: _ClassVar[int]
    approved_emails: _containers.RepeatedCompositeFieldContainer[ApprovedEmail]
    def __init__(self, approved_emails: _Optional[_Iterable[_Union[ApprovedEmail, _Mapping]]] = ...) -> None: ...

class RemoveApprovedEmailRequest(_message.Message):
    __slots__ = ("email",)
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    email: str
    def __init__(self, email: _Optional[str] = ...) -> None: ...

class RemoveApprovedEmailResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CheckInviteRequest(_message.Message):
    __slots__ = ("token",)
    TOKEN_FIELD_NUMBER: _ClassVar[int]
    token: str
    def __init__(self, token: _Optional[str] = ...) -> None: ...

class CheckInviteResponse(_message.Message):
    __slots__ = ("valid", "role", "email")
    VALID_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    valid: bool
    role: _user_pb2.Role
    email: str
    def __init__(self, valid: _Optional[bool] = ..., role: _Optional[_Union[_user_pb2.Role, str]] = ..., email: _Optional[str] = ...) -> None: ...
