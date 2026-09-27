package auth

import (
	"net/http"
	"testing"
)

func TestPassword(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		password string
		want     bool
	}{
		{"correct horse", true},
		{"correct horsE", false},
		{"", false},
	} {
		ok, err := CheckPassword(hash, tt.password)
		if err != nil {
			t.Fatal(err)
		}
		if ok != tt.want {
			t.Errorf("CheckPassword(%q) = %v, want %v", tt.password, ok, tt.want)
		}
	}
	if _, err := CheckPassword("$2a$10$bcrypt", "x"); err == nil {
		t.Error("expected error for a non-argon2id hash")
	}
}

func TestNormalizeProfile(t *testing.T) {
	p, err := NormalizeProfile("  Ada Lovelace ", " Ada.L ", " Ada@Wisc.EDU ")
	if err != nil {
		t.Fatal(err)
	}
	if p != (Profile{Name: "Ada Lovelace", Username: "ada.l", Email: "ada@wisc.edu"}) {
		t.Errorf("got %+v", p)
	}

	for _, tt := range []struct{ name, username, email string }{
		{"", "ada", "ada@wisc.edu"},
		{"Ada", "a", "ada@wisc.edu"},
		{"Ada", "-ada", "ada@wisc.edu"},
		{"Ada", "ada lovelace", "ada@wisc.edu"},
		{"Ada", "ada", "ada"},
		{"Ada", "ada", "ada@localhost"},
		{"Ada", "ada", "Ada <ada@wisc.edu>"},
	} {
		if _, err := NormalizeProfile(tt.name, tt.username, tt.email); err == nil {
			t.Errorf("NormalizeProfile(%q, %q, %q) accepted", tt.name, tt.username, tt.email)
		}
	}
}

func TestInEmailDomain(t *testing.T) {
	for _, tt := range []struct {
		email, domain string
		ok            bool
	}{
		{"ada@wisc.edu", "wisc.edu", true},
		{"ada@gmail.com", "wisc.edu", false},
		{"ada@evilwisc.edu", "wisc.edu", false},
		{"ada@wisc.edu.evil.com", "wisc.edu", false},
		{"ada@cs.wisc.edu", "wisc.edu", false},
		{"ada@gmail.com", "", true},
	} {
		if got := InEmailDomain(tt.email, tt.domain); got != tt.ok {
			t.Errorf("InEmailDomain(%q, %q) = %v, want %v", tt.email, tt.domain, got, tt.ok)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	for _, tt := range []struct {
		password string
		ok       bool
	}{
		{"Sh0rt!", false},
		{"lowercaseonly", false},
		{"lower12345", false},
		{"Lower12345", true},
		{"lower-case 1", true},
		{"UPPER CASE!", false},
		{"UPPER CASE 1!", true},
	} {
		if err := ValidatePassword(tt.password); (err == nil) != tt.ok {
			t.Errorf("ValidatePassword(%q) = %v, want ok %v", tt.password, err, tt.ok)
		}
	}
}

func TestClientIP(t *testing.T) {
	cf := http.Header{"Cf-Connecting-Ip": {" 203.0.113.7 "}}
	for _, tt := range []struct {
		name   string
		h      http.Header
		header string
		want   string
	}{
		{"peer without proxy", nil, "", "10.0.0.2"},
		{"spoofed header is ignored", cf, "", "10.0.0.2"},
		{"trusted header", cf, "CF-Connecting-IP", "203.0.113.7"},
		{"trusted header missing", http.Header{}, "CF-Connecting-IP", "10.0.0.2"},
	} {
		if got := clientIP("10.0.0.2:5123", tt.h, tt.header); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
