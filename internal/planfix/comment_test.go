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

func TestListComments(t *testing.T) {
	var gotMethod, gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"result":"success","comments":[
			{"id":10,"text":"hi","type":"comment","timestamp":"2026-01-01 10:00","author":{"id":3,"name":"Ann"}}
		]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	list, raw, err := c.ListComments(context.Background(), 1, "id,text")
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/rest/task/1/comment/list" {
		t.Errorf("request = %s %s, want POST /rest/task/1/comment/list", gotMethod, gotPath)
	}
	if body["offset"] != float64(0) || body["pageSize"] != float64(100) {
		t.Errorf("body offset/pageSize = %v/%v, want 0/100", body["offset"], body["pageSize"])
	}
	if body["fields"] != "id,text" {
		t.Errorf("body fields = %v, want id,text", body["fields"])
	}
	if len(list.Comments) != 1 {
		t.Fatalf("comments = %+v, want one entry", list.Comments)
	}
	cm := list.Comments[0]
	if cm.ID != 10 || cm.Text != "hi" || cm.Timestamp != "2026-01-01 10:00" || cm.Author.Name != "Ann" {
		t.Errorf("comment = %+v", cm)
	}
	if !strings.Contains(string(raw), `"comments"`) {
		t.Errorf("raw = %s", raw)
	}
}

func TestListCommentsOmitsEmptyFields(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = w.Write([]byte(`{"result":"success","comments":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, _, err := c.ListComments(context.Background(), 1, ""); err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if _, present := body["fields"]; present {
		t.Errorf("body unexpectedly contains fields: %v", body)
	}
}

func TestAddComment(t *testing.T) {
	var gotMethod, gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"result":"success","id":55}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	id, raw, err := c.AddComment(context.Background(), 1, "hello", true)
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/rest/task/1/comment" {
		t.Errorf("request = %s %s, want POST /rest/task/1/comment", gotMethod, gotPath)
	}
	if body["text"] != "hello" {
		t.Errorf("body text = %v, want hello", body["text"])
	}
	if body["silent"] != true {
		t.Errorf("body silent = %v, want true", body["silent"])
	}
	if id != 55 {
		t.Errorf("id = %d, want 55", id)
	}
	if !strings.Contains(string(raw), `"id":55`) {
		t.Errorf("raw = %s", raw)
	}
}

func TestAddCommentOmitsSilentWhenFalse(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = w.Write([]byte(`{"result":"success","id":2}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, _, err := c.AddComment(context.Background(), 1, "hi", false); err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if _, present := body["silent"]; present {
		t.Errorf("body unexpectedly contains silent: %v", body)
	}
}

func TestAddCommentErrorReturnsNilRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"result":"failure","code":5,"message":"Access denied"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	id, raw, err := c.AddComment(context.Background(), 1, "hello", false)
	if err == nil {
		t.Fatal("AddComment() error = nil, want API error")
	}
	if id != 0 || raw != nil {
		t.Errorf("id/raw = %d/%v, want 0/nil on error", id, raw)
	}
}
