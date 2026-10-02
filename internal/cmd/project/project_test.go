package project

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

// testCmd builds the project group with a stub client and fixed options.
func testCmd(srv *httptest.Server, opts Options) *cobra.Command {
	return NewCmd(stubClient(srv), func() Options { return opts })
}

// decodeBody reads the request body into a map.
func decodeBody(r *http.Request, dst any) error {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

func TestNewCmdListsSubcommands(t *testing.T) {
	cmd := NewCmd(nil, func() Options { return Options{} })
	out, err := exec(t, cmd, "--help")
	if err != nil {
		t.Fatalf("help error = %v", err)
	}
	if !strings.Contains(out, "list") {
		t.Errorf("project help missing %q:\n%s", "list", out)
	}
}

func TestListRendersTable(t *testing.T) {
	var gotMethod, gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = decodeBody(r, &body)
		_, _ = w.Write([]byte(`{"result":"success","projects":[
			{"id":1,"name":"Alpha","status":{"id":2,"name":"Active"}},
			{"id":2,"name":"Beta","status":{"id":3,"name":"Closed"}}
		]}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	out, err := exec(t, cmd, "list")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/project/list" {
		t.Errorf("request = %s %s, want POST /project/list", gotMethod, gotPath)
	}
	if body["fields"] != defaultListFields {
		t.Errorf("body fields = %v, want %q", body["fields"], defaultListFields)
	}
	if body["offset"] != float64(0) || body["pageSize"] != float64(defaultListLimit) {
		t.Errorf("body offset/pageSize = %v/%v, want 0/%d", body["offset"], body["pageSize"], defaultListLimit)
	}
	for _, want := range []string{"ID", "NAME", "STATUS", "Alpha", "Active", "Beta", "Closed"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %q:\n%s", want, out)
		}
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("line count = %d, want 3:\n%s", len(lines), out)
	}
}

func TestListFieldsDriveColumns(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = decodeBody(r, &body)
		_, _ = w.Write([]byte(`{"result":"success","projects":[{"id":9,"name":"Solo","status":{"id":1,"name":"New"}}]}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{Fields: "id,name"})
	out, err := exec(t, cmd, "list")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if body["fields"] != "id,name" {
		t.Errorf("body fields = %v, want id,name", body["fields"])
	}
	if strings.Contains(out, "STATUS") {
		t.Errorf("output must not contain STATUS for --fields id,name:\n%s", out)
	}
	for _, want := range []string{"ID", "NAME", "Solo"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestListPassesLimitOffset(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = decodeBody(r, &body)
		_, _ = w.Write([]byte(`{"result":"success","projects":[]}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{})
	if _, err := exec(t, cmd, "list", "--limit", "5", "--offset", "15"); err != nil {
		t.Fatalf("list error = %v", err)
	}
	if body["pageSize"] != float64(5) || body["offset"] != float64(15) {
		t.Errorf("body pageSize/offset = %v/%v, want 5/15", body["pageSize"], body["offset"])
	}
}

func TestListQuietPrintsIDsOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","projects":[
			{"id":1,"name":"Alpha"},{"id":2,"name":"Beta"}
		]}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{Quiet: true})
	out, err := exec(t, cmd, "list")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if strings.Contains(out, "NAME") || strings.Contains(out, "Alpha") {
		t.Errorf("quiet list must print ids only:\n%s", out)
	}
	if got, want := strings.Fields(out), []string{"1", "2"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("quiet output = %q, want ids %v", out, want)
	}
}

func TestListJSONOutputsPrettyRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","projects":[{"id":3}]}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{JSON: true})
	out, err := exec(t, cmd, "list")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if !strings.Contains(out, "{\n  \"result\"") || !strings.Contains(out, "\"id\": 3") {
		t.Errorf("json output not pretty-printed:\n%s", out)
	}
}
