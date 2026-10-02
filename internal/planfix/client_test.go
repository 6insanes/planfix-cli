package planfix

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSONSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/rest/task/1" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`{"result":"success","task":{"id":1}}`))
	}))
	defer srv.Close()

	c := New("x", "tok")
	c.BaseURL = srv.URL + "/rest"
	body, err := c.JSON(context.Background(), http.MethodGet, "/task/1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"id":1`) {
		t.Fatalf("body = %s", body)
	}
}

func TestJSONFailureEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"result":"failure","code":5,"message":"Access denied"}`))
	}))
	defer srv.Close()

	c := New("x", "tok")
	c.BaseURL = srv.URL + "/rest"
	var apiErr *APIError
	_, err := c.JSON(context.Background(), http.MethodGet, "/task/1", nil)
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Code != 5 {
		t.Fatalf("code = %d", apiErr.Code)
	}
}
