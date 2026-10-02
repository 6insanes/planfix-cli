// Package planfix is a thin typed client for the Planfix REST API.
package planfix

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to one Planfix account.
type Client struct {
	BaseURL   string // e.g. https://example.planfix.ru/rest
	Token     string
	UserAgent string
	HTTP      *http.Client
}

// New builds a client for domain+token.
func New(domain, token string) *Client {
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	return &Client{
		BaseURL:   "https://" + domain + "/rest",
		Token:     token,
		UserAgent: "planfix-cli",
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}
}

// Do sends a request and returns the raw response. Callers inspect StatusCode.
func (c *Client) Do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(buf)
	}
	url := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	return c.HTTP.Do(req)
}

// JSON sends a request and returns the response body bytes.
// Non-success (HTTP >= 300 or Planfix failure envelope) maps to *APIError.
func (c *Client) JSON(ctx context.Context, method, path string, body any) ([]byte, error) {
	resp, err := c.Do(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if apiErr := ParseError(resp.StatusCode, raw); apiErr != nil {
		return nil, apiErr
	}
	return raw, nil
}
