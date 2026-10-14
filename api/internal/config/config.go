package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// EmailVerification is how much sign-up trusts the email an account gives.
type EmailVerification string

const (
	// VerifyOff accepts any email.
	VerifyOff EmailVerification = "off"
	// VerifyDomain requires an email at SignupEmailDomain.
	VerifyDomain EmailVerification = "domain"
	// VerifyCode also requires entering a code sent to the email.
	VerifyCode EmailVerification = "verify"
)

type Config struct {
	Addr        string
	DatabaseURL string
	S3Endpoint  string
	// S3PublicEndpoint is the MinIO address browsers use for presigned URLs.
	S3PublicEndpoint string
	S3Bucket         string
	S3AccessKey      string
	S3SecretKey      string
	// WebDir holds the built web app. Empty means the API serves only RPCs,
	// which is the case in development where Vite serves the app.
	WebDir string
	// PublicURL is where browsers reach the app. Cookies are marked Secure
	// when it is HTTPS.
	PublicURL string
	// AllowSignup lets anyone register as a labeler. The first account can
	// always register and becomes an admin.
	AllowSignup bool
	// SignupEmailDomain limits password sign-up to emails at this domain.
	// Empty allows any email.
	SignupEmailDomain string
	EmailVerification EmailVerification
	// Emails go through Resend when ResendAPIKey is set. Without it, a local
	// instance logs them instead, and anything else sends none.
	ResendAPIKey string
	EmailFrom    string
	LogEmails    bool
	TeamName     string
	// Slack sign-in is enabled when SlackClientID is set.
	SlackClientID     string
	SlackClientSecret string
	SlackTeamID       string
	// WorkerToken authenticates GPU workers, as do tokens saved in the
	// database.
	WorkerToken string
	// EmbedAtIngest has the GPU worker compute SAM image embeddings for every
	// clip after ingest, as long as some worker token exists.
	EmbedAtIngest bool
	// ClientIPHeader is trusted for the client's address when set. Only set it
	// when every request comes through the proxy that writes it.
	ClientIPHeader string
	// A labeler's work goes to review while their mean score over recent gold
	// checks is below GoldReviewThreshold, once they have answered at least
	// GoldReviewMinAttempts checks.
	GoldReviewThreshold   float64
	GoldReviewMinAttempts int
	// UpdateCheck has the API look up the latest release on GitHub.
	UpdateCheck bool
	// StuckClaimDays is how long a claimed clip goes without edits before the
	// admin panel lists it as stuck.
	StuckClaimDays int
	// SamModel is the Hugging Face model GPU workers track with, unless a
	// worker pins its own.
	SamModel string
	// BackupHour is the hour of the day scheduled backups run, or -1 when
	// they're off. BackupKeep is how many scheduled backups are kept.
	BackupHour int
	BackupKeep int
}

// Lookup returns a setting's raw value, or false when nothing sets it and
// its default applies.
type Lookup func(key string) (string, bool)

// Env looks settings up in the environment. An empty variable counts as
// unset, because Compose and .env files pass every variable they list
// whether or not it has a value.
func Env(key string) (string, bool) {
	v := os.Getenv(key)
	return v, v != ""
}

// FieldError is a setting with a value that can't be used.
type FieldError struct {
	Key     string
	Message string
}

func (e *FieldError) Error() string {
	return e.Key + ": " + e.Message
}

func invalid(key, format string, args ...any) *FieldError {
	return &FieldError{Key: key, Message: fmt.Sprintf(format, args...)}
}

func Load() (Config, error) {
	return Parse(Env)
}

// Parse builds the config from lookup, falling back to each setting's
// default, and checks that the settings work together.
func Parse(lookup Lookup) (Config, error) {
	p := parser{lookup: lookup}
	cfg := Config{
		Addr:                  p.str("KRILL_API_ADDR"),
		DatabaseURL:           p.str("KRILL_DATABASE_URL"),
		S3Endpoint:            p.str("KRILL_S3_ENDPOINT"),
		S3PublicEndpoint:      p.str("KRILL_S3_PUBLIC_ENDPOINT"),
		S3Bucket:              p.str("KRILL_S3_BUCKET"),
		S3AccessKey:           p.str("KRILL_S3_ACCESS_KEY"),
		S3SecretKey:           p.str("KRILL_S3_SECRET_KEY"),
		WebDir:                p.str("KRILL_WEB_DIR"),
		PublicURL:             strings.TrimRight(p.str("KRILL_PUBLIC_URL"), "/"),
		TeamName:              p.str("KRILL_TEAM_NAME"),
		AllowSignup:           p.bool("KRILL_ALLOW_SIGNUP"),
		SignupEmailDomain:     strings.ToLower(p.str("KRILL_SIGNUP_EMAIL_DOMAIN")),
		EmailVerification:     EmailVerification(p.choice("KRILL_EMAIL_VERIFICATION")),
		ResendAPIKey:          p.str("KRILL_RESEND_API_KEY"),
		EmailFrom:             p.str("KRILL_EMAIL_FROM"),
		SlackClientID:         p.str("KRILL_SLACK_CLIENT_ID"),
		SlackClientSecret:     p.str("KRILL_SLACK_CLIENT_SECRET"),
		SlackTeamID:           p.str("KRILL_SLACK_TEAM_ID"),
		WorkerToken:           p.str("KRILL_WORKER_TOKEN"),
		EmbedAtIngest:         p.bool("KRILL_EMBED_AT_INGEST"),
		ClientIPHeader:        p.str("KRILL_CLIENT_IP_HEADER"),
		GoldReviewThreshold:   p.float("KRILL_GOLD_REVIEW_THRESHOLD"),
		GoldReviewMinAttempts: p.int("KRILL_GOLD_REVIEW_MIN_ATTEMPTS"),
		UpdateCheck:           p.bool("KRILL_UPDATE_CHECK"),
		StuckClaimDays:        p.int("KRILL_STUCK_CLAIM_DAYS"),
		SamModel:              p.choice("KRILL_SAM_MODEL"),
		BackupHour:            p.backupHour("KRILL_BACKUP_SCHEDULE"),
		BackupKeep:            p.int("KRILL_BACKUP_KEEP"),
	}
	if p.err != nil {
		return Config{}, p.err
	}
	if cfg.TeamName == "" {
		cfg.TeamName = defaults["KRILL_TEAM_NAME"]
	}
	if cfg.S3PublicEndpoint == "" {
		cfg.S3PublicEndpoint = cfg.S3Endpoint
	}
	cfg.LogEmails = cfg.ResendAPIKey == "" && isLocal(cfg.PublicURL)
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.DatabaseURL == "" {
		return invalid("KRILL_DATABASE_URL", "is required")
	}
	if c.GoldReviewThreshold < 0 || c.GoldReviewThreshold > 1 {
		return invalid("KRILL_GOLD_REVIEW_THRESHOLD", "must be between 0 and 1")
	}
	if c.GoldReviewMinAttempts < 1 {
		return invalid("KRILL_GOLD_REVIEW_MIN_ATTEMPTS", "must be at least 1")
	}
	if c.StuckClaimDays < 1 {
		return invalid("KRILL_STUCK_CLAIM_DAYS", "must be at least 1")
	}
	if strings.HasPrefix(c.PublicURL, "https://") && !strings.HasPrefix(c.S3PublicEndpoint, "https://") {
		return invalid("KRILL_S3_PUBLIC_ENDPOINT", "must be HTTPS when KRILL_PUBLIC_URL is")
	}
	if c.SlackClientID != "" {
		if c.SlackClientSecret == "" {
			return invalid("KRILL_SLACK_CLIENT_SECRET", "is required with a Slack client ID")
		}
		if c.SlackTeamID == "" {
			return invalid("KRILL_SLACK_TEAM_ID", "is required with a Slack client ID")
		}
	}
	if c.ResendAPIKey != "" && c.EmailFrom == "" {
		return invalid("KRILL_EMAIL_FROM", "is required with a Resend API key")
	}
	if c.EmailVerification == VerifyCode && !c.CanEmail() {
		return invalid("KRILL_RESEND_API_KEY", "is required to send verification codes")
	}
	if c.BackupKeep < 1 {
		return invalid("KRILL_BACKUP_KEEP", "must be at least 1")
	}
	return nil
}

// CanEmail reports whether the API can send email, for real or to its log.
func (c Config) CanEmail() bool {
	return c.ResendAPIKey != "" || c.LogEmails
}

func isLocal(publicURL string) bool {
	u, err := url.Parse(publicURL)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// parser keeps the first error so Parse reads as a list of fields.
type parser struct {
	lookup Lookup
	err    error
}

func (p *parser) raw(key string) string {
	if _, ok := byKey[key]; !ok {
		panic("config: " + key + " is not a setting")
	}
	if v, ok := p.lookup(key); ok {
		return strings.TrimSpace(v)
	}
	return defaults[key]
}

func (p *parser) fail(err *FieldError) {
	if p.err == nil {
		p.err = err
	}
}

func (p *parser) str(key string) string {
	return p.raw(key)
}

func (p *parser) bool(key string) bool {
	v, err := strconv.ParseBool(p.raw(key))
	if err != nil {
		p.fail(invalid(key, "must be true or false"))
	}
	return v
}

func (p *parser) int(key string) int {
	v, err := strconv.Atoi(p.raw(key))
	if err != nil {
		p.fail(invalid(key, "must be a whole number"))
	}
	return v
}

func (p *parser) float(key string) float64 {
	v, err := strconv.ParseFloat(p.raw(key), 64)
	if err != nil {
		p.fail(invalid(key, "must be a number"))
	}
	return v
}

func (p *parser) choice(key string) string {
	v := strings.ToLower(p.raw(key))
	choices := byKey[key].Choices
	values := make([]string, len(choices))
	for i, c := range choices {
		if v == c.Value {
			return v
		}
		values[i] = c.Value
	}
	p.fail(invalid(key, "must be %s", orList(values)))
	return v
}

func orList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " or " + items[1]
	}
	return strings.Join(items[:len(items)-1], ", ") + ", or " + items[len(items)-1]
}

func (p *parser) backupHour(key string) int {
	v := p.choice(key)
	if v == BackupOff {
		return -1
	}
	h, _ := strconv.Atoi(v)
	return h
}
