package task

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"planfix-cli/internal/planfix"
)

func jsonDecode(r *http.Request, dst any) error {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

// stubClient returns a ClientFunc pointing at srv.
func stubClient(srv *httptest.Server) ClientFunc {
	return func() (*planfix.Client, error) {
		c, err := planfix.New("example.com", "tok")
		if err != nil {
			return nil, err
		}
		c.BaseURL = srv.URL
		return c, nil
	}
}

// exec runs cmd with args and returns captured stdout. Usage/error output
// is silenced to mirror the root command's behaviour.
func exec(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	buf := new(bytes.Buffer)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestViewRejectsInvalidID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called for invalid id")
	}))
	defer srv.Close()

	for _, args := range [][]string{{"abc"}, {"0"}, {"--", "-3"}} {
		cmd := NewCmd(stubClient(srv), func() Options { return Options{} })
		out, err := exec(t, cmd, append([]string{"view"}, args...)...)
		if err == nil {
			t.Errorf("view %v: error = nil, want failure", args)
			continue
		}
		if !strings.Contains(err.Error(), "invalid task id") {
			t.Errorf("view %v: error = %q, want invalid task id", args, err)
		}
		if out != "" {
			t.Errorf("view %v: output = %q, want empty", args, out)
		}
	}
}

func TestViewRequiresID(t *testing.T) {
	cmd := NewCmd(nil, func() Options { return Options{} })
	if _, err := exec(t, cmd, "view"); err == nil {
		t.Error("view without args: error = nil, want arity failure")
	}
}

func TestListRendersTableColumns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/task/list" {
			t.Errorf("request = %s %s, want POST /task/list", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"result":"success","tasks":[
			{"id":1,"name":"First","status":{"id":2,"name":"In progress"},"priority":"high"},
			{"id":2,"name":"Second","status":{"id":1,"name":"New"},"priority":"low"}
		]}`))
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{} })
	out, err := exec(t, cmd, "list")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	for _, want := range []string{"ID", "NAME", "STATUS", "PRIORITY", "First", "In progress", "Second", "low"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %q:\n%s", want, out)
		}
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("line count = %d, want 3:\n%s", len(lines), out)
	}
}

func TestListHonoursLimitAndOffset(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body = map[string]any{}
		_ = jsonDecode(r, &body)
		_, _ = w.Write([]byte(`{"result":"success","tasks":[]}`))
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{Fields: "id,name"} })
	if _, err := exec(t, cmd, "list", "--limit", "5", "--offset", "15"); err != nil {
		t.Fatalf("list error = %v", err)
	}
	if body["pageSize"] != float64(5) || body["offset"] != float64(15) {
		t.Errorf("body = %v, want pageSize=5 offset=15", body)
	}
	if body["fields"] != "id,name" {
		t.Errorf("body fields = %v, want id,name", body["fields"])
	}
}

func TestListJSONOutputsRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","tasks":[{"id":9}]}`))
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{JSON: true} })
	out, err := exec(t, cmd, "list")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if !strings.Contains(out, "{\n  \"result\"") || !strings.Contains(out, "\"id\": 9") {
		t.Errorf("json output not pretty-printed:\n%s", out)
	}
}

func TestListQuietPrintsIDsOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","tasks":[
			{"id":1,"name":"First","status":{"id":2,"name":"In progress"},"priority":"high"},
			{"id":2,"name":"Second","status":{"id":1,"name":"New"},"priority":"low"}
		]}`))
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{Quiet: true} })
	out, err := exec(t, cmd, "list")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if strings.Contains(out, "NAME") || strings.Contains(out, "First") {
		t.Errorf("quiet list must print ids only:\n%s", out)
	}
	got := strings.Fields(out)
	want := []string{"1", "2"}
	if len(got) != len(want) {
		t.Fatalf("quiet output = %q, want ids %v", out, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("quiet output = %q, want ids %v", out, want)
		}
	}
}

func TestViewRendersDetailKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/task/42" {
			t.Errorf("request = %s %s, want GET /task/42", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"result":"success","task":{
			"id":42,"name":"Ship it","description":"long text",
			"status":{"id":3,"name":"Done"},"priority":"urgent",
			"startDate":"2026-02-01","endDate":"2026-02-10"
		}}`))
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{} })
	out, err := exec(t, cmd, "view", "42")
	if err != nil {
		t.Fatalf("view error = %v", err)
	}
	for _, want := range []string{"ID:", "NAME:", "STATUS:", "PRIORITY:", "START:", "END:", "DESCRIPTION:", "Ship it", "Done", "long text"} {
		if !strings.Contains(out, want) {
			t.Errorf("view output missing %q:\n%s", want, out)
		}
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 7 {
		t.Errorf("line count = %d, want 7:\n%s", len(lines), out)
	}
}

func TestViewJSONOutputsRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","task":{"id":3,"name":"X"}}`))
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{JSON: true} })
	out, err := exec(t, cmd, "view", "3")
	if err != nil {
		t.Fatalf("view error = %v", err)
	}
	if !strings.Contains(out, "\"task\"") || !strings.Contains(out, "{\n  \"result\"") {
		t.Errorf("json view output not pretty-printed:\n%s", out)
	}
}

func TestNewCmdListsSubcommands(t *testing.T) {
	cmd := NewCmd(nil, func() Options { return Options{} })
	out, err := exec(t, cmd, "--help")
	if err != nil {
		t.Fatalf("help error = %v", err)
	}
	for _, want := range []string{"list", "view"} {
		if !strings.Contains(out, want) {
			t.Errorf("task help missing %q:\n%s", want, out)
		}
	}
}
