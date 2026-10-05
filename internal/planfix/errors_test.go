package planfix

import "testing"

func TestParseErrorEnvelopeFields(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"spec error field", `{"result":"fail","code":1000,"error":"Task not found by id 10"}`, "Task not found by id 10"},
		{"legacy message field", `{"result":"failure","code":5,"message":"Access denied"}`, "Access denied"},
		{"no message", `{"result":"fail","code":1}`, "unknown error"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			apiErr := ParseError(500, []byte(tt.body))
			if apiErr == nil {
				t.Fatalf("ParseError(%s) = nil, want error", tt.body)
			}
			if apiErr.Message != tt.want {
				t.Errorf("message = %q, want %q", apiErr.Message, tt.want)
			}
		})
	}
}

func TestParseErrorSuccessIsNil(t *testing.T) {
	if apiErr := ParseError(200, []byte(`{"result":"success"}`)); apiErr != nil {
		t.Errorf("ParseError(success) = %v, want nil", apiErr)
	}
}
