package task

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// takeServer stubs /userinfo, the task fetch and the update, capturing the
// update body in got.
func takeServer(t *testing.T, taskJSON string, got *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/userinfo":
			_, _ = w.Write([]byte(`{"type":"user","id":"user:7","name":"Igor"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/task/5":
			_, _ = w.Write([]byte(taskJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/task/5":
			body := map[string]any{}
			_ = jsonDecode(r, &body)
			*got = body
			_, _ = w.Write([]byte(`{"result":"success"}`))
		default:
			t.Errorf("unexpected request = %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func statusID(t *testing.T, body map[string]any) float64 {
	t.Helper()
	status, ok := body["status"].(map[string]any)
	if !ok {
		t.Fatalf("body status = %#v, want {id:N} object", body["status"])
	}
	id, _ := status["id"].(float64)
	return id
}

func userIDs(t *testing.T, body map[string]any) []string {
	t.Helper()
	assignees, ok := body["assignees"].(map[string]any)
	if !ok {
		t.Fatalf("body assignees = %#v, want object", body["assignees"])
	}
	var ids []string
	for _, key := range []string{"users", "groups"} {
		list, _ := assignees[key].([]any)
		for _, item := range list {
			m, _ := item.(map[string]any)
			ids = append(ids, key+":"+asID(m["id"]))
		}
	}
	return ids
}

func asID(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.Itoa(int(x))
	default:
		return ""
	}
}

func TestTakeAssignsMeAndSetsStatus(t *testing.T) {
	var body map[string]any
	srv := takeServer(t, `{"result":"success","task":{"id":5,"assignees":{
		"users":[{"id":"user:9","name":"Other"}],
		"groups":[{"id":3,"name":"Devs"}]
	}}}`, &body)
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{} }, func() string { return "example.com" })
	out, err := exec(t, cmd, "take", "5")
	if err != nil {
		t.Fatalf("take error = %v", err)
	}
	if !strings.Contains(out, "Took task 5") {
		t.Errorf("take output = %q, want Took task 5", out)
	}
	if statusID(t, body) != defaultTakeStatus {
		t.Errorf("body status = %v, want %d", statusID(t, body), defaultTakeStatus)
	}
	ids := strings.Join(userIDs(t, body), ",")
	for _, want := range []string{"users:user:9", "users:user:7", "groups:3"} {
		if !strings.Contains(ids, want) {
			t.Errorf("assignees = %q, want %q", ids, want)
		}
	}
}

func TestTakeHonoursStatusFlagAndSkipsDuplicate(t *testing.T) {
	var body map[string]any
	srv := takeServer(t, `{"result":"success","task":{"id":5,"assignees":{
		"users":[{"id":"user:7","name":"Igor"}]
	}}}`, &body)
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{Quiet: true} }, func() string { return "example.com" })
	out, err := exec(t, cmd, "take", "5", "--status", "6")
	if err != nil {
		t.Fatalf("take --status error = %v", err)
	}
	if out != "5\n" {
		t.Errorf("quiet output = %q, want task id", out)
	}
	if statusID(t, body) != 6 {
		t.Errorf("body status = %v, want 6", statusID(t, body))
	}
	ids := userIDs(t, body)
	if len(ids) != 1 || ids[0] != "users:user:7" {
		t.Errorf("assignees = %v, want single user:7", ids)
	}
}

func TestTakeRejectsInvalidArgs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called for invalid args")
	}))
	defer srv.Close()

	cmd := NewCmd(stubClient(srv), func() Options { return Options{} }, func() string { return "example.com" })
	if _, err := exec(t, cmd, "take", "abc"); err == nil || !strings.Contains(err.Error(), "invalid task id") {
		t.Errorf("take abc: error = %v, want invalid task id", err)
	}
	if _, err := exec(t, cmd, "take", "5", "--status", "0"); err == nil || !strings.Contains(err.Error(), "--status") {
		t.Errorf("take --status 0: error = %v, want --status failure", err)
	}
}
