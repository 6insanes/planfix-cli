package planfix

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

// APIError is a Planfix REST failure (HTTP or application-level).
type APIError struct {
	Status  int
	Code    int
	Message string
	// fromEnvelope marks Message as coming from a Planfix failure envelope
	// (parsed by ParseError) rather than a raw HTTP error body.
	fromEnvelope bool
}

func (e *APIError) Error() string {
	switch {
	case e.Code != 0:
		return fmt.Sprintf("planfix api error %d: %s", e.Code, e.Message)
	case e.fromEnvelope:
		return fmt.Sprintf("planfix failure: %s", e.Message)
	default:
		return fmt.Sprintf("planfix http %d: %s", e.Status, e.Message)
	}
}

// Hint returns an actionable suggestion for known app codes.
func (e *APIError) Hint() string {
	switch e.Code {
	case 1:
		return "unknown token — run `planfix auth login`"
	case 5:
		return "token lacks the required scope — create a token with the needed permissions"
	case 41:
		return "unknown filter id"
	default:
		return ""
	}
}

// ParseError turns a response body into *APIError, or nil on success.
func ParseError(status int, body []byte) *APIError {
	var envelope struct {
		Result  string `json:"result"`
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil {
		if envelope.Result == "failure" || envelope.Code != 0 {
			msg := envelope.Message
			if msg == "" {
				msg = "unknown error"
			}
			return &APIError{Status: status, Code: envelope.Code, Message: msg, fromEnvelope: true}
		}
		if status < 300 {
			return nil
		}
	}
	if status >= 300 {
		msg := string(body)
		if len(msg) > 200 {
			cut := 200
			for cut > 0 && !utf8.RuneStart(msg[cut]) {
				cut--
			}
			msg = msg[:cut] + "..."
		}
		return &APIError{Status: status, Message: msg}
	}
	return nil
}
