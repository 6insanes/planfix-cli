package comment

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	for _, want := range []string{"list", "add", "edit", "delete"} {
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

// stubComment serves GET /comment/10 as a comment on taskID and answers
// any mutation with {"result":"success"}.
func stubComment(taskID int, gotMethod, gotPath *string, body *map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = fmt.Fprintf(w, `{"result":"success","comment":{"id":10,"task":{"id":%d}}}`, taskID)
			return
		}
		*gotMethod = r.Method
		*gotPath = r.URL.Path
		if body != nil {
			_ = jsonDecode(r, body)
		}
		_, _ = w.Write([]byte(`{"result":"success"}`))
	}
}

func TestEditBodyUpdatesComment(t *testing.T) {
	var gotMethod, gotPath string
	var body map[string]any
	srv := httptest.NewServer(stubComment(1, &gotMethod, &gotPath, &body))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	out, err := exec(t, cmd, "edit", "1", "10", "--body", "new text")
	if err != nil {
		t.Fatalf("edit error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/task/1/comments/10" {
		t.Errorf("request = %s %s, want POST /task/1/comments/10", gotMethod, gotPath)
	}
	if body["description"] != "new text" {
		t.Errorf("body description = %v, want new text", body["description"])
	}
	if !strings.Contains(out, "Updated comment 10") {
		t.Errorf("output = %q, want \"Updated comment 10\"", out)
	}
}

func TestEditReadsStdinWhenNoBody(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(stubComment(1, new(string), new(string), &body))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	cmd.SetIn(strings.NewReader("piped text\n"))
	if _, err := exec(t, cmd, "edit", "1", "10"); err != nil {
		t.Fatalf("edit error = %v", err)
	}
	if body["description"] != "piped text\n" {
		t.Errorf("body description = %q, want %q", body["description"], "piped text\n")
	}
}

func TestEditRejectsForeignComment(t *testing.T) {
	var mutated bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			mutated = true
			t.Error("mutation must not be called for a foreign comment")
		}
		_, _ = w.Write([]byte(`{"result":"success","comment":{"id":10,"task":{"id":2}}}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	_, err := exec(t, cmd, "edit", "1", "10", "--body", "hi")
	if err == nil {
		t.Fatal("edit foreign comment: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "not a comment on task 1") {
		t.Errorf("error = %q, want task mismatch", err)
	}
	if mutated {
		t.Error("server saw a mutation")
	}
}

func TestEditRejectsInvalidCommentID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called for invalid id")
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	_, err := exec(t, cmd, "edit", "1", "abc", "--body", "hi")
	if err == nil {
		t.Fatal("edit with invalid comment id: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "invalid comment id") {
		t.Errorf("error = %q, want invalid comment id", err)
	}
}

func TestEditJSONFallsBackToIDEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"result":"success","comment":{"id":10,"task":{"id":1}}}`))
			return
		}
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{JSON: true})
	out, err := exec(t, cmd, "edit", "1", "10", "--body", "hi")
	if err != nil {
		t.Fatalf("edit error = %v", err)
	}
	if !strings.Contains(out, `"id": 10`) {
		t.Errorf("json output = %q, want id envelope", out)
	}
}

func TestDeleteChecksTaskThenDeletes(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(stubComment(1, &gotMethod, &gotPath, nil))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	out, err := exec(t, cmd, "delete", "1", "10")
	if err != nil {
		t.Fatalf("delete error = %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/comment/10" {
		t.Errorf("request = %s %s, want DELETE /comment/10", gotMethod, gotPath)
	}
	if !strings.Contains(out, "Deleted comment 10") {
		t.Errorf("output = %q, want \"Deleted comment 10\"", out)
	}
}

func TestDeleteRejectsForeignComment(t *testing.T) {
	var mutated bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			mutated = true
			t.Error("delete must not be called for a foreign comment")
		}
		_, _ = w.Write([]byte(`{"result":"success","comment":{"id":10,"task":{"id":2}}}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	_, err := exec(t, cmd, "delete", "1", "10")
	if err == nil {
		t.Fatal("delete foreign comment: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "not a comment on task 1") {
		t.Errorf("error = %q, want task mismatch", err)
	}
	if mutated {
		t.Error("server saw a mutation")
	}
}

func TestDeleteQuietPrintsIDOnly(t *testing.T) {
	srv := httptest.NewServer(stubComment(1, new(string), new(string), nil))
	defer srv.Close()

	cmd := testCmd(srv, Options{Quiet: true})
	out, err := exec(t, cmd, "delete", "1", "10")
	if err != nil {
		t.Fatalf("delete error = %v", err)
	}
	if out != "10\n" {
		t.Errorf("quiet output = %q, want \"10\\n\"", out)
	}
}

func TestDeleteRejectsInvalidIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called for invalid id")
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	for _, args := range [][]string{{"abc", "10"}, {"1", "abc"}, {"1", "0"}} {
		_, err := exec(t, cmd, append([]string{"delete"}, args...)...)
		if err == nil {
			t.Errorf("delete %v: error = nil, want failure", args)
			continue
		}
		if !strings.Contains(err.Error(), "invalid") {
			t.Errorf("delete %v: error = %q, want invalid id", args, err)
		}
	}
}
