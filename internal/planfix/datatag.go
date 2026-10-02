package planfix

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// DataTag is a Planfix data tag definition.
type DataTag struct {
	ID     int         `json:"id"`
	Name   string      `json:"name"`
	Fields []DataField `json:"fields,omitempty"`
}

// DataField is a custom field on a data tag.
type DataField struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Type        int    `json:"type"`
	DirectoryID int    `json:"directoryId,omitempty"`
}

// DataTagList is the GET /datatag payload.
type DataTagList struct {
	DataTags []DataTag `json:"dataTags"`
}

// ListDataTags lists the account's data tags.
func (c *Client) ListDataTags(ctx context.Context) (*DataTagList, error) {
	raw, err := c.JSON(ctx, http.MethodGet, "/datatag", nil)
	if err != nil {
		return nil, err
	}
	var envelope DataTagList
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	return &envelope, nil
}

// GetDataTag fetches one data tag with its fields.
func (c *Client) GetDataTag(ctx context.Context, id int) (*DataTag, error) {
	raw, err := c.JSON(ctx, http.MethodGet, fmt.Sprintf("/datatag/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		DataTag DataTag `json:"dataTag"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	return &envelope.DataTag, nil
}
