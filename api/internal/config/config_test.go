package config

import "testing"

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
