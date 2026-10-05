package planfix

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetUserInfo(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"sub":"acme:user:123","account_name":"acme","type":"user",
			"id":"user:123","login":"dev","email":"dev@example.com","name":"Developer"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	info, err := c.GetUserInfo(context.Background())
	if err != nil {
		t.Fatalf("GetUserInfo() error = %v", err)
	}
	if gotPath != "/rest/userinfo" {
		t.Errorf("path = %q, want /rest/userinfo", gotPath)
	}
	if info.ID != (PersonRef{ID: 123, Type: "user"}) {
		t.Errorf("id = %+v, want user:123", info.ID)
	}
	if info.Name != "Developer" || info.Login != "dev" || info.Email != "dev@example.com" || info.Type != "user" {
		t.Errorf("info = %+v", info)
	}
}

func TestGetUserInfoBareIDFallsBackToType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":7,"type":"contact","name":"Client"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	info, err := c.GetUserInfo(context.Background())
	if err != nil {
		t.Fatalf("GetUserInfo() error = %v", err)
	}
	if info.ID != (PersonRef{ID: 7, Type: "contact"}) {
		t.Errorf("id = %+v, want contact:7", info.ID)
	}
}
