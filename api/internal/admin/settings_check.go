package admin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/email"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/settings"
)

// SettingsCheck tries the saved Slack and email settings.
type SettingsCheck struct {
	krillv1connect.UnimplementedSettingsCheckServiceHandler
	slack    *auth.Slack
	settings *settings.Store
	mailer   func() email.Sender
}

func NewSettingsCheck(slack *auth.Slack, store *settings.Store, mailer func() email.Sender) *SettingsCheck {
	return &SettingsCheck{slack: slack, settings: store, mailer: mailer}
}

func slackResult(c auth.SlackCheck) *krillv1.GetSlackTestResultResponse {
	return &krillv1.GetSlackTestResultResponse{
		Problem:        c.Problem,
		Detail:         c.Detail,
		RedirectUrl:    c.RedirectURL,
		TeamId:         c.TeamID,
		TeamName:       c.TeamName,
		ExpectedTeamId: c.ExpectedTeamID,
		UserName:       c.UserName,
	}
}

func (s *SettingsCheck) CheckSlackCredentials(ctx context.Context, _ *krillv1.CheckSlackCredentialsRequest) (*krillv1.CheckSlackCredentialsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	c := s.slack.CheckCredentials(ctx)
	return &krillv1.CheckSlackCredentialsResponse{Problem: c.Problem, Detail: c.Detail, RedirectUrl: c.RedirectURL}, nil
}

func (s *SettingsCheck) StartSlackTest(ctx context.Context, _ *krillv1.StartSlackTestRequest) (*krillv1.StartSlackTestResponse, error) {
	url, id, c := s.slack.StartTest(auth.CallerID(ctx))
	if c.Problem != krillv1.SlackProblem_SLACK_PROBLEM_UNSPECIFIED {
		err := connect.NewError(connect.CodeFailedPrecondition, errors.New("fix the Slack settings first"))
		if d, derr := connect.NewErrorDetail(slackResult(c)); derr == nil {
			err.AddDetail(d)
		}
		return nil, err
	}
	return &krillv1.StartSlackTestResponse{AuthorizeUrl: url, TestId: id, RedirectUrl: c.RedirectURL}, nil
}

func (s *SettingsCheck) GetSlackTestResult(ctx context.Context, req *krillv1.GetSlackTestResultRequest) (*krillv1.GetSlackTestResultResponse, error) {
	return slackResult(s.slack.TestResult(auth.CallerID(ctx), req.GetTestId())), nil
}

func (s *SettingsCheck) sample(ctx context.Context) email.Sample {
	sess, _ := auth.SessionFrom(ctx)
	cfg := s.settings.Config()
	return email.Sample{
		To:        sess.User.Email,
		Name:      sess.User.Name,
		Username:  sess.User.Username,
		Team:      cfg.TeamName,
		PublicURL: cfg.PublicURL,
		Now:       time.Now(),
	}
}

func (s *SettingsCheck) ListEmailPreviews(ctx context.Context, _ *krillv1.ListEmailPreviewsRequest) (*krillv1.ListEmailPreviewsResponse, error) {
	previews, err := email.Previews(s.sample(ctx))
	if err != nil {
		return nil, rpc.Internal(err, "render emails")
	}
	out := make([]*krillv1.EmailPreview, len(previews))
	for i, p := range previews {
		out[i] = &krillv1.EmailPreview{
			Name:    p.Name,
			Label:   p.Label,
			Subject: p.Message.Subject,
			Html:    p.Message.HTML,
			Text:    p.Message.Text,
		}
	}
	return &krillv1.ListEmailPreviewsResponse{Emails: out}, nil
}

func (s *SettingsCheck) SendEmailPreview(ctx context.Context, req *krillv1.SendEmailPreviewRequest) (*krillv1.SendEmailPreviewResponse, error) {
	mailer := s.mailer()
	if mailer == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("set a Resend API key first"))
	}
	sample := s.sample(ctx)
	msg, err := email.PreviewMessage(req.GetName(), sample)
	if err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	if err := mailer.Send(ctx, msg); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("email was not sent: %w", err))
	}
	_, logged := mailer.(email.Log)
	return &krillv1.SendEmailPreviewResponse{To: sample.To, Logged: logged}, nil
}
