package config

import (
	"errors"
	"os"
)

type Config struct {
	Addr        string
	DatabaseURL string
	S3Endpoint  string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
}

func Load() (Config, error) {
	cfg := Config{
		Addr:        getenv("KRILL_API_ADDR", ":8080"),
		DatabaseURL: os.Getenv("KRILL_DATABASE_URL"),
		S3Endpoint:  getenv("KRILL_S3_ENDPOINT", "http://localhost:9000"),
		S3Bucket:    getenv("KRILL_S3_BUCKET", "krill"),
		S3AccessKey: os.Getenv("KRILL_S3_ACCESS_KEY"),
		S3SecretKey: os.Getenv("KRILL_S3_SECRET_KEY"),
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
