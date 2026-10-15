// Package email renders and sends Krill's emails. The templates are built from
// the React Email sources in /emails with `just gen`.
package email

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"log/slog"
	texttemplate "text/template"
)

//go:embed templates
var files embed.FS

var (
	htmlTemplates = htmltemplate.Must(htmltemplate.ParseFS(files, "templates/*.html"))
	textTemplates = texttemplate.Must(texttemplate.ParseFS(files, "templates/*.txt"))
)

type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

type Sender interface {
	Send(ctx context.Context, m Message) error
}

// Code fills in the templates for an emailed code. Email is for verification
// and Username for the others.
type Code struct {
	Code     string
	Minutes  int
	IP       string
	Device   string
	Time     string
	Team     string
	Year     int
	Email    string
	Username string
}

func VerifyEmail(to string, c Code) (Message, error) {
	return render(to, c.Code+" is your Krill verification code", "verify-email", c)
}

func ResetPassword(to string, c Code) (Message, error) {
	return render(to, c.Code+" is your Krill password reset code", "reset-password", c)
}

func SignInCode(to string, c Code) (Message, error) {
	return render(to, c.Code+" is your Krill sign-in code", "sign-in-code", c)
}

// Test fills in the template for an email an admin sends to check email works.
type Test struct {
	Username string
	Team     string
	Year     int
}

func TestEmail(to string, t Test) (Message, error) {
	return render(to, "Krill can send email", "test-email", t)
}

// Invite fills in the template for an emailed invite link.
type Invite struct {
	Inviter string
	// Role reads like "labeler".
	Role string
	URL  string
	// Expires reads like "on 21 October 2026".
	Expires string
	Team    string
	Year    int
}

func InviteEmail(to string, i Invite) (Message, error) {
	return render(to, i.Inviter+" invited you to Krill", "invite", i)
}

func render(to, subject, name string, data any) (Message, error) {
	var html, text bytes.Buffer
	if err := htmlTemplates.ExecuteTemplate(&html, name+".html", data); err != nil {
		return Message{}, fmt.Errorf("render %s html: %w", name, err)
	}
	if err := textTemplates.ExecuteTemplate(&text, name+".txt", data); err != nil {
		return Message{}, fmt.Errorf("render %s text: %w", name, err)
	}
	return Message{To: to, Subject: subject, HTML: html.String(), Text: text.String()}, nil
}

// Log writes emails to the log instead of sending them, for local development.
type Log struct{}

func (Log) Send(_ context.Context, m Message) error {
	slog.Info("email not sent; KRILL_RESEND_API_KEY is unset", "to", m.To, "subject", m.Subject, "text", m.Text)
	return nil
}
