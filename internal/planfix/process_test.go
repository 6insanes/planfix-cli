package planfix

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListTaskStatuses(t *testing.T) {
	var gotMethod, gotPath, gotFields string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotFields = r.URL.Query().Get("fields")
		_, _ = w.Write([]byte(`{"result":"success","statuses":[
			{"id":1,"name":"New","color":"#4573b1","isActive":true},
			{"id":2,"name":"Done","isActive":false}
		]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	list, raw, err := c.ListTaskStatuses(context.Background(), 7)
	if err != nil {
		t.Fatalf("ListTaskStatuses() error = %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/rest/process/task/7" {
		t.Errorf("request = %s %s, want GET /rest/process/task/7", gotMethod, gotPath)
	}
	if gotFields != taskStatusFields {
		t.Errorf("fields = %q, want %q", gotFields, taskStatusFields)
	}
	if len(list.Statuses) != 2 {
		t.Fatalf("statuses = %+v, want 2 entries", list.Statuses)
	}
	s := list.Statuses[0]
	if s.ID != 1 || s.Name != "New" || s.Color != "#4573b1" || !s.IsActive {
		t.Errorf("status = %+v", s)
	}
	if list.Statuses[1].ID != 2 || list.Statuses[1].Name != "Done" || list.Statuses[1].IsActive {
		t.Errorf("status = %+v", list.Statuses[1])
	}
	if !strings.Contains(string(raw), `"statuses"`) {
		t.Errorf("raw = %s", raw)
	}
}

func TestListTaskStatusesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"result":"fail","error":"denied"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, _, err := c.ListTaskStatuses(context.Background(), 7); err == nil {
		t.Error("ListTaskStatuses() error = nil, want failure")
	}
}
