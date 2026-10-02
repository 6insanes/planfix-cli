package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveProfileName(t *testing.T) {
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
			t.Setenv("PLANFIX_PROFILE", tt.env)
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
	stored := c.Profiles["default"]
	if stored.Domain != "file.planfix.ru" || stored.Token != "file-token" {
		t.Fatalf("stored profile must not be mutated by Resolve, got %+v", stored)
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
	// Clear env so the error path isn't masked by env-only synthesis.
	t.Setenv("PLANFIX_DOMAIN", "")
	t.Setenv("PLANFIX_TOKEN", "")
	c := &Config{Profiles: map[string]*Profile{}}
	if _, err := Resolve(c, "nope"); err == nil {
		t.Fatal("expected error for missing profile")
	}
}

func TestResolveNilProfile(t *testing.T) {
	// A YAML entry like `default:` unmarshals to a nil pointer.
	t.Setenv("PLANFIX_DOMAIN", "")
	t.Setenv("PLANFIX_TOKEN", "")
	c := &Config{Profiles: map[string]*Profile{"default": nil}}
	if _, err := Resolve(c, "default"); err == nil {
		t.Fatal("expected error for nil profile entry")
	}
}

func TestResolveEnvOnlyNoProfile(t *testing.T) {
	t.Setenv("PLANFIX_DOMAIN", "env.planfix.ru")
	t.Setenv("PLANFIX_TOKEN", "env-token")
	c := &Config{Profiles: map[string]*Profile{}}
	p, err := Resolve(c, "default")
	if err != nil {
		t.Fatalf("env-only resolve must succeed, got %v", err)
	}
	if p.Domain != "env.planfix.ru" || p.Token != "env-token" {
		t.Fatalf("want env credentials, got %+v", p)
	}
}

func TestResolveEnvIncompleteNoProfile(t *testing.T) {
	t.Setenv("PLANFIX_DOMAIN", "env.planfix.ru")
	t.Setenv("PLANFIX_TOKEN", "")
	c := &Config{Profiles: map[string]*Profile{}}
	if _, err := Resolve(c, "default"); err == nil {
		t.Fatal("expected error when env is incomplete and profile is missing")
	}
}

func TestLoadMissingFile(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "absent.yml"))
	if err != nil {
		t.Fatalf("missing file must not error: %v", err)
	}
	if c == nil || c.CurrentProfile != "" || len(c.Profiles) != 0 {
		t.Fatalf("want empty config, got %+v", c)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("profiles: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected parse error for invalid YAML")
	}
}

func TestResolvePath(t *testing.T) {
	t.Setenv("PLANFIX_CONFIG", "/tmp/custom-planfix.yml")
	if got := ResolvePath(); got != "/tmp/custom-planfix.yml" {
		t.Fatalf("ResolvePath() = %q, want env override", got)
	}

	t.Setenv("PLANFIX_CONFIG", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("UserHomeDir: %v", err)
	}
	want := filepath.Join(home, ".config", "planfix", "config.yml")
	if got := ResolvePath(); got != want {
		t.Fatalf("ResolvePath() = %q, want %q", got, want)
	}
}

func TestSaveResetsModeWithStaleTmp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	// A pre-existing world-readable tmp file must not leak its mode into config.yml.
	if err := os.WriteFile(path+".tmp", []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, &Config{Profiles: map[string]*Profile{}}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %v, want 0600", info.Mode().Perm())
	}
}
