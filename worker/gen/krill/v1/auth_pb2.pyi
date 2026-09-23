from krill.v1 import user_pb2 as _user_pb2
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetSessionRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetSessionResponse(_message.Message):
    __slots__ = ("user", "slack_enabled", "signup_enabled")
    USER_FIELD_NUMBER: _ClassVar[int]
    SLACK_ENABLED_FIELD_NUMBER: _ClassVar[int]
    SIGNUP_ENABLED_FIELD_NUMBER: _ClassVar[int]
    user: _user_pb2.User
    slack_enabled: bool
    signup_enabled: bool
    def __init__(self, user: _Optional[_Union[_user_pb2.User, _Mapping]] = ..., slack_enabled: _Optional[bool] = ..., signup_enabled: _Optional[bool] = ...) -> None: ...

class LoginRequest(_message.Message):
    __slots__ = ("login", "password")
    LOGIN_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    login: str
    password: str
    def __init__(self, login: _Optional[str] = ..., password: _Optional[str] = ...) -> None: ...

class LoginResponse(_message.Message):
    __slots__ = ("user",)
    USER_FIELD_NUMBER: _ClassVar[int]
    user: _user_pb2.User
    def __init__(self, user: _Optional[_Union[_user_pb2.User, _Mapping]] = ...) -> None: ...

class RegisterRequest(_message.Message):
    __slots__ = ("name", "username", "email", "password")
    NAME_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    PASSWORD_FIELD_NUMBER: _ClassVar[int]
    name: str
    username: str
    email: str
    password: str
    def __init__(self, name: _Optional[str] = ..., username: _Optional[str] = ..., email: _Optional[str] = ..., password: _Optional[str] = ...) -> None: ...

class RegisterResponse(_message.Message):
    __slots__ = ("user",)
    USER_FIELD_NUMBER: _ClassVar[int]
    user: _user_pb2.User
    def __init__(self, user: _Optional[_Union[_user_pb2.User, _Mapping]] = ...) -> None: ...

class LogoutRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class LogoutResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class UpdateProfileRequest(_message.Message):
    __slots__ = ("name", "username", "email")
    NAME_FIELD_NUMBER: _ClassVar[int]
    USERNAME_FIELD_NUMBER: _ClassVar[int]
    EMAIL_FIELD_NUMBER: _ClassVar[int]
    name: str
    username: str
    email: str
    def __init__(self, name: _Optional[str] = ..., username: _Optional[str] = ..., email: _Optional[str] = ...) -> None: ...

class UpdateProfileResponse(_message.Message):
    __slots__ = ("user",)
    USER_FIELD_NUMBER: _ClassVar[int]
    user: _user_pb2.User
    def __init__(self, user: _Optional[_Union[_user_pb2.User, _Mapping]] = ...) -> None: ...

class ChangePasswordRequest(_message.Message):
    __slots__ = ("current_password", "new_password")
    CURRENT_PASSWORD_FIELD_NUMBER: _ClassVar[int]
    NEW_PASSWORD_FIELD_NUMBER: _ClassVar[int]
    current_password: str
    new_password: str
    def __init__(self, current_password: _Optional[str] = ..., new_password: _Optional[str] = ...) -> None: ...

class ChangePasswordResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...
