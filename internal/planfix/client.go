// Package planfix is a thin typed client for the Planfix REST API.
package planfix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// New builds a client for domain+token. The domain may carry an http(s) scheme
// and a trailing slash; both are stripped. An empty domain is an error.
func New(domain, token string) (*Client, error) {
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	if domain == "" {
		return nil, fmt.Errorf("planfix domain must not be empty")
	}
	return &Client{
		BaseURL:   "https://" + domain + "/rest",
		Token:     token,
		UserAgent: "planfix-cli",
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Do sends a request and returns the raw response with an open body;
// the caller must close resp.Body. Callers inspect StatusCode.
func (c *Client) Do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		rdr = bytes.NewReader(buf)
	}
	url := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
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
