package planfix

import (
	"context"
	"encoding/json"
	"net/http"
)

// Project is a Planfix project (subset of fields the CLI uses).
type Project struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status Status `json:"status"`
}

// ProjectList is the POST /project/list envelope payload.
type ProjectList struct {
	Projects []Project `json:"projects"`
}

// User is a Planfix user (subset of fields the CLI uses).
type User struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email,omitempty"`
	Status string `json:"status,omitempty"`
}

// UserList is the POST /user/list envelope payload.
type UserList struct {
	Users []User `json:"users"`
}

// ListProjects posts to /project/list. An empty fields omits the key.
func (c *Client) ListProjects(ctx context.Context, offset, pageSize int, fields string) (*ProjectList, []byte, error) {
	body := map[string]any{"offset": offset, "pageSize": pageSize}
	if fields != "" {
		body["fields"] = fields
	}
	raw, err := c.JSON(ctx, http.MethodPost, "/project/list", body)
	if err != nil {
		return nil, nil, err
	}
	var envelope ProjectList
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, raw, err
	}
	return &envelope, raw, nil
}

// ListUsers posts to /user/list. An empty fields omits the key.
func (c *Client) ListUsers(ctx context.Context, offset, pageSize int, fields string) (*UserList, []byte, error) {
	body := map[string]any{"offset": offset, "pageSize": pageSize}
	if fields != "" {
		body["fields"] = fields
	}
	raw, err := c.JSON(ctx, http.MethodPost, "/user/list", body)
	if err != nil {
		return nil, nil, err
	}
	var envelope UserList
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, raw, err
	}
	return &envelope, raw, nil
}
