package planfix

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListProjects(t *testing.T) {
	var gotMethod, gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"result":"success","projects":[
			{"id":1,"name":"Alpha","status":{"id":2,"name":"Active"}},
			{"id":2,"name":"Beta","status":{"id":3,"name":"Closed"}}
		]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	list, raw, err := c.ListProjects(context.Background(), 10, 25, "id,name,status")
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/rest/project/list" {
		t.Errorf("request = %s %s, want POST /rest/project/list", gotMethod, gotPath)
	}
	if body["offset"] != float64(10) || body["pageSize"] != float64(25) {
		t.Errorf("body offset/pageSize = %v/%v, want 10/25", body["offset"], body["pageSize"])
	}
	if body["fields"] != "id,name,status" {
		t.Errorf("body fields = %v, want id,name,status", body["fields"])
	}
	if len(list.Projects) != 2 {
		t.Fatalf("projects = %+v, want 2 entries", list.Projects)
	}
	p := list.Projects[0]
	if p.ID != 1 || p.Name != "Alpha" || p.Status.ID != 2 || p.Status.Name != "Active" {
		t.Errorf("project = %+v", p)
	}
	if list.Projects[1].ID != 2 || list.Projects[1].Name != "Beta" {
		t.Errorf("project = %+v", list.Projects[1])
	}
	if !strings.Contains(string(raw), `"projects"`) {
		t.Errorf("raw = %s", raw)
	}
}

func TestListProjectsOmitsEmptyFields(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = w.Write([]byte(`{"result":"success","projects":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, _, err := c.ListProjects(context.Background(), 0, 50, ""); err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if _, present := body["fields"]; present {
		t.Errorf("body unexpectedly contains fields: %v", body)
	}
}

func TestListProjectsFailureEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"failure","code":1,"message":"unknown token"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	list, raw, err := c.ListProjects(context.Background(), 0, 50, "")
	if err == nil {
		t.Fatal("ListProjects() error = nil, want API error")
	}
	if list != nil || raw != nil {
		t.Errorf("list/raw = %v/%v, want nil on error", list, raw)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 1 {
		t.Errorf("err = %v, want *APIError code 1", err)
	}
}

func TestListUsers(t *testing.T) {
	var gotMethod, gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"result":"success","users":[
			{"id":7,"name":"Ann Smith","email":"ann@example.com","status":"active"},
			{"id":8,"name":"Bob","email":"bob@example.com","status":""}
		]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	list, raw, err := c.ListUsers(context.Background(), 5, 10, "id,name,email")
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/rest/user/list" {
		t.Errorf("request = %s %s, want POST /rest/user/list", gotMethod, gotPath)
	}
	if body["offset"] != float64(5) || body["pageSize"] != float64(10) {
		t.Errorf("body offset/pageSize = %v/%v, want 5/10", body["offset"], body["pageSize"])
	}
	if body["fields"] != "id,name,email" {
		t.Errorf("body fields = %v, want id,name,email", body["fields"])
	}
	if len(list.Users) != 2 {
		t.Fatalf("users = %+v, want 2 entries", list.Users)
	}
	u := list.Users[0]
	if u.ID != 7 || u.Name != "Ann Smith" || u.Email != "ann@example.com" || u.Status != "active" {
		t.Errorf("user = %+v", u)
	}
	if !strings.Contains(string(raw), `"users"`) {
		t.Errorf("raw = %s", raw)
	}
}

func TestListUsersOmitsEmptyFields(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = w.Write([]byte(`{"result":"success","users":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, _, err := c.ListUsers(context.Background(), 0, 50, ""); err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if _, present := body["fields"]; present {
		t.Errorf("body unexpectedly contains fields: %v", body)
	}
}

func TestListUsersFailureEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"failure","code":5,"message":"no scope"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	list, raw, err := c.ListUsers(context.Background(), 0, 50, "")
	if err == nil {
		t.Fatal("ListUsers() error = nil, want API error")
	}
	if list != nil || raw != nil {
		t.Errorf("list/raw = %v/%v, want nil on error", list, raw)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 5 {
		t.Errorf("err = %v, want *APIError code 5", err)
	}
}
