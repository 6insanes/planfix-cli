package task

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// writeCmd builds the task group with a stub client and domain getter.
func writeCmd(srv *httptest.Server, opts Options) *cobra.Command {
	return NewCmd(stubClient(srv), func() Options { return opts },
		func() string { return "example.com" })
}

func TestCreateRequiresName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called without --name")
	}))
	defer srv.Close()

	cmd := writeCmd(srv, Options{})
	_, err := exec(t, cmd, "create")
	if err == nil {
		t.Fatal("create without --name: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "--name") {
		t.Errorf("error = %q, want mention of --name", err)
	}
}

func TestCreateQuietPrintsIDOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","id":99}`))
	}))
	defer srv.Close()

	cmd := writeCmd(srv, Options{Quiet: true})
	out, err := exec(t, cmd, "create", "--name", "New task")
	if err != nil {
		t.Fatalf("create error = %v", err)
	}
	if out != "99\n" {
		t.Errorf("quiet output = %q, want \"99\\n\"", out)
	}
}

func TestCreateDefaultPrintsCreatedLine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","id":42}`))
	}))
	defer srv.Close()

	cmd := writeCmd(srv, Options{})
	out, err := exec(t, cmd, "create", "--name", "New task")
	if err != nil {
		t.Fatalf("create error = %v", err)
	}
	if !strings.Contains(out, "Created task 42") {
		t.Errorf("output = %q, want \"Created task 42\"", out)
	}
}

func TestCreateJSONOutputsPrettyRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","id":7}`))
	}))
	defer srv.Close()

	cmd := writeCmd(srv, Options{JSON: true})
	out, err := exec(t, cmd, "create", "--name", "New task")
	if err != nil {
		t.Fatalf("create error = %v", err)
	}
	if !strings.Contains(out, "{\n  \"result\"") || !strings.Contains(out, "\"id\": 7") {
		t.Errorf("json output not pretty-printed:\n%s", out)
	}
}

func TestCreatePostsFlagsAsBody(t *testing.T) {
	var gotMethod, gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = jsonDecode(r, &body)
		_, _ = w.Write([]byte(`{"result":"success","id":10}`))
	}))
	defer srv.Close()

	cmd := writeCmd(srv, Options{})
	if _, err := exec(t, cmd, "create",
		"--name", "New",
		"--description", "desc",
		"--project", "3",
		"--parent", "4",
		"--assignees", "user:7,group:2",
		"--start-date", "2026-01-01",
		"--end-date", "2026-01-31",
	); err != nil {
		t.Fatalf("create error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/task" {
		t.Errorf("request = %s %s, want POST /task", gotMethod, gotPath)
	}
	if body["name"] != "New" || body["description"] != "desc" {
		t.Errorf("body name/description = %v/%v", body["name"], body["description"])
	}
	if body["startDate"] != "2026-01-01" || body["endDate"] != "2026-01-31" {
		t.Errorf("body dates = %v/%v", body["startDate"], body["endDate"])
	}
	if project, _ := body["project"].(map[string]any); project == nil || project["id"] != float64(3) {
		t.Errorf("body project = %v, want id 3", body["project"])
	}
	if parent, _ := body["parent"].(map[string]any); parent == nil || parent["id"] != float64(4) {
		t.Errorf("body parent = %v, want id 4", body["parent"])
	}
	assignees, _ := body["assignees"].(map[string]any)
	users, _ := assignees["users"].([]any)
	if len(users) != 1 {
		t.Fatalf("body assignees.users = %#v, want one entry", body["assignees"])
	}
	first, _ := users[0].(map[string]any)
	if first["id"] != "user:7" {
		t.Errorf("users[0] = %#v, want id %q", first, "user:7")
	}
	groups, _ := assignees["groups"].([]any)
	if len(groups) != 1 {
		t.Fatalf("body assignees.groups = %#v, want one entry", body["assignees"])
	}
	group, _ := groups[0].(map[string]any)
	if group["id"] != float64(2) {
		t.Errorf("groups[0] = %#v, want id 2", group)
	}
}

func TestCreateRejectsInvalidAssignees(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called for invalid assignees")
	}))
	defer srv.Close()

	cmd := writeCmd(srv, Options{})
	_, err := exec(t, cmd, "create", "--name", "X", "--assignees", "bogus")
	if err == nil {
		t.Fatal("create with invalid assignees: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "invalid person") {
		t.Errorf("error = %q, want invalid person", err)
	}
}

func TestUpdateSendsOnlyChangedFieldsWithStatus(t *testing.T) {
	var gotMethod, gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		_ = jsonDecode(r, &body)
		_, _ = w.Write([]byte(`{"result":"success"}`))
	}))
	defer srv.Close()

	cmd := writeCmd(srv, Options{Quiet: true})
	out, err := exec(t, cmd, "update", "5", "--name", "Renamed", "--status", "3")
	if err != nil {
		t.Fatalf("update error = %v", err)
	}
	if out != "5\n" {
		t.Errorf("quiet output = %q, want \"5\\n\"", out)
	}
	if gotMethod != http.MethodPost || gotPath != "/task/5" {
		t.Errorf("request = %s %s, want POST /task/5", gotMethod, gotPath)
	}
	if body["name"] != "Renamed" {
		t.Errorf("body name = %v, want Renamed", body["name"])
	}
	if st, _ := body["status"].(map[string]any); st == nil || st["id"] != float64(3) {
		t.Errorf("body status = %v, want {id:3}", body["status"])
	}
	for _, key := range []string{"description", "startDate", "endDate", "assignees"} {
		if _, present := body[key]; present {
			t.Errorf("body unexpectedly contains %q: %v", key, body)
		}
	}
}

func TestUpdateDefaultPrintsUpdatedLine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success"}`))
	}))
	defer srv.Close()

	cmd := writeCmd(srv, Options{})
	out, err := exec(t, cmd, "update", "5", "--description", "d")
	if err != nil {
		t.Fatalf("update error = %v", err)
	}
	if !strings.Contains(out, "Updated task 5") {
		t.Errorf("output = %q, want \"Updated task 5\"", out)
	}
}

func TestUpdateRejectsInvalidID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called for invalid id")
	}))
	defer srv.Close()

	cmd := writeCmd(srv, Options{})
	_, err := exec(t, cmd, "update", "abc", "--name", "X")
	if err == nil {
		t.Fatal("update with invalid id: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "invalid task id") {
		t.Errorf("error = %q, want invalid task id", err)
	}
}

func TestOpenPrintsURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("open must not call the API")
	}))
	defer srv.Close()

	cmd := writeCmd(srv, Options{})
	out, err := exec(t, cmd, "open", "1")
	if err != nil {
		t.Fatalf("open error = %v", err)
	}
	if out != "https://example.com/task/1\n" {
		t.Errorf("open output = %q, want https://example.com/task/1", out)
	}
}

func TestOpenNormalizesDomain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("open must not call the API")
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{} },
		func() string { return "https://example.com/" })
	out, err := exec(t, cmd, "open", "1")
	if err != nil {
		t.Fatalf("open error = %v", err)
	}
	if out != "https://example.com/task/1\n" {
		t.Errorf("open output = %q, want https://example.com/task/1 (scheme and slash stripped)", out)
	}
}

func TestOpenRejectsInvalidID(t *testing.T) {
	cmd := writeCmd(nil, Options{})
	for _, args := range [][]string{{"abc"}, {"0"}, {"--", "-3"}} {
		_, err := exec(t, cmd, append([]string{"open"}, args...)...)
		if err == nil {
			t.Errorf("open %v: error = nil, want failure", args)
			continue
		}
		if !strings.Contains(err.Error(), "invalid task id") {
			t.Errorf("open %v: error = %q, want invalid task id", args, err)
		}
	}
}

func TestParsePeople(t *testing.T) {
	got, err := parsePeople("user:1, contact:2 ,group:3")
	if err != nil {
		t.Fatalf("parsePeople() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	want := []struct {
		typ string
		id  int
	}{{"user", 1}, {"contact", 2}, {"group", 3}}
	for i := range want {
		if got[i].Type != want[i].typ || got[i].ID != want[i].id {
			t.Errorf("got[%d] = %+v, want %s:%d", i, got[i], want[i].typ, want[i].id)
		}
	}
	if people, err := parsePeople(""); err != nil || people != nil {
		t.Errorf("parsePeople(\"\") = %v, %v, want nil, nil", people, err)
	}
}

func TestParsePeopleInvalid(t *testing.T) {
	for _, spec := range []string{"abc", "user:x", "user:0", "foo:1", "user:", "user:1;2"} {
		if _, err := parsePeople(spec); err == nil {
			t.Errorf("parsePeople(%q): error = nil, want failure", spec)
		} else if !strings.Contains(err.Error(), "invalid") {
			t.Errorf("parsePeople(%q): error = %q, want mention of invalid", spec, err)
		}
	}
}
