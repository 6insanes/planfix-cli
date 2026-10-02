package planfix

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
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
	if got := e.Error(); !strings.HasPrefix(got, "planfix api error 1: ") {
		t.Fatalf("Error() = %q", got)
	}
}

func TestParseErrorFailureEnvelopeNoCode(t *testing.T) {
	e := ParseError(200, []byte(`{"result":"failure","message":"boom"}`))
	if e == nil {
		t.Fatal("expected APIError")
	}
	if e.Code != 0 || e.Message != "boom" {
		t.Fatalf("got %+v", e)
	}
	if got := e.Error(); got != "planfix failure: boom" {
		t.Fatalf("Error() = %q, want %q", got, "planfix failure: boom")
	}
}

func TestParseErrorHTTPStatus(t *testing.T) {
	e := ParseError(502, []byte("bad gateway"))
	if e == nil || e.Status != 502 {
		t.Fatalf("got %+v", e)
	}
	if got := e.Error(); got != "planfix http 502: bad gateway" {
		t.Fatalf("Error() = %q", got)
	}
}

func TestParseErrorSuccess(t *testing.T) {
	if e := ParseError(200, []byte(`{"result":"success"}`)); e != nil {
		t.Fatalf("expected nil, got %+v", e)
	}
}

func TestParseErrorTruncatesOnRuneBoundary(t *testing.T) {
	body := []byte(strings.Repeat("あ", 100)) // 3 bytes each, far over 200 bytes
	e := ParseError(500, body)
	if e == nil {
		t.Fatal("expected APIError")
	}
	if !strings.HasSuffix(e.Message, "...") {
		t.Fatalf("message not truncated: %q", e.Message)
	}
	if !utf8.ValidString(e.Message) {
		t.Fatalf("truncated message is not valid UTF-8: %q", e.Message)
	}
	if n := utf8.RuneCountInString(strings.TrimSuffix(e.Message, "...")); n > 200 {
		t.Fatalf("truncated message has %d runes, want <= 200", n)
	}
}

func TestHintErr(t *testing.T) {
	if got := HintErr(nil); got != "" {
		t.Errorf("HintErr(nil) = %q, want empty", got)
	}
	if got := HintErr(errors.New("plain")); got != "" {
		t.Errorf("HintErr(plain) = %q, want empty", got)
	}
	apiErr := &APIError{Code: 1, Message: "token not found"}
	if got := HintErr(apiErr); !strings.Contains(got, "auth login") {
		t.Errorf("HintErr(APIError) = %q, want auth login hint", got)
	}
	// Wrapped errors must still surface the hint (errors.As, not type assert).
	wrapped := fmt.Errorf("ping failed: %w", fmt.Errorf("inner: %w", apiErr))
	if got := HintErr(wrapped); !strings.Contains(got, "auth login") {
		t.Errorf("HintErr(wrapped) = %q, want auth login hint", got)
	}
	// Unknown codes hint nothing.
	if got := HintErr(&APIError{Code: 999}); got != "" {
		t.Errorf("HintErr(code=999) = %q, want empty", got)
	}
}

func TestWrapHint(t *testing.T) {
	// Plain errors pass through untouched.
	plain := errors.New("plain")
	if got := WrapHint(plain); got != plain {
		t.Errorf("WrapHint(plain) = %v, want the same error", got)
	}
	// Hinted errors gain the " (hint)" suffix and stay unwrappable.
	apiErr := &APIError{Code: 1, Message: "token not found"}
	got := WrapHint(apiErr)
	if !strings.Contains(got.Error(), "auth login") || !strings.HasSuffix(got.Error(), ")") {
		t.Errorf("WrapHint(APIError) = %q, want hint suffix", got)
	}
	if !errors.As(got, new(*APIError)) {
		t.Errorf("WrapHint(APIError) must remain unwrappable via errors.As")
	}
}

func TestHint(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{5, "scope"},
		{41, "filter"},
	}
	for _, tt := range tests {
		hint := (&APIError{Code: tt.code}).Hint()
		if !strings.Contains(hint, tt.want) {
			t.Errorf("Hint(code=%d) = %q, want it to contain %q", tt.code, hint, tt.want)
		}
	}
	if got := (&APIError{Code: 999}).Hint(); got != "" {
		t.Errorf("Hint(code=999) = %q, want empty", got)
	}
}
