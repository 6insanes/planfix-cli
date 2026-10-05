package planfix

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// TimePoint is a Planfix date-time value: calendar date (dd-MM-yyyy),
// clock time (HH:MM), and an ISO datetime.
type TimePoint struct {
	Date     string `json:"date,omitempty"`
	Time     string `json:"time,omitempty"`
	Datetime string `json:"datetime,omitempty"`
}

// String renders "date time", falling back to the ISO datetime.
func (t TimePoint) String() string {
	if s := strings.TrimSpace(t.Date + " " + t.Time); s != "" {
		return s
	}
	return t.Datetime
}

// Comment is a task comment. JSON tags mirror the CommentResponse wire
// names: the text is "description", the author is "owner".
type Comment struct {
	ID        int       `json:"id"`
	Text      string    `json:"description,omitempty"`
	Type      string    `json:"type,omitempty"`
	Timestamp TimePoint `json:"dateTime,omitempty"`
	Author    PersonRef `json:"owner,omitempty"`
}

// CommentList is the POST /task/{id}/comments/list payload.
type CommentList struct {
	Comments []Comment `json:"comments"`
}

// commentPageSize is the comment/list page size used by ListComments and
// the paging ListWorklog loop.
const commentPageSize = 100

// ListComments posts to /task/{id}/comments/list. An empty fields omits
// the "fields" body key. It fetches a single page.
func (c *Client) ListComments(ctx context.Context, taskID int, fields string) (*CommentList, []byte, error) {
	raw, err := c.listCommentsPage(ctx, taskID, fields, 0, commentPageSize)
	if err != nil {
		return nil, nil, err
	}
	var envelope CommentList
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, raw, err
	}
	return &envelope, raw, nil
}

// listCommentsPage fetches one offset/pageSize page of /comments/list.
func (c *Client) listCommentsPage(ctx context.Context, taskID int, fields string, offset, pageSize int) ([]byte, error) {
	body := map[string]any{"offset": offset, "pageSize": pageSize}
	if fields != "" {
		body["fields"] = fields
	}
	return c.JSON(ctx, http.MethodPost, fmt.Sprintf("/task/%d/comments/list", taskID), body)
}

// AddComment posts a text comment and returns its id. silent adds the
// comment without notifications (the silent=true query parameter).
func (c *Client) AddComment(ctx context.Context, taskID int, text string, silent bool) (int, []byte, error) {
	path := fmt.Sprintf("/task/%d/comments/", taskID)
	if silent {
		path += "?silent=true"
	}
	body := map[string]any{"description": text}
	raw, err := c.JSON(ctx, http.MethodPost, path, body)
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
