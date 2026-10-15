from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class SlackProblem(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SLACK_PROBLEM_UNSPECIFIED: _ClassVar[SlackProblem]
    SLACK_PROBLEM_OFF: _ClassVar[SlackProblem]
    SLACK_PROBLEM_CLIENT_ID: _ClassVar[SlackProblem]
    SLACK_PROBLEM_CLIENT_SECRET: _ClassVar[SlackProblem]
    SLACK_PROBLEM_REDIRECT_URL: _ClassVar[SlackProblem]
    SLACK_PROBLEM_TEAM_ID: _ClassVar[SlackProblem]
    SLACK_PROBLEM_WRONG_TEAM: _ClassVar[SlackProblem]
    SLACK_PROBLEM_CANCELLED: _ClassVar[SlackProblem]
    SLACK_PROBLEM_EXPIRED: _ClassVar[SlackProblem]
    SLACK_PROBLEM_UNREACHABLE: _ClassVar[SlackProblem]
    SLACK_PROBLEM_OTHER: _ClassVar[SlackProblem]
SLACK_PROBLEM_UNSPECIFIED: SlackProblem
SLACK_PROBLEM_OFF: SlackProblem
SLACK_PROBLEM_CLIENT_ID: SlackProblem
SLACK_PROBLEM_CLIENT_SECRET: SlackProblem
SLACK_PROBLEM_REDIRECT_URL: SlackProblem
SLACK_PROBLEM_TEAM_ID: SlackProblem
SLACK_PROBLEM_WRONG_TEAM: SlackProblem
SLACK_PROBLEM_CANCELLED: SlackProblem
SLACK_PROBLEM_EXPIRED: SlackProblem
SLACK_PROBLEM_UNREACHABLE: SlackProblem
SLACK_PROBLEM_OTHER: SlackProblem

class CheckSlackCredentialsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CheckSlackCredentialsResponse(_message.Message):
    __slots__ = ("problem", "detail", "redirect_url")
    PROBLEM_FIELD_NUMBER: _ClassVar[int]
    DETAIL_FIELD_NUMBER: _ClassVar[int]
    REDIRECT_URL_FIELD_NUMBER: _ClassVar[int]
    problem: SlackProblem
    detail: str
    redirect_url: str
    def __init__(self, problem: _Optional[_Union[SlackProblem, str]] = ..., detail: _Optional[str] = ..., redirect_url: _Optional[str] = ...) -> None: ...

class StartSlackTestRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class StartSlackTestResponse(_message.Message):
    __slots__ = ("authorize_url", "test_id", "redirect_url")
    AUTHORIZE_URL_FIELD_NUMBER: _ClassVar[int]
    TEST_ID_FIELD_NUMBER: _ClassVar[int]
    REDIRECT_URL_FIELD_NUMBER: _ClassVar[int]
    authorize_url: str
    test_id: str
    redirect_url: str
    def __init__(self, authorize_url: _Optional[str] = ..., test_id: _Optional[str] = ..., redirect_url: _Optional[str] = ...) -> None: ...

class GetSlackTestResultRequest(_message.Message):
    __slots__ = ("test_id",)
    TEST_ID_FIELD_NUMBER: _ClassVar[int]
    test_id: str
    def __init__(self, test_id: _Optional[str] = ...) -> None: ...

class GetSlackTestResultResponse(_message.Message):
    __slots__ = ("problem", "detail", "redirect_url", "team_id", "team_name", "expected_team_id", "user_name")
    PROBLEM_FIELD_NUMBER: _ClassVar[int]
    DETAIL_FIELD_NUMBER: _ClassVar[int]
    REDIRECT_URL_FIELD_NUMBER: _ClassVar[int]
    TEAM_ID_FIELD_NUMBER: _ClassVar[int]
    TEAM_NAME_FIELD_NUMBER: _ClassVar[int]
    EXPECTED_TEAM_ID_FIELD_NUMBER: _ClassVar[int]
    USER_NAME_FIELD_NUMBER: _ClassVar[int]
    problem: SlackProblem
    detail: str
    redirect_url: str
    team_id: str
    team_name: str
    expected_team_id: str
    user_name: str
    def __init__(self, problem: _Optional[_Union[SlackProblem, str]] = ..., detail: _Optional[str] = ..., redirect_url: _Optional[str] = ..., team_id: _Optional[str] = ..., team_name: _Optional[str] = ..., expected_team_id: _Optional[str] = ..., user_name: _Optional[str] = ...) -> None: ...

class ListEmailPreviewsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class EmailPreview(_message.Message):
    __slots__ = ("name", "label", "subject", "html", "text")
    NAME_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    SUBJECT_FIELD_NUMBER: _ClassVar[int]
    HTML_FIELD_NUMBER: _ClassVar[int]
    TEXT_FIELD_NUMBER: _ClassVar[int]
    name: str
    label: str
    subject: str
    html: str
    text: str
    def __init__(self, name: _Optional[str] = ..., label: _Optional[str] = ..., subject: _Optional[str] = ..., html: _Optional[str] = ..., text: _Optional[str] = ...) -> None: ...

class ListEmailPreviewsResponse(_message.Message):
    __slots__ = ("emails",)
    EMAILS_FIELD_NUMBER: _ClassVar[int]
    emails: _containers.RepeatedCompositeFieldContainer[EmailPreview]
    def __init__(self, emails: _Optional[_Iterable[_Union[EmailPreview, _Mapping]]] = ...) -> None: ...

class SendEmailPreviewRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class SendEmailPreviewResponse(_message.Message):
    __slots__ = ("to", "logged")
    TO_FIELD_NUMBER: _ClassVar[int]
    LOGGED_FIELD_NUMBER: _ClassVar[int]
    to: str
    logged: bool
    def __init__(self, to: _Optional[str] = ..., logged: _Optional[bool] = ...) -> None: ...
