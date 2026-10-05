package planfix

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// datatagFields are the system fields requested from /datatag/list and
// /datatag/{id} so discovery sees each tag's custom fields.
const datatagFields = "id,name,group,fields"

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

// DataTagList is the POST /datatag/list payload.
type DataTagList struct {
	DataTags []DataTag `json:"dataTags"`
}

// ListDataTags lists the account's data tags. The spec schema names the
// list "dataTags" but its example uses "dataTag"; both are accepted.
func (c *Client) ListDataTags(ctx context.Context) (*DataTagList, error) {
	body := map[string]any{"offset": 0, "pageSize": 100, "fields": datatagFields}
	raw, err := c.JSON(ctx, http.MethodPost, "/datatag/list", body)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		DataTags []DataTag `json:"dataTags"`
		DataTag  []DataTag `json:"dataTag"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	tags := envelope.DataTags
	if len(tags) == 0 {
		tags = envelope.DataTag
	}
	return &DataTagList{DataTags: tags}, nil
}

// GetDataTag fetches one data tag with its fields.
func (c *Client) GetDataTag(ctx context.Context, id int) (*DataTag, error) {
	path := fmt.Sprintf("/datatag/%d?%s", id, url.Values{"fields": {datatagFields}}.Encode())
	raw, err := c.JSON(ctx, http.MethodGet, path, nil)
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
