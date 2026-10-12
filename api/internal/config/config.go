package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	TeamName          string
	// Slack sign-in is enabled when SlackClientID is set.
	SlackClientID     string
	SlackClientSecret string
	SlackTeamID       string
	// WorkerToken authenticates the GPU worker. Empty turns worker RPCs off.
	WorkerToken string
	// ClientIPHeader is trusted for the client's address when set. Only set it
	// when every request comes through the proxy that writes it.
	ClientIPHeader string
	// A labeler's work goes to review while their mean score over recent gold
	// checks is below GoldReviewThreshold, once they have answered at least
	// GoldReviewMinAttempts checks.
	GoldReviewThreshold   float64
	GoldReviewMinAttempts int
}

func Load() (Config, error) {
	cfg := Config{
		Addr:              getenv("KRILL_API_ADDR", ":8080"),
		DatabaseURL:       os.Getenv("KRILL_DATABASE_URL"),
		S3Endpoint:        getenv("KRILL_S3_ENDPOINT", "http://localhost:9000"),
		S3PublicEndpoint:  os.Getenv("KRILL_S3_PUBLIC_ENDPOINT"),
		S3Bucket:          getenv("KRILL_S3_BUCKET", "krill"),
		S3AccessKey:       os.Getenv("KRILL_S3_ACCESS_KEY"),
		S3SecretKey:       os.Getenv("KRILL_S3_SECRET_KEY"),
		WebDir:            os.Getenv("KRILL_WEB_DIR"),
		PublicURL:         strings.TrimRight(getenv("KRILL_PUBLIC_URL", "http://localhost:8080"), "/"),
		SlackClientID:     os.Getenv("KRILL_SLACK_CLIENT_ID"),
		SlackClientSecret: os.Getenv("KRILL_SLACK_CLIENT_SECRET"),
		SlackTeamID:       os.Getenv("KRILL_SLACK_TEAM_ID"),
		WorkerToken:       os.Getenv("KRILL_WORKER_TOKEN"),
		SignupEmailDomain: strings.ToLower(strings.TrimSpace(getenv("KRILL_SIGNUP_EMAIL_DOMAIN", "wisc.edu"))),
		ClientIPHeader:    strings.TrimSpace(os.Getenv("KRILL_CLIENT_IP_HEADER")),
	}
	if cfg.TeamName = strings.TrimSpace(os.Getenv("KRILL_TEAM_NAME")); cfg.TeamName == "" {
		cfg.TeamName = "Wisconsin Autonomous Underwater Vehicles"
	}
	var err error
	if cfg.AllowSignup, err = strconv.ParseBool(getenv("KRILL_ALLOW_SIGNUP", "true")); err != nil {
		return Config{}, fmt.Errorf("KRILL_ALLOW_SIGNUP: %w", err)
	}
	if cfg.GoldReviewThreshold, err = strconv.ParseFloat(getenv("KRILL_GOLD_REVIEW_THRESHOLD", "0.7"), 64); err != nil {
		return Config{}, fmt.Errorf("KRILL_GOLD_REVIEW_THRESHOLD: %w", err)
	}
	if cfg.GoldReviewThreshold < 0 || cfg.GoldReviewThreshold > 1 {
		return Config{}, errors.New("KRILL_GOLD_REVIEW_THRESHOLD must be between 0 and 1")
	}
	if cfg.GoldReviewMinAttempts, err = strconv.Atoi(getenv("KRILL_GOLD_REVIEW_MIN_ATTEMPTS", "3")); err != nil {
		return Config{}, fmt.Errorf("KRILL_GOLD_REVIEW_MIN_ATTEMPTS: %w", err)
	}
	if cfg.GoldReviewMinAttempts < 1 {
		return Config{}, errors.New("KRILL_GOLD_REVIEW_MIN_ATTEMPTS must be at least 1")
	}
	if cfg.S3PublicEndpoint == "" {
		cfg.S3PublicEndpoint = cfg.S3Endpoint
	}
	if strings.HasPrefix(cfg.PublicURL, "https://") && !strings.HasPrefix(cfg.S3PublicEndpoint, "https://") {
		return Config{}, errors.New("KRILL_S3_PUBLIC_ENDPOINT must be HTTPS when KRILL_PUBLIC_URL is")
	}
	if cfg.SlackClientID != "" && (cfg.SlackClientSecret == "" || cfg.SlackTeamID == "") {
		return Config{}, errors.New("KRILL_SLACK_CLIENT_SECRET and KRILL_SLACK_TEAM_ID are required with KRILL_SLACK_CLIENT_ID")
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("KRILL_DATABASE_URL is required")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
