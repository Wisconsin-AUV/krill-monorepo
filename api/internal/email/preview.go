package email

import (
	"fmt"
	"time"
)

// Sample is who a preview is addressed to.
type Sample struct {
	To       string
	Username string
	Team     string
	Now      time.Time
}

type Preview struct {
	Name    string
	Label   string
	Message Message
}

// previews builds every email the API sends through the same functions that
// send it, so a preview shows exactly what goes out.
var previews = []struct {
	name, label string
	build       func(Sample) (Message, error)
}{
	{"verify-email", "Verify email", func(s Sample) (Message, error) { return VerifyEmail(s.To, sampleCode(s)) }},
	{"reset-password", "Reset password", func(s Sample) (Message, error) { return ResetPassword(s.To, sampleCode(s)) }},
	{"sign-in-code", "Sign-in code", func(s Sample) (Message, error) { return SignInCode(s.To, sampleCode(s)) }},
	{"test-email", "Test email", func(s Sample) (Message, error) {
		return TestEmail(s.To, Test{Username: s.Username, Team: s.Team, Year: s.Now.Year()})
	}},
}

func sampleCode(s Sample) Code {
	return Code{
		Code:     "042137",
		Minutes:  10,
		IP:       "128.104.24.71",
		Device:   "Firefox on macOS",
		Time:     s.Now.UTC().Format("2 January 2006, 15:04 UTC"),
		Team:     s.Team,
		Year:     s.Now.Year(),
		Email:    s.To,
		Username: s.Username,
	}
}

func Previews(s Sample) ([]Preview, error) {
	out := make([]Preview, len(previews))
	for i, p := range previews {
		m, err := p.build(s)
		if err != nil {
			return nil, err
		}
		out[i] = Preview{Name: p.name, Label: p.label, Message: m}
	}
	return out, nil
}

func PreviewMessage(name string, s Sample) (Message, error) {
	for _, p := range previews {
		if p.name == name {
			return p.build(s)
		}
	}
	return Message{}, fmt.Errorf("no email named %q", name)
}
