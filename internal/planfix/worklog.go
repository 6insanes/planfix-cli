package planfix

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/6insanes/planfix-cli/internal/config"
)

// WorklogEntry is one time-tracking record to write.
type WorklogEntry struct {
	Date     string // dd-MM-yyyy
	From     string // HH:MM
	To       string // HH:MM
	WorkType string // list value name or directory entry key, "" = omit
	Note     string // work details text, "" = omit
}

// WorklogRow is one entry for display.
type WorklogRow struct {
	Date     string
	From     string
	To       string
	Hours    float64
	WorkType string
	Note     string
	Author   string
}

// DataTagEntry is one data tag entry returned by /datatag/{id}/entry/list.
type DataTagEntry struct {
	Key             int              `json:"key"`
	CommentID       int              `json:"commentId"`
	CustomFieldData []map[string]any `json:"customFieldData,omitempty"`
}

// CreateWorklogEntry writes a data tag entry to the task's worklog tag and
// returns the created entry's key (falling back to its comment id).
func (c *Client) CreateWorklogEntry(ctx context.Context, taskID int, meta *config.WorklogMeta, e WorklogEntry) (int, []byte, error) {
	if e.WorkType != "" && meta.FieldWorkType == 0 {
		return 0, nil, fmt.Errorf(
			"work type %q requested but data tag %d has no work type field", e.WorkType, meta.DataTagID)
	}
	timeValue := any(map[string]any{
		"from": map[string]any{"time": e.From},
		"to":   map[string]any{"time": e.To},
	})
	if meta.TimeInMinutes {
		timeValue = minutesBetween(e.From, e.To)
	}
	cfd := []map[string]any{
		{"field": map[string]any{"id": meta.FieldDate}, "value": map[string]any{"date": e.Date}},
		{"field": map[string]any{"id": meta.FieldTime}, "value": timeValue},
	}
	if e.WorkType != "" {
		v, err := workTypeValue(meta, e.WorkType)
		if err != nil {
			return 0, nil, err
		}
		cfd = append(cfd, map[string]any{
			"field": map[string]any{"id": meta.FieldWorkType},
			"value": v,
		})
	}
	if e.Note != "" && meta.FieldNote != 0 {
		cfd = append(cfd, map[string]any{
			"field": map[string]any{"id": meta.FieldNote},
			"value": e.Note,
		})
	}
	body := map[string]any{
		"dataTag": map[string]any{"id": meta.DataTagID},
		"items":   []map[string]any{{"customFieldData": cfd}},
	}
	raw, err := c.JSON(ctx, http.MethodPost, fmt.Sprintf("/task/%d/datatags/", taskID), body)
	if err != nil {
		return 0, nil, err
	}
	var envelope struct {
		Keys      []int `json:"keys"`
		CommentID int   `json:"commentId"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, raw, err
	}
	if len(envelope.Keys) > 0 {
		return envelope.Keys[0], raw, nil
	}
	return envelope.CommentID, raw, nil
}

// workTypeValue maps a --work-type argument onto the field's value shape:
// a bare enum value name for list fields, an {id} object for directory-entry
// fields. List names are validated against the tag's enum values when known.
func workTypeValue(meta *config.WorklogMeta, s string) (any, error) {
	if meta.WorkTypeDirectory != 0 {
		id, err := strconv.Atoi(s)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf(
				"invalid work type %q: work type is a directory entry on this account, pass its numeric key", s)
		}
		return map[string]any{"id": id}, nil
	}
	if len(meta.WorkTypeValues) > 0 {
		for _, v := range meta.WorkTypeValues {
			if strings.EqualFold(v, s) {
				return v, nil
			}
		}
		return nil, fmt.Errorf("invalid work type %q: allowed values: %s",
			s, strings.Join(meta.WorkTypeValues, ", "))
	}
	return s, nil
}

// worklogEntryFields are requested from /datatag/{id}/entry/list: the
// entry identity plus the meta's custom fields, which are requested by
// their numeric field ids.
func worklogEntryFields(meta *config.WorklogMeta) string {
	ids := []string{"key", "commentId", strconv.Itoa(meta.FieldDate), strconv.Itoa(meta.FieldTime)}
	if meta.FieldWorkType != 0 {
		ids = append(ids, strconv.Itoa(meta.FieldWorkType))
	}
	if meta.FieldNote != 0 {
		ids = append(ids, strconv.Itoa(meta.FieldNote))
	}
	return strings.Join(ids, ",")
}

// apiDateLayout is the worklog date format (dd-MM-yyyy).
const apiDateLayout = "02-01-2006"

// ListWorklog returns the task's worklog rows: data tag entries of the
// meta's tag mapped onto date/period/work type and joined with the authors
// of their comments. Entries are fetched in pages of commentPageSize until
// a short page, then rows are sorted by date and start time.
func (c *Client) ListWorklog(ctx context.Context, taskID int, meta *config.WorklogMeta) ([]WorklogRow, []byte, error) {
	authors, err := c.commentAuthors(ctx, taskID)
	if err != nil {
		return nil, nil, err
	}
	entries, raw, err := c.listDataTagEntries(ctx, meta.DataTagID, taskID, worklogEntryFields(meta))
	if err != nil {
		return nil, nil, err
	}
	rows := make([]WorklogRow, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, worklogRow(e, meta, authors[e.CommentID]))
	}
	sortWorklogRows(rows)
	return rows, raw, nil
}

// commentAuthors maps comment ids to author names for one task.
func (c *Client) commentAuthors(ctx context.Context, taskID int) (map[int]string, error) {
	authors := map[int]string{}
	for offset := 0; ; offset += commentPageSize {
		pageRaw, err := c.listCommentsPage(ctx, taskID, "id,owner", offset, commentPageSize)
		if err != nil {
			return nil, err
		}
		var envelope struct {
			Comments []Comment `json:"comments"`
		}
		if err := json.Unmarshal(pageRaw, &envelope); err != nil {
			return nil, err
		}
		for _, cm := range envelope.Comments {
			authors[cm.ID] = cm.Author.Name
		}
		if len(envelope.Comments) < commentPageSize {
			break
		}
	}
	return authors, nil
}

// listDataTagEntries pages through POST /datatag/{id}/entry/list for one
// task. It returns the typed entries and the raw response body, which is
// merged verbatim across pages when more than one page is fetched.
func (c *Client) listDataTagEntries(ctx context.Context, dataTagID, taskID int, fields string) ([]DataTagEntry, []byte, error) {
	var (
		entries []DataTagEntry
		raws    []json.RawMessage
		raw     []byte
		pages   int
	)
	for offset := 0; ; offset += commentPageSize {
		body := map[string]any{"offset": offset, "pageSize": commentPageSize, "taskId": taskID}
		if fields != "" {
			body["fields"] = fields
		}
		pageRaw, err := c.JSON(ctx, http.MethodPost, fmt.Sprintf("/datatag/%d/entry/list", dataTagID), body)
		if err != nil {
			return nil, nil, err
		}
		var envelope struct {
			DataTagEntries []json.RawMessage `json:"dataTagEntries"`
		}
		if err := json.Unmarshal(pageRaw, &envelope); err != nil {
			return nil, pageRaw, err
		}
		for _, item := range envelope.DataTagEntries {
			var e DataTagEntry
			if err := json.Unmarshal(item, &e); err != nil {
				return nil, pageRaw, err
			}
			entries = append(entries, e)
		}
		if pages == 0 {
			raw = pageRaw
		}
		pages++
		raws = append(raws, envelope.DataTagEntries...)
		if len(envelope.DataTagEntries) < commentPageSize {
			break
		}
	}
	if pages > 1 {
		merged, err := json.Marshal(struct {
			Result         string            `json:"result"`
			DataTagEntries []json.RawMessage `json:"dataTagEntries"`
		}{Result: "success", DataTagEntries: raws})
		if err != nil {
			return nil, raw, err
		}
		raw = merged
	}
	return entries, raw, nil
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

// worklogRow maps one data tag entry's customFieldData onto a WorklogRow.
func worklogRow(e DataTagEntry, meta *config.WorklogMeta, author string) WorklogRow {
	row := WorklogRow{Author: author}
	minutes := 0.0
	for _, item := range e.CustomFieldData {
		id := nestedInt(item, "field", "id")
		switch {
		case id == meta.FieldDate && id != 0:
			row.Date = nestedStr(item, "value", "date")
		case id == meta.FieldTime && id != 0:
			if meta.TimeInMinutes {
				minutes = numberValue(item, "value")
			} else {
				row.From = nestedStr(item, "value", "from", "time")
				row.To = nestedStr(item, "value", "to", "time")
			}
		case meta.FieldWorkType != 0 && id == meta.FieldWorkType:
			row.WorkType = valueLabel(item["value"])
		case meta.FieldNote != 0 && id == meta.FieldNote:
			row.Note = valueLabel(item["value"])
		}
	}
	if meta.TimeInMinutes {
		row.Hours = minutes / 60
	} else {
		row.Hours = hoursBetween(row.From, row.To)
	}
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

// valueLabel renders a custom-field value for display: enum/text strings
// as-is, objects by name or directory value, numbers by value.
func valueLabel(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case map[string]any:
		return directoryLabel(t)
	}
	return ""
}

// directoryLabel renders an object value: the name when the API returns one,
// then the directory entry's value, otherwise the numeric id.
func directoryLabel(v map[string]any) string {
	if s, _ := v["name"].(string); s != "" {
		return s
	}
	if s, _ := v["value"].(string); s != "" {
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

// minutesBetween returns the whole minutes between two HH:MM bounds.
func minutesBetween(from, to string) int {
	return int(math.Round(hoursBetween(from, to) * 60))
}

// numberValue reads m[key] as a bare JSON number or numeric string; 0
// when absent or non-numeric.
func numberValue(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}
