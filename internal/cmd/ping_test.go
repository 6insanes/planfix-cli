package cmd

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"planfix-cli/internal/planfix"
)

func newTestClient(t *testing.T, srv *httptest.Server) *planfix.Client {
	t.Helper()
	c, err := planfix.New("example.com", "secret-token")
	if err != nil {
		t.Fatalf("planfix.New() error = %v", err)
	}
	c.BaseURL = srv.URL
	return c
}

func TestDoPingPrintsOK(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"result":"success"}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	if err := doPing(context.Background(), newTestClient(t, srv), &buf); err != nil {
		t.Fatalf("doPing() error = %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "OK" {
		t.Errorf("output = %q, want %q", got, "OK")
	}
	if gotMethod != http.MethodGet || gotPath != "/ping" {
		t.Errorf("request = %s %s, want GET /ping", gotMethod, gotPath)
	}
	if want := "Bearer secret-token"; gotAuth != want {
		t.Errorf("Authorization = %q, want %q", gotAuth, want)
	}
}

func TestDoPingWrapsHint(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"unknown token", `{"result":"failure","code":1,"message":"token not found"}`, "auth login"},
		{"scope denied", `{"result":"failure","code":5,"message":"access denied"}`, "scope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			err := doPing(context.Background(), newTestClient(t, srv), &bytes.Buffer{})
			if err == nil {
				t.Fatal("doPing() error = nil, want failure")
			}
			var apiErr *planfix.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("doPing() error %v is not wrapped *APIError", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not contain hint %q", err, tt.want)
			}
		})
	}
}

// executeRoot runs the package-level root command tree.
func executeRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return buf.String(), err
}

// isolateEnv points the config at path and clears profile/credential env vars.
func isolateEnv(t *testing.T, path string) {
	t.Helper()
	t.Setenv("PLANFIX_CONFIG", path)
	t.Setenv("PLANFIX_PROFILE", "")
	t.Setenv("PLANFIX_DOMAIN", "")
	t.Setenv("PLANFIX_TOKEN", "")
	prev := globalOpts
	globalOpts = GlobalOpts{}
	t.Cleanup(func() { globalOpts = prev })
}

func TestPingCommandFailsWithoutCredentials(t *testing.T) {
	isolateEnv(t, filepath.Join(t.TempDir(), "missing.yml"))
	out, err := executeRoot(t, "ping")
	if err == nil {
		t.Fatal("ping error = nil, want failure without credentials")
	}
	if !strings.Contains(err.Error(), "planfix auth login") {
		t.Errorf("error %q does not suggest `planfix auth login`", err)
	}
	if out != "" {
		t.Errorf("output = %q, want empty on failure", out)
	}
}

func TestPingCommandFailsOnEmptyToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	yaml := "current_profile: \"\"\nprofiles:\n  default:\n    domain: example.planfix.ru\n    token: \"\"\n"
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	isolateEnv(t, path)
	_, err := executeRoot(t, "ping")
	if err == nil {
		t.Fatal("ping error = nil, want failure on empty token")
	}
	if !strings.Contains(err.Error(), "empty domain or token") {
		t.Errorf("error %q does not mention empty credentials", err)
	}
}

func TestRootRegistersAuthAndPing(t *testing.T) {
	out, err := executeRoot(t, "auth", "--help")
	if err != nil {
		t.Fatalf("auth --help error = %v", err)
	}
	for _, sub := range []string{"login", "status", "logout"} {
		if !strings.Contains(out, sub) {
			t.Errorf("auth help missing subcommand %q:\n%s", sub, out)
		}
	}
	out, err = executeRoot(t, "--help")
	if err != nil {
		t.Fatalf("--help error = %v", err)
	}
	if !strings.Contains(out, "ping") {
		t.Errorf("root help missing ping command:\n%s", out)
	}
}
