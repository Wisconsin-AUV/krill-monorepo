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
	TeamName    string
	// Slack sign-in is enabled when SlackClientID is set.
	SlackClientID     string
	SlackClientSecret string
	SlackTeamID       string
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
	}
	if cfg.TeamName = strings.TrimSpace(os.Getenv("KRILL_TEAM_NAME")); cfg.TeamName == "" {
		cfg.TeamName = "Wisconsin Autonomous Underwater Vehicles"
	}
	var err error
	if cfg.AllowSignup, err = strconv.ParseBool(getenv("KRILL_ALLOW_SIGNUP", "true")); err != nil {
		return Config{}, fmt.Errorf("KRILL_ALLOW_SIGNUP: %w", err)
	}
	if cfg.S3PublicEndpoint == "" {
		cfg.S3PublicEndpoint = cfg.S3Endpoint
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
