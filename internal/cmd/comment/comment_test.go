package comment

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/6insanes/planfix-cli/internal/planfix"
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

// testCmd builds the comment group with a stub client and fixed options.
func testCmd(srv *httptest.Server, opts Options) *cobra.Command {
	return NewCmd(stubClient(srv), func() Options { return opts })
}

func TestNewCmdListsSubcommands(t *testing.T) {
	cmd := NewCmd(nil, func() Options { return Options{} })
	out, err := exec(t, cmd, "--help")
	if err != nil {
		t.Fatalf("help error = %v", err)
	}
	for _, want := range []string{"list", "add"} {
		if !strings.Contains(out, want) {
			t.Errorf("comment help missing %q:\n%s", want, out)
		}
	}
}

func TestListRendersTable(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"result":"success","comments":[
			{"id":10,"description":"hi there","dateTime":{"date":"01-01-2026","time":"10:00"},"owner":{"id":"user:3","name":"Ann"}},
			{"id":11,"description":"done","dateTime":{"date":"02-01-2026","time":"11:00"},"owner":{"id":"user:4","name":"Bob"}}
		]}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	out, err := exec(t, cmd, "list", "1")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/task/1/comments/list" {
		t.Errorf("request = %s %s, want POST /task/1/comments/list", gotMethod, gotPath)
	}
	for _, want := range []string{"ID", "CREATED", "AUTHOR", "TEXT", "hi there", "Ann", "done", "Bob", "01-01-2026 10:00"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %q:\n%s", want, out)
		}
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("line count = %d, want 3:\n%s", len(lines), out)
	}
}

func TestListQuietPrintsIDsOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","comments":[
			{"id":10,"description":"hi"},{"id":11,"description":"done"}
		]}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{Quiet: true})
	out, err := exec(t, cmd, "list", "1")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if strings.Contains(out, "TEXT") || strings.Contains(out, "hi") {
		t.Errorf("quiet list must print ids only:\n%s", out)
	}
	if got, want := strings.Fields(out), []string{"10", "11"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("quiet output = %q, want ids %v", out, want)
	}
}

func TestListJSONOutputsPrettyRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","comments":[{"id":7}]}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{JSON: true})
	out, err := exec(t, cmd, "list", "1")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if !strings.Contains(out, "{\n  \"result\"") || !strings.Contains(out, "\"id\": 7") {
		t.Errorf("json output not pretty-printed:\n%s", out)
	}
}

func TestListRejectsInvalidID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called for invalid id")
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	for _, args := range [][]string{{"abc"}, {"0"}, {"--", "-3"}} {
		out, err := exec(t, cmd, append([]string{"list"}, args...)...)
		if err == nil {
			t.Errorf("list %v: error = nil, want failure", args)
			continue
		}
		if !strings.Contains(err.Error(), "invalid task id") {
			t.Errorf("list %v: error = %q, want invalid task id", args, err)
		}
		if out != "" {
			t.Errorf("list %v: output = %q, want empty", args, out)
		}
	}
}

func TestAddBodyPostsComment(t *testing.T) {
	var gotMethod, gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = jsonDecode(r, &body)
		_, _ = w.Write([]byte(`{"result":"success","id":55}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	out, err := exec(t, cmd, "add", "1", "--body", "hello world")
	if err != nil {
		t.Fatalf("add error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/task/1/comments/" {
		t.Errorf("request = %s %s, want POST /task/1/comments/", gotMethod, gotPath)
	}
	if body["description"] != "hello world" {
		t.Errorf("body description = %v, want hello world", body["description"])
	}
	if _, present := body["silent"]; present {
		t.Errorf("body unexpectedly contains silent: %v", body)
	}
	if !strings.Contains(out, "Added comment 55") {
		t.Errorf("output = %q, want \"Added comment 55\"", out)
	}
}

func TestAddSilentFlag(t *testing.T) {
	var gotQuery string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_ = jsonDecode(r, &body)
		_, _ = w.Write([]byte(`{"result":"success","id":55}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	if _, err := exec(t, cmd, "add", "1", "--body", "hi", "--silent"); err != nil {
		t.Fatalf("add error = %v", err)
	}
	if gotQuery != "silent=true" {
		t.Errorf("query = %q, want silent=true", gotQuery)
	}
	if _, present := body["silent"]; present {
		t.Errorf("body unexpectedly contains silent: %v", body)
	}
}

func TestAddReadsStdinWhenNoBody(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = jsonDecode(r, &body)
		_, _ = w.Write([]byte(`{"result":"success","id":9}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	cmd.SetIn(strings.NewReader("piped text\n"))
	out, err := exec(t, cmd, "add", "1")
	if err != nil {
		t.Fatalf("add error = %v", err)
	}
	if body["description"] != "piped text\n" {
		t.Errorf("body description = %q, want %q", body["description"], "piped text\n")
	}
	if !strings.Contains(out, "Added comment 9") {
		t.Errorf("output = %q, want \"Added comment 9\"", out)
	}
}

func TestAddQuietPrintsIDOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","id":55}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{Quiet: true})
	out, err := exec(t, cmd, "add", "1", "--body", "hi")
	if err != nil {
		t.Fatalf("add error = %v", err)
	}
	if out != "55\n" {
		t.Errorf("quiet output = %q, want \"55\\n\"", out)
	}
}

func TestAddJSONOutputsPrettyRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","id":55}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{JSON: true})
	out, err := exec(t, cmd, "add", "1", "--body", "hi")
	if err != nil {
		t.Fatalf("add error = %v", err)
	}
	if !strings.Contains(out, "{\n  \"result\"") || !strings.Contains(out, "\"id\": 55") {
		t.Errorf("json output not pretty-printed:\n%s", out)
	}
}

func TestAddRequiresText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called without text")
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	cmd.SetIn(strings.NewReader(""))
	_, err := exec(t, cmd, "add", "1")
	if err == nil {
		t.Fatal("add without text: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "--body") {
		t.Errorf("error = %q, want mention of --body", err)
	}
}

func TestAddRejectsInvalidID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called for invalid id")
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	_, err := exec(t, cmd, "add", "abc", "--body", "hi")
	if err == nil {
		t.Fatal("add with invalid id: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "invalid task id") {
		t.Errorf("error = %q, want invalid task id", err)
	}
}
