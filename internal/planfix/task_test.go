package planfix

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetTask(t *testing.T) {
	var gotMethod, gotPath, gotFields string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotFields = r.URL.Query().Get("fields")
		_, _ = w.Write([]byte(`{"result":"success","task":{"id":7,"name":"Do thing","status":{"id":1,"name":"New"},"priority":"high","startDate":"2026-01-01"}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	task, raw, err := c.GetTask(context.Background(), 7, "id,name,status")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/rest/task/7" {
		t.Errorf("request = %s %s, want GET /rest/task/7", gotMethod, gotPath)
	}
	if gotFields != "id,name,status" {
		t.Errorf("decoded fields = %q, want id,name,status", gotFields)
	}
	if task.ID != 7 || task.Name != "Do thing" || task.Status.Name != "New" || task.Priority != "high" {
		t.Errorf("task = %+v", task)
	}
	if !strings.Contains(string(raw), `"id":7`) {
		t.Errorf("raw = %s", raw)
	}
}

func TestGetTaskEscapesFieldsQuery(t *testing.T) {
	var gotQuery, gotFields string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		gotFields = r.URL.Query().Get("fields")
		_, _ = w.Write([]byte(`{"result":"success","task":{"id":1}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	const fields = "id, name & status.name=x"
	if _, _, err := c.GetTask(context.Background(), 1, fields); err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if gotFields != fields {
		t.Errorf("decoded fields = %q, want %q", gotFields, fields)
	}
	if want := "fields=id%2C+name+%26+status.name%3Dx"; gotQuery != want {
		t.Errorf("query = %q, want %q", gotQuery, want)
	}
}

func TestGetTaskNoFieldsOmitsQuery(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"result":"success","task":{"id":1}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, _, err := c.GetTask(context.Background(), 1, ""); err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want empty", gotQuery)
	}
}

func TestListTasks(t *testing.T) {
	var gotMethod, gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"result":"success","tasks":[{"id":1,"name":"A"},{"id":2,"name":"B"}]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	list, raw, err := c.ListTasks(context.Background(), ListTasksRequest{
		Offset:      10,
		PageSize:    25,
		Fields:      "id,name,status,priority",
		SavedFilter: ":in",
		FilterJSON:  `[{"field":"status","operator":"neq","values":["6"]}]`,
	})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/rest/task/list" {
		t.Errorf("request = %s %s, want POST /rest/task/list", gotMethod, gotPath)
	}
	if body["offset"] != float64(10) || body["pageSize"] != float64(25) {
		t.Errorf("body offset/pageSize = %v/%v, want 10/25", body["offset"], body["pageSize"])
	}
	if body["fields"] != "id,name,status,priority" {
		t.Errorf("body fields = %v", body["fields"])
	}
	if body["filterId"] != ":in" {
		t.Errorf("body filterId = %v, want :in", body["filterId"])
	}
	filters, ok := body["filters"].([]any)
	if !ok || len(filters) != 1 {
		t.Errorf("body filters = %#v, want one-element array", body["filters"])
	}
	if len(list.Tasks) != 2 || list.Tasks[0].ID != 1 || list.Tasks[1].Name != "B" {
		t.Errorf("tasks = %+v", list.Tasks)
	}
	if !strings.Contains(string(raw), `"tasks"`) {
		t.Errorf("raw = %s", raw)
	}
}

func TestListTasksOmitsEmptyOptions(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = w.Write([]byte(`{"result":"success","tasks":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, _, err := c.ListTasks(context.Background(), ListTasksRequest{Offset: 0, PageSize: 50}); err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	for _, key := range []string{"fields", "filterId", "filters"} {
		if _, present := body[key]; present {
			t.Errorf("body unexpectedly contains %q: %v", key, body)
		}
	}
}

func TestListTasksInvalidFilterJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("server must not be called for invalid filter JSON")
		_, _ = w.Write([]byte(`{"result":"success","tasks":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, _, err := c.ListTasks(context.Background(), ListTasksRequest{FilterJSON: "{not json"})
	if err == nil {
		t.Fatal("ListTasks() error = nil, want invalid --filter error")
	}
	if !strings.Contains(err.Error(), "filter") {
		t.Errorf("error %q does not mention filter", err)
	}
}
