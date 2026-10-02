package planfix

import (
	"strings"
	"testing"
)

func TestParseErrorFailureEnvelope(t *testing.T) {
	body := []byte(`{"result":"failure","code":1,"message":"Unknown token"}`)
	e := ParseError(200, body)
	if e == nil {
		t.Fatal("expected APIError")
	}
	if e.Code != 1 || e.Message != "Unknown token" {
		t.Fatalf("got %+v", e)
	}
	if !strings.Contains(e.Hint(), "auth login") {
		t.Fatalf("hint = %q", e.Hint())
	}
}

func TestParseErrorHTTPStatus(t *testing.T) {
	e := ParseError(502, []byte("bad gateway"))
	if e == nil || e.Status != 502 {
		t.Fatalf("got %+v", e)
	}
}

func TestParseErrorSuccess(t *testing.T) {
	if e := ParseError(200, []byte(`{"result":"success"}`)); e != nil {
		t.Fatalf("expected nil, got %+v", e)
	}
}
