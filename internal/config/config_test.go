package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProfileName(t *testing.T) {
	t.Setenv("PLANFIX_PROFILE", "")
	tests := []struct {
		name    string
		flag    string
		env     string
		current string
		want    string
	}{
		{"flag wins", "work", "envp", "curp", "work"},
		{"env wins over current", "", "envp", "curp", "envp"},
		{"current wins over default", "", "", "curp", "curp"},
		{"fallback default", "", "", "", "default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv("PLANFIX_PROFILE", tt.env)
			}
			c := &Config{CurrentProfile: tt.current}
			got := ResolveProfileName(tt.flag, c)
			if got != tt.want {
				t.Fatalf("ResolveProfileName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolvePrecedence(t *testing.T) {
	t.Setenv("PLANFIX_DOMAIN", "env.planfix.ru")
	t.Setenv("PLANFIX_TOKEN", "env-token")

	c := &Config{Profiles: map[string]*Profile{
		"default": {Domain: "file.planfix.ru", Token: "file-token"},
	}}
	p, err := Resolve(c, "default")
	if err != nil {
		t.Fatal(err)
	}
	if p.Domain != "env.planfix.ru" || p.Token != "env-token" {
		t.Fatalf("env should override file, got %+v", p)
	}

	t.Setenv("PLANFIX_DOMAIN", "")
	t.Setenv("PLANFIX_TOKEN", "")
	p, err = Resolve(c, "default")
	if err != nil {
		t.Fatal(err)
	}
	if p.Domain != "file.planfix.ru" {
		t.Fatalf("file values expected, got %+v", p)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")

	c := &Config{
		CurrentProfile: "default",
		Profiles: map[string]*Profile{
			"default": {
				Domain:  "example.planfix.ru",
				Token:   "secret",
				Worklog: &WorklogMeta{DataTagID: 123, FieldDate: 456, FieldTime: 789},
			},
		},
	}
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %v, want 0600", info.Mode().Perm())
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	p := got.Profiles["default"]
	if p.Domain != "example.planfix.ru" || p.Token != "secret" || p.Worklog.DataTagID != 123 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestResolveMissingProfile(t *testing.T) {
	c := &Config{Profiles: map[string]*Profile{}}
	if _, err := Resolve(c, "nope"); err == nil {
		t.Fatal("expected error for missing profile")
	}
}
