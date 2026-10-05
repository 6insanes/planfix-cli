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

// CreateTaskRequest is the input to POST /task. Fields carry no JSON tags:
// CreateTask builds the wire shape in its method body, omitting empty values.
type CreateTaskRequest struct {
	Name        string
	Description string
	ProjectID   int
	ParentID    int
	Assignees   []PersonRef
	StartDate   string
	EndDate     string
}

// UpdateTaskRequest is the input to POST /task/{id} (partial). Fields carry
// no JSON tags: UpdateTask builds the wire shape in its method body. Pointer
// fields are sent only when set, so a zero value leaves the attribute untouched.
type UpdateTaskRequest struct {
	Name        *string
	Description *string
	StartDate   *string
	EndDate     *string
	Status      *int
	Assignees   []PersonRef
}

// assigneesBody splits person refs into the wire shape the API expects:
// user/contact refs under "users" with prefixed ids, group refs under
// "groups" with plain ids.
func assigneesBody(refs []PersonRef) map[string]any {
	var users, groups []PersonRef
	for _, r := range refs {
		if r.Type == "group" {
			groups = append(groups, r)
		} else {
			users = append(users, r)
		}
	}
	out := map[string]any{}
	if len(users) > 0 {
		out["users"] = users
	}
	if len(groups) > 0 {
		out["groups"] = groups
	}
	return out
}

// CreateTask posts a new task and returns its id.
func (c *Client) CreateTask(ctx context.Context, req CreateTaskRequest) (int, []byte, error) {
	body := map[string]any{"name": req.Name}
	if req.Description != "" {
		body["description"] = req.Description
	}
	if req.ProjectID > 0 {
		body["project"] = map[string]any{"id": req.ProjectID}
	}
	if req.ParentID > 0 {
		body["parent"] = map[string]any{"id": req.ParentID}
	}
	if len(req.Assignees) > 0 {
		body["assignees"] = assigneesBody(req.Assignees)
	}
	if req.StartDate != "" {
		body["startDate"] = req.StartDate
	}
	if req.EndDate != "" {
		body["endDate"] = req.EndDate
	}
	raw, err := c.JSON(ctx, http.MethodPost, "/task", body)
	if err != nil {
		return 0, nil, err
	}
	var envelope struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, raw, err
	}
	return envelope.ID, raw, nil
}

// UpdateTask posts a partial update.
func (c *Client) UpdateTask(ctx context.Context, id int, req UpdateTaskRequest) ([]byte, error) {
	body := map[string]any{}
	if req.Name != nil {
		body["name"] = *req.Name
	}
	if req.Description != nil {
		body["description"] = *req.Description
	}
	if req.StartDate != nil {
		body["startDate"] = *req.StartDate
	}
	if req.EndDate != nil {
		body["endDate"] = *req.EndDate
	}
	if req.Status != nil {
		body["status"] = map[string]any{"id": *req.Status}
	}
	if len(req.Assignees) > 0 {
		body["assignees"] = assigneesBody(req.Assignees)
	}
	return c.JSON(ctx, http.MethodPost, fmt.Sprintf("/task/%d", id), body)
}
