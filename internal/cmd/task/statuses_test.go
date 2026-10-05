package task

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// statusesServer stubs the two calls `task statuses` makes: the task fetch
// (processId + current status) and the process status list.
func statusesServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/task/42":
			if got := r.URL.Query().Get("fields"); got != statusesTaskFields {
				t.Errorf("task fields = %q, want %q", got, statusesTaskFields)
			}
			_, _ = w.Write([]byte(`{"result":"success","task":{"id":42,"status":{"id":2,"name":"In progress"},"processId":7}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/object/42/statuses":
			// The object route is scope-restricted on some tokens; the
			// command must fall back to the process route.
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"result":"fail","error":"Scope denied, method not allowed"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/process/task/7":
			_, _ = w.Write([]byte(`{"result":"success","statuses":[
				{"id":1,"name":"New","isActive":true},
				{"id":2,"name":"In progress","isActive":true},
				{"id":3,"name":"Done","isActive":false}
			]}`))
		default:
			t.Errorf("unexpected request = %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestStatusesMarksCurrent(t *testing.T) {
	srv := statusesServer(t)
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{} }, func() string { return "example.com" })
	out, err := exec(t, cmd, "statuses", "42")
	if err != nil {
		t.Fatalf("statuses error = %v", err)
	}
	for _, want := range []string{"ID", "NAME", "ACTIVE", "CURRENT", "New", "In progress", "Done", "true", "false"} {
		if !strings.Contains(out, want) {
			t.Errorf("statuses output missing %q:\n%s", want, out)
		}
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("line count = %d, want 4:\n%s", len(lines), out)
	}
	for _, line := range lines[1:] {
		gotCurrent := strings.HasSuffix(strings.TrimRight(line, " "), "*")
		wantCurrent := strings.Contains(line, "In progress")
		if gotCurrent != wantCurrent {
			t.Errorf("line %q: current = %v, want %v", line, gotCurrent, wantCurrent)
		}
	}
}

func TestStatusesQuietAndJSON(t *testing.T) {
	srv := statusesServer(t)
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{Quiet: true} }, func() string { return "example.com" })
	out, err := exec(t, cmd, "statuses", "42")
	if err != nil {
		t.Fatalf("statuses --quiet error = %v", err)
	}
	if out != "1\n2\n3\n" {
		t.Errorf("quiet output = %q, want ids only", out)
	}

	cmd = NewCmd(stubClient(srv), func() Options { return Options{JSON: true} }, func() string { return "example.com" })
	out, err = exec(t, cmd, "statuses", "42")
	if err != nil {
		t.Fatalf("statuses --json error = %v", err)
	}
	if !strings.Contains(out, `"statuses"`) {
		t.Errorf("json output = %q, want raw statuses payload", out)
	}
}

func TestStatusesRejectsInvalidID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called for invalid id")
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{} }, func() string { return "example.com" })
	if _, err := exec(t, cmd, "statuses", "abc"); err == nil || !strings.Contains(err.Error(), "invalid task id") {
		t.Errorf("statuses abc: error = %v, want invalid task id", err)
	}
}

func TestStatusesRequiresID(t *testing.T) {
	cmd := NewCmd(nil, func() Options { return Options{} }, func() string { return "example.com" })
	if _, err := exec(t, cmd, "statuses"); err == nil {
		t.Error("statuses without args: error = nil, want arity failure")
	}
}
