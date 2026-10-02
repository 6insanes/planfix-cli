package planfix

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Comment is a task comment.
type Comment struct {
	ID        int    `json:"id"`
	Text      string `json:"text,omitempty"`
	Type      string `json:"type,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Author    struct {
		ID   int    `json:"id"`
		Name string `json:"name,omitempty"`
	} `json:"author,omitempty"`
}

// CommentList is the POST /task/{id}/comment/list payload.
type CommentList struct {
	Comments []Comment `json:"comments"`
}

// ListComments posts to /task/{id}/comment/list. An empty fields omits the
// "fields" body key.
func (c *Client) ListComments(ctx context.Context, taskID int, fields string) (*CommentList, []byte, error) {
	body := map[string]any{"offset": 0, "pageSize": 100}
	if fields != "" {
		body["fields"] = fields
	}
	raw, err := c.JSON(ctx, http.MethodPost, fmt.Sprintf("/task/%d/comment/list", taskID), body)
	if err != nil {
		return nil, nil, err
	}
	var envelope CommentList
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, raw, err
	}
	return &envelope, raw, nil
}

// AddComment posts a text comment and returns its id. silent adds the
// comment without notifications.
func (c *Client) AddComment(ctx context.Context, taskID int, text string, silent bool) (int, []byte, error) {
	body := map[string]any{"text": text}
	if silent {
		body["silent"] = true
	}
	raw, err := c.JSON(ctx, http.MethodPost, fmt.Sprintf("/task/%d/comment", taskID), body)
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
