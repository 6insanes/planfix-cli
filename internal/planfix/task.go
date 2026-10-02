package planfix

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// GetTask fetches one task. Returns the typed task and the raw response body.
func (c *Client) GetTask(ctx context.Context, id int, fields string) (*Task, []byte, error) {
	path := fmt.Sprintf("/task/%d", id)
	if fields != "" {
		path += "?" + url.Values{"fields": {fields}}.Encode()
	}
	raw, err := c.JSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, nil, err
	}
	var envelope struct {
		Task Task `json:"task"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, raw, err
	}
	return &envelope.Task, raw, nil
}

// ListTasks posts to /task/list. An invalid FilterJSON fails before any
// network call.
func (c *Client) ListTasks(ctx context.Context, req ListTasksRequest) (*TaskList, []byte, error) {
	body := map[string]any{
		"offset":   req.Offset,
		"pageSize": req.PageSize,
	}
	if req.Fields != "" {
		body["fields"] = req.Fields
	}
	if req.SavedFilter != "" {
		body["filterId"] = req.SavedFilter
	}
	if req.FilterJSON != "" {
		var filters any
		if err := json.Unmarshal([]byte(req.FilterJSON), &filters); err != nil {
			return nil, nil, fmt.Errorf("invalid --filter JSON: %w", err)
		}
		body["filters"] = filters
	}
	raw, err := c.JSON(ctx, http.MethodPost, "/task/list", body)
	if err != nil {
		return nil, nil, err
	}
	var envelope TaskList
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, raw, err
	}
	return &envelope, raw, nil
}
