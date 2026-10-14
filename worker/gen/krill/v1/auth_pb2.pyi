from krill.v1 import user_pb2 as _user_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class GetSessionRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetSessionResponse(_message.Message):
    __slots__ = ("user", "slack_enabled", "signup_enabled", "team_name", "all_permissions", "roles", "email_verification_required", "email_codes_enabled")
    USER_FIELD_NUMBER: _ClassVar[int]
    SLACK_ENABLED_FIELD_NUMBER: _ClassVar[int]
    SIGNUP_ENABLED_FIELD_NUMBER: _ClassVar[int]
    TEAM_NAME_FIELD_NUMBER: _ClassVar[int]
    ALL_PERMISSIONS_FIELD_NUMBER: _ClassVar[int]
    ROLES_FIELD_NUMBER: _ClassVar[int]
    EMAIL_VERIFICATION_REQUIRED_FIELD_NUMBER: _ClassVar[int]
    EMAIL_CODES_ENABLED_FIELD_NUMBER: _ClassVar[int]
    user: _user_pb2.User
    slack_enabled: bool
    signup_enabled: bool
    team_name: str
    all_permissions: _containers.RepeatedCompositeFieldContainer[_user_pb2.PermissionInfo]
    roles: _containers.RepeatedCompositeFieldContainer[_user_pb2.RoleInfo]
    email_verification_required: bool
    email_codes_enabled: bool
    def __init__(self, user: _Optional[_Union[_user_pb2.User, _Mapping]] = ..., slack_enabled: _Optional[bool] = ..., signup_enabled: _Optional[bool] = ..., team_name: _Optional[str] = ..., all_permissions: _Optional[_Iterable[_Union[_user_pb2.PermissionInfo, _Mapping]]] = ..., roles: _Optional[_Iterable[_Union[_user_pb2.RoleInfo, _Mapping]]] = ..., email_verification_required: _Optional[bool] = ..., email_codes_enabled: _Optional[bool] = ...) -> None: ...

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

class SendVerificationCodeRequest(_message.Message):
    __slots__ = ("resend",)
    RESEND_FIELD_NUMBER: _ClassVar[int]
    resend: bool
    def __init__(self, resend: _Optional[bool] = ...) -> None: ...

class SendVerificationCodeResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class VerifyEmailRequest(_message.Message):
    __slots__ = ("code",)
    CODE_FIELD_NUMBER: _ClassVar[int]
    code: str
    def __init__(self, code: _Optional[str] = ...) -> None: ...

class VerifyEmailResponse(_message.Message):
    __slots__ = ("user",)
    USER_FIELD_NUMBER: _ClassVar[int]
    user: _user_pb2.User
    def __init__(self, user: _Optional[_Union[_user_pb2.User, _Mapping]] = ...) -> None: ...

class RequestPasswordResetRequest(_message.Message):
    __slots__ = ("login",)
    LOGIN_FIELD_NUMBER: _ClassVar[int]
    login: str
    def __init__(self, login: _Optional[str] = ...) -> None: ...

class RequestPasswordResetResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CheckResetCodeRequest(_message.Message):
    __slots__ = ("login", "code")
    LOGIN_FIELD_NUMBER: _ClassVar[int]
    CODE_FIELD_NUMBER: _ClassVar[int]
    login: str
    code: str
    def __init__(self, login: _Optional[str] = ..., code: _Optional[str] = ...) -> None: ...

class CheckResetCodeResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ResetPasswordRequest(_message.Message):
    __slots__ = ("login", "code", "new_password")
    LOGIN_FIELD_NUMBER: _ClassVar[int]
    CODE_FIELD_NUMBER: _ClassVar[int]
    NEW_PASSWORD_FIELD_NUMBER: _ClassVar[int]
    login: str
    code: str
    new_password: str
    def __init__(self, login: _Optional[str] = ..., code: _Optional[str] = ..., new_password: _Optional[str] = ...) -> None: ...

class ResetPasswordResponse(_message.Message):
    __slots__ = ("user",)
    USER_FIELD_NUMBER: _ClassVar[int]
    user: _user_pb2.User
    def __init__(self, user: _Optional[_Union[_user_pb2.User, _Mapping]] = ...) -> None: ...

class RequestSignInCodeRequest(_message.Message):
    __slots__ = ("login",)
    LOGIN_FIELD_NUMBER: _ClassVar[int]
    login: str
    def __init__(self, login: _Optional[str] = ...) -> None: ...

class RequestSignInCodeResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class SignInWithCodeRequest(_message.Message):
    __slots__ = ("login", "code")
    LOGIN_FIELD_NUMBER: _ClassVar[int]
    CODE_FIELD_NUMBER: _ClassVar[int]
    login: str
    code: str
    def __init__(self, login: _Optional[str] = ..., code: _Optional[str] = ...) -> None: ...

class SignInWithCodeResponse(_message.Message):
    __slots__ = ("user",)
    USER_FIELD_NUMBER: _ClassVar[int]
    user: _user_pb2.User
    def __init__(self, user: _Optional[_Union[_user_pb2.User, _Mapping]] = ...) -> None: ...
