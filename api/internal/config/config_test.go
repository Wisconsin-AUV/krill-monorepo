package config

import (
	"errors"
	"testing"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("KRILL_DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when KRILL_DATABASE_URL is empty")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("KRILL_DATABASE_URL", "postgres://localhost/krill")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", cfg.Addr)
	}
}

func TestLoadPublicEndpointFallsBack(t *testing.T) {
	t.Setenv("KRILL_DATABASE_URL", "postgres://localhost/krill")
	t.Setenv("KRILL_S3_ENDPOINT", "http://minio:9000")
	t.Setenv("KRILL_S3_PUBLIC_ENDPOINT", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.S3PublicEndpoint != "http://minio:9000" {
		t.Errorf("S3PublicEndpoint = %q, want http://minio:9000", cfg.S3PublicEndpoint)
	}
}

func TestLoadSlackNeedsTeam(t *testing.T) {
	t.Setenv("KRILL_DATABASE_URL", "postgres://localhost/krill")
	t.Setenv("KRILL_SLACK_CLIENT_ID", "id")
	t.Setenv("KRILL_SLACK_CLIENT_SECRET", "secret")
	t.Setenv("KRILL_SLACK_TEAM_ID", "")
	if _, err := Load(); err == nil {
		t.Error("expected error when KRILL_SLACK_TEAM_ID is missing")
	}
}

func TestLoadHTTPSNeedsHTTPSEndpoint(t *testing.T) {
	t.Setenv("KRILL_DATABASE_URL", "postgres://localhost/krill")
	t.Setenv("KRILL_PUBLIC_URL", "https://krill.example.com")
	t.Setenv("KRILL_S3_PUBLIC_ENDPOINT", "http://localhost:9000")
	if _, err := Load(); err == nil {
		t.Error("expected error when the app is HTTPS and frames are HTTP")
	}
	t.Setenv("KRILL_S3_PUBLIC_ENDPOINT", "https://s3.krill.example.com")
	if _, err := Load(); err != nil {
		t.Error(err)
	}
}

func TestLoadVerifyNeedsResendOffLocalhost(t *testing.T) {
	t.Setenv("KRILL_DATABASE_URL", "postgres://localhost/krill")
	t.Setenv("KRILL_EMAIL_VERIFICATION", "verify")
	t.Setenv("KRILL_RESEND_API_KEY", "")
	t.Setenv("KRILL_PUBLIC_URL", "http://localhost:5173")
	if cfg, err := Load(); err != nil || !cfg.LogEmails {
		t.Errorf("localhost should log emails: %v", err)
	}
	t.Setenv("KRILL_PUBLIC_URL", "http://krill.example.com")
	if _, err := Load(); err == nil {
		t.Error("expected error when verifying emails with no way to send them")
	}
	t.Setenv("KRILL_RESEND_API_KEY", "re_key")
	t.Setenv("KRILL_EMAIL_FROM", "Krill <noreply@krill.example.com>")
	if cfg, err := Load(); err != nil || cfg.LogEmails {
		t.Errorf("Resend should send emails: %v", err)
	}
}

func TestLoadRejectsPipelineSettingsOutOfRange(t *testing.T) {
	tests := []struct{ key, value string }{
		{"KRILL_EXPORT_VAL_FRACTION", "0.6"},
		{"KRILL_EXPORT_STRIDE", "0"},
		{"KRILL_EXPORT_STRIDE", "101"},
		{"KRILL_EXPORT_DEDUP_DISTANCE", "17"},
		{"KRILL_RETRAIN_HOUR", "24"},
		{"KRILL_RETRAIN_IMAGE_SIZE", "650"},
		{"KRILL_RETRAIN_BATCH", "0"},
		{"KRILL_PRELABEL_CONFIDENCE", "1"},
	}
	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			t.Setenv("KRILL_DATABASE_URL", "postgres://localhost/krill")
			t.Setenv(tt.key, tt.value)
			_, err := Load()
			var fe *FieldError
			if !errors.As(err, &fe) || fe.Key != tt.key {
				t.Errorf("err = %v, want one about %s", err, tt.key)
			}
		})
	}
}

func TestLoadRetrainHour(t *testing.T) {
	t.Setenv("KRILL_DATABASE_URL", "postgres://localhost/krill")
	for value, want := range map[string]int{"": -1, "off": -1, "0": 0, "23": 23} {
		t.Setenv("KRILL_RETRAIN_HOUR", value)
		cfg, err := Load()
		if err != nil || cfg.RetrainHour != want {
			t.Errorf("KRILL_RETRAIN_HOUR=%q: hour %d, %v, want %d", value, cfg.RetrainHour, err, want)
		}
	}
}
