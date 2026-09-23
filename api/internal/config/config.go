package config

import (
	"errors"
	"os"
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
}

func Load() (Config, error) {
	cfg := Config{
		Addr:             getenv("KRILL_API_ADDR", ":8080"),
		DatabaseURL:      os.Getenv("KRILL_DATABASE_URL"),
		S3Endpoint:       getenv("KRILL_S3_ENDPOINT", "http://localhost:9000"),
		S3PublicEndpoint: os.Getenv("KRILL_S3_PUBLIC_ENDPOINT"),
		S3Bucket:         getenv("KRILL_S3_BUCKET", "krill"),
		S3AccessKey:      os.Getenv("KRILL_S3_ACCESS_KEY"),
		S3SecretKey:      os.Getenv("KRILL_S3_SECRET_KEY"),
		WebDir:           os.Getenv("KRILL_WEB_DIR"),
	}
	if cfg.S3PublicEndpoint == "" {
		cfg.S3PublicEndpoint = cfg.S3Endpoint
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
