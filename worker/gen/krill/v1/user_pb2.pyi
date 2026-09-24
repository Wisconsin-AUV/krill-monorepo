import datetime

from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Role(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    ROLE_UNSPECIFIED: _ClassVar[Role]
    ROLE_LABELER: _ClassVar[Role]
    ROLE_DEVELOPER: _ClassVar[Role]
    ROLE_ADMIN: _ClassVar[Role]

class Permission(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PERMISSION_UNSPECIFIED: _ClassVar[Permission]
    PERMISSION_LABEL: _ClassVar[Permission]
    PERMISSION_MANAGE_VIDEOS: _ClassVar[Permission]
    PERMISSION_MANAGE_LABEL_TYPES: _ClassVar[Permission]
    PERMISSION_MANAGE_EXPORTS: _ClassVar[Permission]
    PERMISSION_MANAGE_USERS: _ClassVar[Permission]
ROLE_UNSPECIFIED: Role
ROLE_LABELER: Role
ROLE_DEVELOPER: Role
ROLE_ADMIN: Role
PERMISSION_UNSPECIFIED: Permission
PERMISSION_LABEL: Permission
PERMISSION_MANAGE_VIDEOS: Permission
PERMISSION_MANAGE_LABEL_TYPES: Permission
PERMISSION_MANAGE_EXPORTS: Permission
PERMISSION_MANAGE_USERS: Permission

class RoleInfo(_message.Message):
    __slots__ = ("role", "label")
    ROLE_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    role: Role
    label: str
    def __init__(self, role: _Optional[_Union[Role, str]] = ..., label: _Optional[str] = ...) -> None: ...

class PermissionInfo(_message.Message):
    __slots__ = ("permission", "description", "role")
    PERMISSION_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    permission: Permission
    description: str
    role: Role
    def __init__(self, permission: _Optional[_Union[Permission, str]] = ..., description: _Optional[str] = ..., role: _Optional[_Union[Role, str]] = ...) -> None: ...

class User(_message.Message):
    __slots__ = ("id", "name", "username", "email", "role", "disabled", "has_password", "slack_linked", "created_at", "last_login_at", "permissions")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    DISABLED_FIELD_NUMBER: _ClassVar[int]
    HAS_PASSWORD_FIELD_NUMBER: _ClassVar[int]
    SLACK_LINKED_FIELD_NUMBER: _ClassVar[int]
    CREATED_AT_FIELD_NUMBER: _ClassVar[int]
    LAST_LOGIN_AT_FIELD_NUMBER: _ClassVar[int]
    PERMISSIONS_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    username: str
    email: str
    role: Role
    disabled: bool
    has_password: bool
    slack_linked: bool
    created_at: _timestamp_pb2.Timestamp
    last_login_at: _timestamp_pb2.Timestamp
    permissions: _containers.RepeatedScalarFieldContainer[Permission]
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., username: _Optional[str] = ..., email: _Optional[str] = ..., role: _Optional[_Union[Role, str]] = ..., disabled: _Optional[bool] = ..., has_password: _Optional[bool] = ..., slack_linked: _Optional[bool] = ..., created_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., last_login_at: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., permissions: _Optional[_Iterable[_Union[Permission, str]]] = ...) -> None: ...

class ListUsersRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListUsersResponse(_message.Message):
    __slots__ = ("users",)
    USERS_FIELD_NUMBER: _ClassVar[int]
    users: _containers.RepeatedCompositeFieldContainer[User]
    def __init__(self, users: _Optional[_Iterable[_Union[User, _Mapping]]] = ...) -> None: ...

class UpdateUserRequest(_message.Message):
    __slots__ = ("id", "role", "disabled")
    ID_FIELD_NUMBER: _ClassVar[int]
    ROLE_FIELD_NUMBER: _ClassVar[int]
    DISABLED_FIELD_NUMBER: _ClassVar[int]
    id: str
    role: Role
    disabled: bool
    def __init__(self, id: _Optional[str] = ..., role: _Optional[_Union[Role, str]] = ..., disabled: _Optional[bool] = ...) -> None: ...

class UpdateUserResponse(_message.Message):
    __slots__ = ("user",)
    USER_FIELD_NUMBER: _ClassVar[int]
    user: User
    def __init__(self, user: _Optional[_Union[User, _Mapping]] = ...) -> None: ...

class SetUserPasswordRequest(_message.Message):
    __slots__ = ("id", "password")
    ID_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    id: str
    password: str
    def __init__(self, id: _Optional[str] = ..., password: _Optional[str] = ...) -> None: ...

class SetUserPasswordResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class DeleteUserRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeleteUserResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
