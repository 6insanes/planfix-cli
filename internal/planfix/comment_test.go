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
			{"id":10,"description":"hi","type":"comment","dateTime":{"date":"01-01-2026","time":"10:00"},"owner":{"id":"user:3","name":"Ann"}}
		]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	list, raw, err := c.ListComments(context.Background(), 1, "id,description")
	if err != nil {
		t.Fatalf("ListComments() error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/rest/task/1/comments/list" {
		t.Errorf("request = %s %s, want POST /rest/task/1/comments/list", gotMethod, gotPath)
	}
	if body["offset"] != float64(0) || body["pageSize"] != float64(100) {
		t.Errorf("body offset/pageSize = %v/%v, want 0/100", body["offset"], body["pageSize"])
	}
	if body["fields"] != "id,description" {
		t.Errorf("body fields = %v, want id,description", body["fields"])
	}
	if len(list.Comments) != 1 {
		t.Fatalf("comments = %+v, want one entry", list.Comments)
	}
	cm := list.Comments[0]
	if cm.ID != 10 || cm.Text != "hi" || cm.Timestamp.String() != "01-01-2026 10:00" || cm.Author.Name != "Ann" {
		t.Errorf("comment = %+v", cm)
	}
	if cm.Author != (PersonRef{ID: 3, Type: "user", Name: "Ann"}) {
		t.Errorf("author = %+v", cm.Author)
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
	var gotMethod, gotPath, gotQuery string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
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
	if gotMethod != http.MethodPost || gotPath != "/rest/task/1/comments/" {
		t.Errorf("request = %s %s, want POST /rest/task/1/comments/", gotMethod, gotPath)
	}
	if gotQuery != "silent=true" {
		t.Errorf("query = %q, want silent=true", gotQuery)
	}
	if body["description"] != "hello" {
		t.Errorf("body description = %v, want hello", body["description"])
	}
	if _, present := body["silent"]; present {
		t.Errorf("body unexpectedly contains silent: %v", body)
	}
	if id != 55 {
		t.Errorf("id = %d, want 55", id)
	}
	if !strings.Contains(string(raw), `"id":55`) {
		t.Errorf("raw = %s", raw)
	}
}

func TestAddCommentOmitsSilentWhenFalse(t *testing.T) {
	var gotQuery string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = w.Write([]byte(`{"result":"success","id":2}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, _, err := c.AddComment(context.Background(), 1, "hi", false); err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want empty", gotQuery)
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

func TestGetComment(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"result":"success","comment":{
			"id":10,"description":"hi","task":{"id":1,"name":"T"},
			"owner":{"id":"user:3","name":"Ann"}
		}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	cm, raw, err := c.GetComment(context.Background(), 10, "id,task")
	if err != nil {
		t.Fatalf("GetComment() error = %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/rest/comment/10" {
		t.Errorf("request = %s %s, want GET /rest/comment/10", gotMethod, gotPath)
	}
	if !strings.Contains(gotQuery, "fields=") {
		t.Errorf("query = %q, want fields", gotQuery)
	}
	if cm.ID != 10 || cm.Text != "hi" || cm.Task == nil || cm.Task.ID != 1 || cm.Task.Name != "T" {
		t.Errorf("comment = %+v", cm)
	}
	if cm.Author != (PersonRef{ID: 3, Type: "user", Name: "Ann"}) {
		t.Errorf("author = %+v", cm.Author)
	}
	if !strings.Contains(string(raw), `"comment"`) {
		t.Errorf("raw = %s", raw)
	}
}

func TestGetCommentOmitsEmptyFields(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"result":"success","comment":{"id":10}}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, _, err := c.GetComment(context.Background(), 10, ""); err != nil {
		t.Fatalf("GetComment() error = %v", err)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want empty", gotQuery)
	}
}

func TestUpdateComment(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"result":"success"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	raw, err := c.UpdateComment(context.Background(), 1, 10, "new text", true)
	if err != nil {
		t.Fatalf("UpdateComment() error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/rest/task/1/comments/10" {
		t.Errorf("request = %s %s, want POST /rest/task/1/comments/10", gotMethod, gotPath)
	}
	if gotQuery != "silent=true" {
		t.Errorf("query = %q, want silent=true", gotQuery)
	}
	if body["description"] != "new text" {
		t.Errorf("body description = %v, want new text", body["description"])
	}
	if !strings.Contains(string(raw), `"result"`) {
		t.Errorf("raw = %s", raw)
	}
}

func TestUpdateCommentOmitsSilentWhenFalse(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"result":"success"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, err := c.UpdateComment(context.Background(), 1, 10, "hi", false); err != nil {
		t.Fatalf("UpdateComment() error = %v", err)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want empty", gotQuery)
	}
}

func TestDeleteComment(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"result":"success"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	raw, err := c.DeleteComment(context.Background(), 10, false)
	if err != nil {
		t.Fatalf("DeleteComment() error = %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/rest/comment/10" {
		t.Errorf("request = %s %s, want DELETE /rest/comment/10", gotMethod, gotPath)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want empty", gotQuery)
	}
	if !strings.Contains(string(raw), `"result"`) {
		t.Errorf("raw = %s", raw)
	}
}

func TestDeleteCommentSilent(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"result":"success"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, err := c.DeleteComment(context.Background(), 10, true); err != nil {
		t.Fatalf("DeleteComment() error = %v", err)
	}
	if gotQuery != "silent=true" {
		t.Errorf("query = %q, want silent=true", gotQuery)
	}
}

func TestUpdateCommentErrorReturnsNilRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"result":"fail","code":5000,"error":"Comment not found by id - 10"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	raw, err := c.UpdateComment(context.Background(), 1, 10, "hi", false)
	if err == nil {
		t.Fatal("UpdateComment() error = nil, want API error")
	}
	if raw != nil {
		t.Errorf("raw = %v, want nil on error", raw)
	}
}

func TestDeleteCommentErrorReturnsNilRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"result":"fail","code":5000,"error":"Comment not found by id - 10"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	raw, err := c.DeleteComment(context.Background(), 10, false)
	if err == nil {
		t.Fatal("DeleteComment() error = nil, want API error")
	}
	if raw != nil {
		t.Errorf("raw = %v, want nil on error", raw)
	}
}
