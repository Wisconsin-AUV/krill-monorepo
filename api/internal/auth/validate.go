package auth

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	minPasswordLen = 8
	// argon2 accepts any length, but a cap stops huge inputs from being hashed.
	maxPasswordLen = 256
	maxNameLen     = 100
	maxEmailLen    = 254
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{1,31}$`)

type Profile struct {
	Name, Username, Email string
}

func NormalizeProfile(name, username, email string) (Profile, error) {
	p := Profile{
		Name:     strings.TrimSpace(name),
		Username: strings.ToLower(strings.TrimSpace(username)),
		Email:    strings.ToLower(strings.TrimSpace(email)),
	}
	if p.Name == "" {
		return Profile{}, errors.New("name is required")
	}
	if utf8.RuneCountInString(p.Name) > maxNameLen {
		return Profile{}, errors.New("name must be at most 100 characters")
	}
	if !usernamePattern.MatchString(p.Username) {
		return Profile{}, errors.New("username must be 2 to 32 characters: letters, numbers, dots, dashes, or underscores")
	}
	if err := validateEmail(p.Email); err != nil {
		return Profile{}, err
	}
	return p, nil
}

func validateEmail(email string) error {
	if len(email) > maxEmailLen {
		return errors.New("email is too long")
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email[strings.LastIndex(email, "@"):], ".") {
		return errors.New("email is not valid")
	}
	return nil
}

// ValidatePassword enforces the rules the web app can check without its
// strength estimator
func ValidatePassword(password string) error {
	n := utf8.RuneCountInString(password)
	if n < minPasswordLen {
		return errors.New("password must be at least 8 characters")
	}
	if n > maxPasswordLen {
		return errors.New("password must be at most 256 characters")
	}
	var upper, lower, digit, symbol bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case unicode.IsNumber(r):
			digit = true
		case !unicode.IsLetter(r):
			symbol = true
		}
	}
	types := 0
	for _, has := range []bool{upper, lower, digit, symbol} {
		if has {
			types++
		}
	}
	if types < 3 {
		return errors.New("password needs at least 3 of: uppercase letter, lowercase letter, number, symbol")
	}
	return nil
}
