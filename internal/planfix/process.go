package planfix

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// taskStatusFields are the system fields requested from
// /process/task/{id}/statuses so statuses render with their id, name and
// activity flag.
const taskStatusFields = "id,name,color,isActive"

// TaskStatus is one status of a task process.
type TaskStatus struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color,omitempty"`
	IsActive bool   `json:"isActive"`
}

// TaskStatusList is the GET /process/task/{id}/statuses envelope payload.
type TaskStatusList struct {
	Statuses []TaskStatus `json:"statuses"`
}

// ListTaskStatuses lists the statuses of one task process.
func (c *Client) ListTaskStatuses(ctx context.Context, processID int) (*TaskStatusList, []byte, error) {
	path := fmt.Sprintf("/process/task/%d?%s", processID, url.Values{"fields": {taskStatusFields}}.Encode())
	return c.getTaskStatuses(ctx, path)
}

// ObjectStatuses lists the statuses of one object (task) by its number.
func (c *Client) ObjectStatuses(ctx context.Context, id int) (*TaskStatusList, []byte, error) {
	path := fmt.Sprintf("/object/%d/statuses?%s", id, url.Values{"fields": {taskStatusFields}}.Encode())
	return c.getTaskStatuses(ctx, path)
}

func (c *Client) getTaskStatuses(ctx context.Context, path string) (*TaskStatusList, []byte, error) {
	raw, err := c.JSON(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, nil, err
	}
	var envelope TaskStatusList
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, raw, err
	}
	return &envelope, raw, nil
}
