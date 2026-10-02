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

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := New("x", "tok")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = srv.URL + "/rest"
	return c
}

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

	c := newTestClient(t, srv)
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

	c := newTestClient(t, srv)
	var apiErr *APIError
	_, err := c.JSON(context.Background(), http.MethodGet, "/task/1", nil)
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Code != 5 {
		t.Fatalf("code = %d", apiErr.Code)
	}
}

func TestJSONPostMarshalsBody(t *testing.T) {
	var received map[string]any
	var contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Write([]byte(`{"result":"success"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	in := struct {
		Name string `json:"name"`
	}{Name: "hello"}
	if _, err := c.JSON(context.Background(), http.MethodPost, "/task", in); err != nil {
		t.Fatal(err)
	}
	if contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
	if received["name"] != "hello" {
		t.Fatalf("server received %v, want name=hello", received)
	}
}

func TestDoNilBodyOmitsContentType(t *testing.T) {
	var contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		w.Write([]byte(`{"result":"success"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	resp, err := c.Do(context.Background(), http.MethodGet, "/task/1", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if contentType != "" {
		t.Fatalf("Content-Type = %q, want empty for nil body", contentType)
	}
}

func TestNewNormalizesDomain(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"acme.planfix.ru", "https://acme.planfix.ru/rest"},
		{"https://acme.planfix.ru", "https://acme.planfix.ru/rest"},
		{"http://acme.planfix.ru", "https://acme.planfix.ru/rest"},
		{"https://acme.planfix.ru/", "https://acme.planfix.ru/rest"},
	}
	for _, tt := range tests {
		c, err := New(tt.in, "tok")
		if err != nil {
			t.Fatalf("New(%q): %v", tt.in, err)
		}
		if c.BaseURL != tt.want {
			t.Errorf("New(%q).BaseURL = %q, want %q", tt.in, c.BaseURL, tt.want)
		}
	}
}

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"acme.planfix.ru", "acme.planfix.ru"},
		{"https://acme.planfix.ru", "acme.planfix.ru"},
		{"http://acme.planfix.ru", "acme.planfix.ru"},
		{"https://acme.planfix.ru/", "acme.planfix.ru"},
		{"http://acme.planfix.ru//", "acme.planfix.ru/"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := NormalizeDomain(tt.in); got != tt.want {
			t.Errorf("NormalizeDomain(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
	// Idempotent: printing a normalized domain twice must not change it.
	if got := NormalizeDomain(NormalizeDomain("https://acme.planfix.ru/")); got != "acme.planfix.ru" {
		t.Errorf("double NormalizeDomain = %q, want acme.planfix.ru", got)
	}
}

func TestNewRejectsEmptyDomain(t *testing.T) {
	for _, in := range []string{"", "https://", "http://", "/"} {
		if _, err := New(in, "tok"); err == nil {
			t.Errorf("New(%q) succeeded, want error", in)
		}
	}
}

func TestDoCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"result":"success"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Do(ctx, http.MethodGet, "/task/1", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
