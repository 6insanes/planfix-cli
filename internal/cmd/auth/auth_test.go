package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/6insanes/planfix-cli/internal/config"
	"github.com/6insanes/planfix-cli/internal/planfix"
)

func defaultName() string { return "default" }

// run executes a fresh auth command tree and returns its output.
func run(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	return runAs(t, defaultName, stdin, args...)
}

func runAs(t *testing.T, resolveName func() string, stdin string, args ...string) (string, error) {
	t.Helper()
	c := NewCmd(resolveName)
	buf := new(bytes.Buffer)
	c.SetOut(buf)
	c.SetErr(buf)
	c.SetIn(strings.NewReader(stdin))
	c.SetArgs(args)
	err := c.Execute()
	return buf.String(), err
}

// setConfigEnv points the config file at path and clears credential env overrides.
func setConfigEnv(t *testing.T, path string) {
	t.Helper()
	t.Setenv("PLANFIX_CONFIG", path)
	t.Setenv("PLANFIX_PROFILE", "")
	t.Setenv("PLANFIX_DOMAIN", "")
	t.Setenv("PLANFIX_TOKEN", "")
}

// stubClient swaps openClient for one pointing at a fake Planfix server.
func stubClient(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	prev := openClient
	openClient = func(domain, token string) (*planfix.Client, error) {
		c, err := planfix.New(domain, token)
		if err != nil {
			return nil, err
		}
		c.BaseURL = srv.URL
		return c, nil
	}
	t.Cleanup(func() { openClient = prev })
}

func TestMask(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "****"},
		{"a", "****"},
		{"abcd", "****"},
		{"abcde", "****"},
		{"tokensecret1234", "to****34"},
		{"日本語テストの秘密", "日本****秘密"}, // 8 runes, 24 bytes
		{"日本語", "****"},           // 3 runes, 9 bytes: too few runes to reveal
	}
	for _, tt := range tests {
		if got := mask(tt.in); got != tt.want {
			t.Errorf("mask(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLoginWritesProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	setConfigEnv(t, path)

	out, err := run(t, "", "login", "--domain", "example.planfix.ru", "--token", "tokensecret1234")
	if err != nil {
		t.Fatalf("login error = %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config file: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("config mode = %o, want 600", perm)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	p := cfg.Profiles["default"]
	if p == nil {
		t.Fatal("profile \"default\" not written")
	}
	if p.Domain != "example.planfix.ru" || p.Token != "tokensecret1234" {
		t.Errorf("profile = %+v, want domain/token stored verbatim", p)
	}
	if cfg.CurrentProfile != "default" {
		t.Errorf("CurrentProfile = %q, want \"default\"", cfg.CurrentProfile)
	}
	if !strings.Contains(out, "to****34") {
		t.Errorf("output %q missing masked token", out)
	}
	if strings.Contains(out, "tokensecret1234") {
		t.Errorf("output %q leaks the raw token", out)
	}
}

func TestLoginPromptsWhenFlagsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	setConfigEnv(t, path)

	out, err := run(t, "example.planfix.ru\ntokensecret9999\n", "login")
	if err != nil {
		t.Fatalf("login error = %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	p := cfg.Profiles["default"]
	if p == nil {
		t.Fatal("profile \"default\" not written")
	}
	if p.Domain != "example.planfix.ru" || p.Token != "tokensecret9999" {
		t.Errorf("profile = %+v, want prompted values", p)
	}
	if !strings.Contains(out, "Domain") || !strings.Contains(out, "Token") {
		t.Errorf("output %q missing prompts", out)
	}
	if strings.Contains(out, "tokensecret9999") {
		t.Errorf("output %q leaks the raw token", out)
	}
}

func TestLoginRefusesOverwriteWithoutForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	setConfigEnv(t, path)
	seed := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]*config.Profile{
			"default": {
				Domain:  "old.example.ru",
				Token:   "oldtoken12345",
				Worklog: &config.WorklogMeta{DataTagID: 7, FieldDate: 456},
			},
		},
	}
	if err := config.Save(path, seed); err != nil {
		t.Fatal(err)
	}

	_, err := run(t, "", "login", "--domain", "new.example.ru", "--token", "newtoken12345")
	if err == nil {
		t.Fatal("login error = nil, want refusal to overwrite")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error %q does not mention --force", err)
	}
	cfg, _ := config.Load(path)
	if got := cfg.Profiles["default"].Token; got != "oldtoken12345" {
		t.Errorf("token = %q after refused login, want unchanged", got)
	}

	if _, err := run(t, "", "login", "--force", "--domain", "new.example.ru", "--token", "newtoken12345"); err != nil {
		t.Fatalf("login --force error = %v", err)
	}
	cfg, _ = config.Load(path)
	p := cfg.Profiles["default"]
	if p.Domain != "new.example.ru" || p.Token != "newtoken12345" {
		t.Errorf("profile after --force = %+v", p)
	}
	if p.Worklog == nil || p.Worklog.DataTagID != 7 {
		t.Errorf("worklog meta = %+v, want preserved across --force", p.Worklog)
	}
}

func TestLoginNameFlagOverridesResolveName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	setConfigEnv(t, path)

	_, err := runAs(t, func() string { return "active" }, "",
		"login", "--name", "work", "--domain", "example.planfix.ru", "--token", "tokensecret1234")
	if err != nil {
		t.Fatalf("login error = %v", err)
	}
	cfg, _ := config.Load(path)
	if cfg.Profiles["work"] == nil {
		t.Error("profile \"work\" not written")
	}
	if _, ok := cfg.Profiles["active"]; ok {
		t.Error("resolveName fallback profile written despite --name")
	}
	if cfg.CurrentProfile != "work" {
		t.Errorf("CurrentProfile = %q, want \"work\"", cfg.CurrentProfile)
	}
}

func TestStatusShowsProfileAndPingOK(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	setConfigEnv(t, path)
	seed := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]*config.Profile{
			"default": {Domain: "example.planfix.ru", Token: "tokensecret1234"},
		},
	}
	if err := config.Save(path, seed); err != nil {
		t.Fatal(err)
	}
	var gotAuth, gotPath string
	var gotDomain, gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"result":"success"}`))
	}))
	t.Cleanup(srv.Close)
	prev := openClient
	openClient = func(domain, token string) (*planfix.Client, error) {
		gotDomain, gotToken = domain, token
		c, err := planfix.New(domain, token)
		if err != nil {
			return nil, err
		}
		c.BaseURL = srv.URL
		return c, nil
	}
	t.Cleanup(func() { openClient = prev })

	out, err := run(t, "", "status")
	if err != nil {
		t.Fatalf("status error = %v", err)
	}
	for _, want := range []string{"profile: default", "domain:", "example.planfix.ru", "to****34", "ping:", "OK"} {
		if !strings.Contains(out, want) {
			t.Errorf("status output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "tokensecret1234") {
		t.Errorf("status output leaks the raw token:\n%s", out)
	}
	if gotPath != "/ping" {
		t.Errorf("ping path = %q, want /ping", gotPath)
	}
	if gotAuth != "Bearer tokensecret1234" {
		t.Errorf("Authorization = %q, want bearer token", gotAuth)
	}
	if gotDomain != "example.planfix.ru" || gotToken != "tokensecret1234" {
		t.Errorf("openClient args = %q %q, want profile domain/token", gotDomain, gotToken)
	}
}

func TestStatusPingFailureCarriesHint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	setConfigEnv(t, path)
	seed := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]*config.Profile{
			"default": {Domain: "example.planfix.ru", Token: "badtoken"},
		},
	}
	if err := config.Save(path, seed); err != nil {
		t.Fatal(err)
	}
	stubClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"failure","code":1,"message":"token not found"}`))
	})

	out, err := run(t, "", "status")
	if err == nil {
		t.Fatal("status error = nil, want ping failure")
	}
	if !strings.Contains(err.Error(), "auth login") {
		t.Errorf("error %q missing hint", err)
	}
	if !strings.Contains(out, "profile: default") {
		t.Errorf("profile info should print before the ping failure:\n%s", out)
	}
}

func TestStatusRejectsEmptyCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	setConfigEnv(t, path)
	seed := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]*config.Profile{
			"default": {Domain: "example.planfix.ru", Token: ""},
		},
	}
	if err := config.Save(path, seed); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "", "status")
	if err == nil {
		t.Fatal("status error = nil, want rejection of empty credentials")
	}
	if !strings.Contains(err.Error(), "empty domain or token") {
		t.Errorf("error %q does not mention empty credentials", err)
	}
	if !strings.Contains(err.Error(), "auth login") {
		t.Errorf("error %q does not suggest `planfix auth login`", err)
	}
	if strings.Contains(out, "profile:") {
		t.Errorf("status header printed before credential rejection:\n%s", out)
	}
}

func TestLogoutRemovesProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	setConfigEnv(t, path)
	seed := &config.Config{
		CurrentProfile: "default",
		Profiles: map[string]*config.Profile{
			"default": {Domain: "example.planfix.ru", Token: "tokensecret1234"},
			"work":    {Domain: "work.example.ru", Token: "worktoken12345"},
		},
	}
	if err := config.Save(path, seed); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, "", "logout")
	if err != nil {
		t.Fatalf("logout error = %v", err)
	}
	if !strings.Contains(out, "default") {
		t.Errorf("logout output %q does not name the profile", out)
	}
	cfg, _ := config.Load(path)
	if _, ok := cfg.Profiles["default"]; ok {
		t.Error("profile \"default\" still present after logout")
	}
	if cfg.Profiles["work"] == nil {
		t.Error("unrelated profile \"work\" was removed")
	}
	if cfg.CurrentProfile != "" {
		t.Errorf("CurrentProfile = %q, want reset to empty", cfg.CurrentProfile)
	}

	if _, err := run(t, "", "logout"); err == nil {
		t.Error("second logout error = nil, want not-found failure")
	}
}

func TestNewCmdHelpListsSubcommands(t *testing.T) {
	out, err := run(t, "", "--help")
	if err != nil {
		t.Fatalf("auth --help error = %v", err)
	}
	for _, sub := range []string{"login", "status", "logout"} {
		if !strings.Contains(out, sub) {
			t.Errorf("auth help missing %q:\n%s", sub, out)
		}
	}
}
