package auth

import "testing"

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

func TestHashPasswordSalts(t *testing.T) {
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")
	if a == b {
		t.Error("two hashes of the same password are equal")
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

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("short"); err == nil {
		t.Error("accepted a short password")
	}
	if err := ValidatePassword("longenough"); err != nil {
		t.Error(err)
	}
}
