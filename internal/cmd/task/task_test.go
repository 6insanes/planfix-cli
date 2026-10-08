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

func TestViewRejectsInvalidID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called for invalid id")
	}))
	defer srv.Close()

	for _, args := range [][]string{{"abc"}, {"0"}, {"--", "-3"}} {
		cmd := NewCmd(stubClient(srv), func() Options { return Options{} }, func() string { return "example.com" })
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
	cmd := NewCmd(nil, func() Options { return Options{} }, func() string { return "example.com" })
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

	cmd := NewCmd(stubClient(srv), func() Options { return Options{} }, func() string { return "example.com" })
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

	cmd := NewCmd(stubClient(srv), func() Options { return Options{Fields: "id,name"} }, func() string { return "example.com" })
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

	cmd := NewCmd(stubClient(srv), func() Options { return Options{JSON: true} }, func() string { return "example.com" })
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

	cmd := NewCmd(stubClient(srv), func() Options { return Options{Quiet: true} }, func() string { return "example.com" })
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

	cmd := NewCmd(stubClient(srv), func() Options { return Options{} }, func() string { return "example.com" })
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

func TestViewRendersAssignees(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/task/42" {
			t.Errorf("request = %s %s, want GET /task/42", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"result":"success","task":{
			"id":42,"name":"Ship it","status":{"id":3,"name":"Done"},"priority":"urgent",
			"assignees":{"users":[{"id":5,"name":"Ann"},{"id":6,"name":"Bob"}]}
		}}`))
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{} }, func() string { return "example.com" })
	out, err := exec(t, cmd, "view", "42")
	if err != nil {
		t.Fatalf("view error = %v", err)
	}
	for _, want := range []string{"ASSIGNEES:", "Ann, Bob"} {
		if !strings.Contains(out, want) {
			t.Errorf("view output missing %q:\n%s", want, out)
		}
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 5 {
		t.Errorf("line count = %d, want 5 (empty START/END/DESCRIPTION skipped):\n%s", len(lines), out)
	}
}

func TestViewPlainStripsHTMLDescription(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","task":{
			"id":42,"name":"Ship it","description":"<p>Do <b>it</b></p><h2>Spec</h2><ul><li>a</li></ul>",
			"status":{"id":3,"name":"Done"},"priority":"urgent"
		}}`))
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{Plain: true} }, func() string { return "example.com" })
	out, err := exec(t, cmd, "view", "42")
	if err != nil {
		t.Fatalf("view error = %v", err)
	}
	for _, unwanted := range []string{"<p>", "<b>", "<h2>", "<li>"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("plain view output contains %q:\n%s", unwanted, out)
		}
	}
	for _, want := range []string{"Do it", "Spec", "- a"} {
		if !strings.Contains(out, want) {
			t.Errorf("plain view output missing %q:\n%s", want, out)
		}
	}
}

func TestViewWithoutPlainKeepsHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","task":{
			"id":42,"name":"Ship it","description":"<p>Do it</p>",
			"status":{"id":3,"name":"Done"},"priority":"urgent"
		}}`))
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{} }, func() string { return "example.com" })
	out, err := exec(t, cmd, "view", "42")
	if err != nil {
		t.Fatalf("view error = %v", err)
	}
	if !strings.Contains(out, "<p>Do it</p>") {
		t.Errorf("default view output must keep raw HTML:\n%s", out)
	}
}

func TestViewFieldsWithSpacesRoundTrip(t *testing.T) {
	var gotFields string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotFields = r.URL.Query().Get("fields")
		_, _ = w.Write([]byte(`{"result":"success","task":{"id":42,"name":"Ship it"}}`))
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{Fields: "id, name"} }, func() string { return "example.com" })
	out, err := exec(t, cmd, "view", "42")
	if err != nil {
		t.Fatalf("view error = %v", err)
	}
	if gotFields != "id, name" {
		t.Errorf("decoded fields = %q, want %q", gotFields, "id, name")
	}
	if !strings.Contains(out, "ID:") || !strings.Contains(out, "NAME:") {
		t.Errorf("view output missing ID/NAME rows:\n%s", out)
	}
}

func TestViewJSONOutputsRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","task":{"id":3,"name":"X"}}`))
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{JSON: true} }, func() string { return "example.com" })
	out, err := exec(t, cmd, "view", "3")
	if err != nil {
		t.Fatalf("view error = %v", err)
	}
	if !strings.Contains(out, "\"task\"") || !strings.Contains(out, "{\n  \"result\"") {
		t.Errorf("json view output not pretty-printed:\n%s", out)
	}
}

func TestListFieldsOverrideColumns(t *testing.T) {
	var reqFields string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = jsonDecode(r, &body)
		reqFields, _ = body["fields"].(string)
		_, _ = w.Write([]byte(`{"result":"success","tasks":[
			{"id":1,"name":"First","status":{"id":2,"name":"In progress"},"priority":"high"},
			{"id":2,"name":"Second","status":{"id":1,"name":"New"},"priority":"low"}
		]}`))
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{Fields: "id,name"} }, func() string { return "example.com" })
	out, err := exec(t, cmd, "list")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if reqFields != "id,name" {
		t.Errorf("request fields = %q, want id,name", reqFields)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("line count = %d, want 3:\n%s", len(lines), out)
	}
	header := strings.Fields(lines[0])
	if len(header) != 2 || header[0] != "ID" || header[1] != "NAME" {
		t.Errorf("header = %v, want [ID NAME]:\n%s", header, out)
	}
	for _, unwanted := range []string{"STATUS", "PRIORITY", "In progress", "high"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("list output must not contain %q:\n%s", unwanted, out)
		}
	}
	for _, want := range []string{"First", "Second"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %q:\n%s", want, out)
		}
	}
}

func TestViewFieldsOverrideKeys(t *testing.T) {
	var reqFields string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/task/42" {
			t.Errorf("request = %s %s, want GET /task/42", r.Method, r.URL.Path)
		}
		reqFields = r.URL.Query().Get("fields")
		_, _ = w.Write([]byte(`{"result":"success","task":{
			"id":42,"name":"Ship it","description":"long text",
			"status":{"id":3,"name":"Done"},"priority":"urgent",
			"startDate":"2026-02-01","endDate":"2026-02-10"
		}}`))
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{Fields: "id,name"} }, func() string { return "example.com" })
	out, err := exec(t, cmd, "view", "42")
	if err != nil {
		t.Fatalf("view error = %v", err)
	}
	if reqFields != "id,name" {
		t.Errorf("request fields = %q, want id,name", reqFields)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("line count = %d, want 2:\n%s", len(lines), out)
	}
	for _, want := range []string{"ID:", "NAME:", "42", "Ship it"} {
		if !strings.Contains(out, want) {
			t.Errorf("view output missing %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"STATUS:", "PRIORITY:", "START:", "END:", "DESCRIPTION:", "Done", "long text"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("view output must not contain %q:\n%s", unwanted, out)
		}
	}
}

func TestDefaultFieldsUnchanged(t *testing.T) {
	var listFields, viewFields string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/task/list":
			var body map[string]any
			_ = jsonDecode(r, &body)
			listFields, _ = body["fields"].(string)
			_, _ = w.Write([]byte(`{"result":"success","tasks":[
				{"id":1,"name":"First","status":{"id":2,"name":"In progress"},"priority":"high"}
			]}`))
		case "/task/42":
			viewFields = r.URL.Query().Get("fields")
			_, _ = w.Write([]byte(`{"result":"success","task":{
				"id":42,"name":"Ship it","description":"long text",
				"status":{"id":3,"name":"Done"},"priority":"urgent",
				"startDate":"2026-02-01","endDate":"2026-02-10"
			}}`))
		default:
			t.Errorf("unexpected request path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{} }, func() string { return "example.com" })
	listOut, err := exec(t, cmd, "list")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	viewOut, err := exec(t, cmd, "view", "42")
	if err != nil {
		t.Fatalf("view error = %v", err)
	}
	if listFields != "id,name,status,priority" {
		t.Errorf("list request fields = %q, want id,name,status,priority", listFields)
	}
	if viewFields != "id,name,description,status,priority,startDate,endDate,assignees" {
		t.Errorf("view request fields = %q, want default view fields", viewFields)
	}
	header := strings.Fields(strings.Split(listOut, "\n")[0])
	wantHeader := []string{"ID", "NAME", "STATUS", "PRIORITY"}
	if len(header) != len(wantHeader) {
		t.Errorf("list header = %v, want %v", header, wantHeader)
	} else {
		for i := range wantHeader {
			if header[i] != wantHeader[i] {
				t.Errorf("list header = %v, want %v", header, wantHeader)
				break
			}
		}
	}
	wantKeys := []string{"ID", "NAME", "STATUS", "PRIORITY", "START", "END", "DESCRIPTION"}
	var keys []string
	for _, line := range strings.Split(strings.TrimRight(viewOut, "\n"), "\n") {
		key, _, ok := strings.Cut(line, ":")
		if !ok {
			t.Errorf("view line %q is not a key/value row", line)
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) != len(wantKeys) {
		t.Errorf("view keys = %v, want %v", keys, wantKeys)
	} else {
		for i := range wantKeys {
			if keys[i] != wantKeys[i] {
				t.Errorf("view keys = %v, want %v", keys, wantKeys)
				break
			}
		}
	}
}

func TestNewCmdListsSubcommands(t *testing.T) {
	cmd := NewCmd(nil, func() Options { return Options{} }, func() string { return "example.com" })
	out, err := exec(t, cmd, "--help")
	if err != nil {
		t.Fatalf("help error = %v", err)
	}
	for _, want := range []string{"list", "view", "create", "update", "open"} {
		if !strings.Contains(out, want) {
			t.Errorf("task help missing %q:\n%s", want, out)
		}
	}
}
