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

func TestLoadAllowSignup(t *testing.T) {
	t.Setenv("KRILL_DATABASE_URL", "postgres://localhost/krill")
	t.Setenv("KRILL_ALLOW_SIGNUP", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AllowSignup {
		t.Error("AllowSignup = true, want false")
	}
	t.Setenv("KRILL_ALLOW_SIGNUP", "maybe")
	if _, err := Load(); err == nil {
		t.Error("expected error for an invalid KRILL_ALLOW_SIGNUP")
	}
}
