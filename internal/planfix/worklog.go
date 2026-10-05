package planfix

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/6insanes/planfix-cli/internal/config"
)

// WorklogEntry is one time-tracking record to write.
type WorklogEntry struct {
	Date        string // dd-MM-yyyy
	From        string // HH:MM
	To          string // HH:MM
	WorkTypeKey int    // directory entry key, 0 = omit
}

// WorklogRow is one entry for display.
type WorklogRow struct {
	Date     string
	From     string
	To       string
	Hours    float64
	WorkType string
	Author   string
}

// DataTagEntry is a comment carrying a data tag and its custom field values.
type DataTagEntry struct {
	Comment
	DataTag struct {
		ID int `json:"id"`
	} `json:"dataTag"`
	CustomFieldData []map[string]any `json:"customFieldData,omitempty"`
}

// CreateWorklogEntry writes a data-tag comment carrying the worklog fields.
func (c *Client) CreateWorklogEntry(ctx context.Context, taskID int, meta *config.WorklogMeta, e WorklogEntry) (int, []byte, error) {
	if e.WorkTypeKey < 0 {
		return 0, nil, fmt.Errorf("work type key %d must not be negative", e.WorkTypeKey)
	}
	if e.WorkTypeKey != 0 && meta.FieldWorkType == 0 {
		return 0, nil, fmt.Errorf(
			"work type key %d requested but the account has no work type field", e.WorkTypeKey)
	}
	cfd := []map[string]any{
		{"field": map[string]any{"id": meta.FieldDate}, "value": map[string]any{"date": e.Date}},
		{
			"field": map[string]any{"id": meta.FieldTime},
			"value": map[string]any{
				"from": map[string]any{"time": e.From},
				"to":   map[string]any{"time": e.To},
			},
		},
	}
	if e.WorkTypeKey > 0 && meta.FieldWorkType > 0 {
		cfd = append(cfd, map[string]any{
			"field": map[string]any{"id": meta.FieldWorkType},
			"value": map[string]any{"id": e.WorkTypeKey},
		})
	}
	body := map[string]any{
		"type":            "DataTag",
		"dataTag":         map[string]any{"id": meta.DataTagID},
		"customFieldData": cfd,
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

// worklogListFields are requested from comment/list so data-tag entries can
// be identified and parsed.
const worklogListFields = "id,text,type,dataTag,customFieldData,author,timestamp"

// apiDateLayout is the worklog date format (dd-MM-yyyy).
const apiDateLayout = "02-01-2006"

// ListWorklog returns the task's worklog rows: comments whose data tag is
// the worklog meta's tag, parsed into date/period/work type. Comments are
// fetched in pages of commentPageSize until a short page, then rows are
// sorted by date and start time.
func (c *Client) ListWorklog(ctx context.Context, taskID int, meta *config.WorklogMeta) ([]WorklogRow, []byte, error) {
	var (
		comments []DataTagEntry
		raw      []byte
		pages    int
	)
	for offset := 0; ; offset += commentPageSize {
		pageRaw, err := c.listCommentsPage(ctx, taskID, worklogListFields, offset, commentPageSize)
		if err != nil {
			return nil, nil, err
		}
		var envelope struct {
			Comments []DataTagEntry `json:"comments"`
		}
		if err := json.Unmarshal(pageRaw, &envelope); err != nil {
			return nil, pageRaw, err
		}
		if pages == 0 {
			raw = pageRaw
		}
		pages++
		comments = append(comments, envelope.Comments...)
		if len(envelope.Comments) < commentPageSize {
			break
		}
	}
	if pages > 1 {
		merged, err := json.Marshal(struct {
			Result   string         `json:"result"`
			Comments []DataTagEntry `json:"comments"`
		}{Result: "success", Comments: comments})
		if err != nil {
			return nil, raw, err
		}
		raw = merged
	}
	rows := make([]WorklogRow, 0, len(comments))
	for _, e := range comments {
		if e.DataTag.ID != meta.DataTagID {
			continue
		}
		rows = append(rows, worklogRow(e, meta))
	}
	sortWorklogRows(rows)
	return rows, raw, nil
}

// sortWorklogRows orders rows by calendar date, then by start time.
func sortWorklogRows(rows []WorklogRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		di, dj := parseWorklogDate(rows[i].Date), parseWorklogDate(rows[j].Date)
		if !di.Equal(dj) {
			return di.Before(dj)
		}
		return rows[i].From < rows[j].From
	})
}

// parseWorklogDate parses dd-MM-yyyy; malformed dates sort as the zero time.
func parseWorklogDate(s string) time.Time {
	d, err := time.Parse(apiDateLayout, s)
	if err != nil {
		return time.Time{}
	}
	return d
}

// worklogRow maps one data-tag comment's customFieldData onto a WorklogRow.
func worklogRow(e DataTagEntry, meta *config.WorklogMeta) WorklogRow {
	row := WorklogRow{Author: e.Author.Name}
	for _, item := range e.CustomFieldData {
		id := nestedInt(item, "field", "id")
		value, _ := item["value"].(map[string]any)
		switch {
		case id == meta.FieldDate && id != 0:
			row.Date = nestedStr(item, "value", "date")
		case id == meta.FieldTime && id != 0:
			row.From = nestedStr(item, "value", "from", "time")
			row.To = nestedStr(item, "value", "to", "time")
		case meta.FieldWorkType != 0 && id == meta.FieldWorkType:
			row.WorkType = directoryLabel(value)
		}
	}
	row.Hours = hoursBetween(row.From, row.To)
	return row
}

// nestedInt reads m[key][sub] as an int; 0 when absent or non-numeric.
func nestedInt(m map[string]any, key, sub string) int {
	obj, _ := m[key].(map[string]any)
	n, _ := obj[sub].(float64)
	return int(n)
}

// nestedStr walks m through the given keys and returns the final string
// value, "" when any hop is missing or not a string.
func nestedStr(m map[string]any, keys ...string) string {
	cur := any(m)
	for _, k := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = obj[k]
	}
	s, _ := cur.(string)
	return s
}

// directoryLabel renders a directory-entry value: the name when the API
// returns one, otherwise the numeric id.
func directoryLabel(v map[string]any) string {
	if s, _ := v["name"].(string); s != "" {
		return s
	}
	if n, _ := v["id"].(float64); n != 0 {
		return strconv.Itoa(int(n))
	}
	return ""
}

// hoursBetween computes worked hours from HH:MM bounds. Malformed bounds
// yield 0; a wrapped (overnight) interval adds 24h.
func hoursBetween(from, to string) float64 {
	f, err1 := time.Parse("15:04", from)
	t, err2 := time.Parse("15:04", to)
	if err1 != nil || err2 != nil {
		return 0
	}
	d := t.Sub(f)
	if d < 0 {
		d += 24 * time.Hour
	}
	return d.Hours()
}
