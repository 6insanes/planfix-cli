package planfix

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/6insanes/planfix-cli/internal/config"
)

func worklogTestMeta() *config.WorklogMeta {
	return &config.WorklogMeta{
		DataTagID:     123,
		FieldDate:     456,
		FieldTime:     789,
		FieldWorkType: 101,
	}
}

func TestCreateWorklogEntry(t *testing.T) {
	var gotMethod, gotPath string
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		_, _ = w.Write([]byte(`{"result":"success","keys":[77],"commentId":55}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	id, raw, err := c.CreateWorklogEntry(context.Background(), 1, worklogTestMeta(), WorklogEntry{
		Date:        "02-10-2026",
		From:        "10:00",
		To:          "12:00",
		WorkTypeKey: 1,
	})
	if err != nil {
		t.Fatalf("CreateWorklogEntry() error = %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/rest/task/1/datatags/" {
		t.Errorf("request = %s %s, want POST /rest/task/1/datatags/", gotMethod, gotPath)
	}
	tag, _ := body["dataTag"].(map[string]any)
	if tag["id"] != float64(123) {
		t.Errorf("dataTag id = %v, want 123", tag["id"])
	}
	items, _ := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %#v, want one entry", body["items"])
	}
	cfd, _ := items[0].(map[string]any)["customFieldData"].([]any)
	if len(cfd) != 3 {
		t.Fatalf("customFieldData len = %d, want 3: %v", len(cfd), body["items"])
	}
	dateItem := cfd[0].(map[string]any)
	if dateItem["field"].(map[string]any)["id"] != float64(456) {
		t.Errorf("date field id = %v, want 456", dateItem["field"])
	}
	dateVal := dateItem["value"].(map[string]any)
	if dateVal["date"] != "02-10-2026" {
		t.Errorf("date value = %v, want 02-10-2026", dateVal["date"])
	}
	timeItem := cfd[1].(map[string]any)
	if timeItem["field"].(map[string]any)["id"] != float64(789) {
		t.Errorf("time field id = %v, want 789", timeItem["field"])
	}
	timeVal := timeItem["value"].(map[string]any)
	from := timeVal["from"].(map[string]any)
	to := timeVal["to"].(map[string]any)
	if from["time"] != "10:00" || to["time"] != "12:00" {
		t.Errorf("period = %v/%v, want 10:00/12:00", from["time"], to["time"])
	}
	wtItem := cfd[2].(map[string]any)
	if wtItem["field"].(map[string]any)["id"] != float64(101) {
		t.Errorf("work type field id = %v, want 101", wtItem["field"])
	}
	wtVal := wtItem["value"].(map[string]any)
	if wtVal["id"] != float64(1) {
		t.Errorf("work type value = %v, want 1", wtVal["id"])
	}
	if id != 77 {
		t.Errorf("id = %d, want 77 (first entry key)", id)
	}
	if !strings.Contains(string(raw), `"commentId":55`) {
		t.Errorf("raw = %s", raw)
	}
}

func TestCreateWorklogEntryRequiresWorkTypeField(t *testing.T) {
	var calls int
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = w.Write([]byte(`{"result":"success","keys":[5],"commentId":5}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	meta := worklogTestMeta()
	meta.FieldWorkType = 0 // discovered without a work-type field
	if _, _, err := c.CreateWorklogEntry(context.Background(), 1, meta, WorklogEntry{
		Date: "02-10-2026", From: "10:00", To: "12:00", WorkTypeKey: 1,
	}); err == nil {
		t.Fatal("work type without field: error = nil, want failure")
	} else if !strings.Contains(err.Error(), "work type") {
		t.Errorf("error = %q, want mention of work type", err)
	}
	if calls != 0 {
		t.Errorf("API calls = %d, want 0 (rejected before the request)", calls)
	}

	// WorkTypeKey 0 with a mapped field omits the entry.
	if _, _, err := c.CreateWorklogEntry(context.Background(), 1, worklogTestMeta(), WorklogEntry{
		Date: "02-10-2026", From: "10:00", To: "12:00",
	}); err != nil {
		t.Fatalf("CreateWorklogEntry() error = %v", err)
	}
	items, _ := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %#v, want one entry", body["items"])
	}
	cfd, _ := items[0].(map[string]any)["customFieldData"].([]any)
	if len(cfd) != 2 {
		t.Errorf("customFieldData len = %d, want 2 (WorkTypeKey 0)", len(cfd))
	}
}

func TestCreateWorklogEntryRejectsNegativeWorkType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("API must not be called for a negative work type key")
		_, _ = w.Write([]byte(`{"result":"success","id":5}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	if _, _, err := c.CreateWorklogEntry(context.Background(), 1, worklogTestMeta(), WorklogEntry{
		Date: "02-10-2026", From: "10:00", To: "12:00", WorkTypeKey: -1,
	}); err == nil {
		t.Fatal("negative work type: error = nil, want failure")
	} else if !strings.Contains(err.Error(), "negative") {
		t.Errorf("error = %q, want mention of negative", err)
	}
}

func TestCreateWorklogEntryAPIErrorReturnsNilRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"result":"failure","code":5,"message":"Access denied"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	id, raw, err := c.CreateWorklogEntry(context.Background(), 1, worklogTestMeta(), WorklogEntry{
		Date: "02-10-2026", From: "10:00", To: "12:00",
	})
	if err == nil {
		t.Fatal("CreateWorklogEntry() error = nil, want API error")
	}
	if id != 0 || raw != nil {
		t.Errorf("id/raw = %d/%v, want 0/nil on error", id, raw)
	}
}

func TestCreateWorklogEntryMinutesField(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = w.Write([]byte(`{"result":"success","keys":[7],"commentId":7}`))
	}))
	defer srv.Close()

	meta := worklogTestMeta()
	meta.TimeInMinutes = true
	c := newTestClient(t, srv)
	if _, _, err := c.CreateWorklogEntry(context.Background(), 1, meta, WorklogEntry{
		Date: "02-10-2026", From: "10:00", To: "11:30",
	}); err != nil {
		t.Fatalf("CreateWorklogEntry() error = %v", err)
	}
	items, _ := body["items"].([]any)
	cfd, _ := items[0].(map[string]any)["customFieldData"].([]any)
	if len(cfd) != 2 {
		t.Fatalf("customFieldData = %v, want 2 items", cfd)
	}
	if got := cfd[1].(map[string]any)["value"]; got != float64(90) {
		t.Errorf("time value = %v, want 90 minutes", got)
	}
}

func TestListWorklog(t *testing.T) {
	var entryBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/task/1/comments/list":
			_, _ = w.Write([]byte(`{"result":"success","comments":[
				{"id":30,"owner":{"id":"user:3","name":"Ann"}},
				{"id":40,"owner":{"id":"user:4","name":"Bob"}}
			]}`))
		case "/rest/datatag/123/entry/list":
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &entryBody)
			_, _ = w.Write([]byte(`{"result":"success","dataTagEntries":[
				{"key":1,"commentId":30,"customFieldData":[
					{"field":{"id":456},"value":{"date":"02-10-2026"}},
					{"field":{"id":789},"value":{"from":{"time":"10:00"},"to":{"time":"12:00"}}},
					{"field":{"id":101},"value":{"id":1,"name":"Dev"}}
				]},
				{"key":2,"commentId":40,"customFieldData":[
					{"field":{"id":456},"value":{"date":"03-10-2026"}},
					{"field":{"id":789},"value":{"from":{"time":"09:00"},"to":{"time":"09:30"}}},
					{"field":{"id":101},"value":{"id":7}}
				]}
			]}`))
		default:
			t.Errorf("unexpected path = %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	rows, raw, err := c.ListWorklog(context.Background(), 1, worklogTestMeta())
	if err != nil {
		t.Fatalf("ListWorklog() error = %v", err)
	}
	fields, _ := entryBody["fields"].(string)
	for _, want := range []string{"key", "commentId", "456", "789", "101"} {
		if !strings.Contains(fields, want) {
			t.Errorf("fields = %q, want mention of %q", fields, want)
		}
	}
	if entryBody["taskId"] != float64(1) {
		t.Errorf("entry body taskId = %v, want 1", entryBody["taskId"])
	}
	if !strings.Contains(string(raw), `"dataTagEntries"`) {
		t.Errorf("raw = %s", raw)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want 2 entries", rows)
	}
	r0 := rows[0]
	if r0.Date != "02-10-2026" || r0.From != "10:00" || r0.To != "12:00" {
		t.Errorf("row0 interval = %s %s-%s", r0.Date, r0.From, r0.To)
	}
	if r0.Hours != 2 {
		t.Errorf("row0 hours = %v, want 2", r0.Hours)
	}
	if r0.WorkType != "Dev" {
		t.Errorf("row0 work type = %q, want Dev", r0.WorkType)
	}
	if r0.Author != "Ann" {
		t.Errorf("row0 author = %q, want Ann", r0.Author)
	}
	r1 := rows[1]
	if r1.Hours != 0.5 {
		t.Errorf("row1 hours = %v, want 0.5", r1.Hours)
	}
	if r1.WorkType != "7" {
		t.Errorf("row1 work type = %q, want 7 (id fallback)", r1.WorkType)
	}
	if r1.Author != "Bob" {
		t.Errorf("row1 author = %q, want Bob", r1.Author)
	}
}

func TestListWorklogPagesBeyond100Entries(t *testing.T) {
	var offsets, pageSizes []float64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/task/1/comments/list" {
			_, _ = w.Write([]byte(`{"result":"success","comments":[]}`))
			return
		}
		if r.URL.Path != "/rest/datatag/123/entry/list" {
			t.Errorf("unexpected path = %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]any
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		off, _ := body["offset"].(float64)
		ps, _ := body["pageSize"].(float64)
		offsets = append(offsets, off)
		pageSizes = append(pageSizes, ps)

		n, start := 100, 1
		if off > 0 {
			n, start = 1, 101 // short page ends the loop
		}
		parts := make([]string, 0, n)
		for i := 0; i < n; i++ {
			parts = append(parts, fmt.Sprintf(
				`{"key":%d,"commentId":0,
				 "customFieldData":[
					{"field":{"id":456},"value":{"date":"02-10-2026"}},
					{"field":{"id":789},"value":{"from":{"time":"10:00"},"to":{"time":"12:00"}}}
				 ]}`, start+i))
		}
		_, _ = fmt.Fprintf(w, `{"result":"success","dataTagEntries":[%s]}`, strings.Join(parts, ","))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	rows, raw, err := c.ListWorklog(context.Background(), 1, worklogTestMeta())
	if err != nil {
		t.Fatalf("ListWorklog() error = %v", err)
	}
	if len(rows) != 101 {
		t.Errorf("rows = %d, want 101 (paged past the first 100)", len(rows))
	}
	if len(offsets) != 2 || offsets[0] != 0 || offsets[1] != 100 {
		t.Errorf("offsets = %v, want [0 100]", offsets)
	}
	for _, ps := range pageSizes {
		if ps != 100 {
			t.Errorf("pageSize = %v, want 100", ps)
		}
	}
	var merged struct {
		DataTagEntries []json.RawMessage `json:"dataTagEntries"`
	}
	if err := json.Unmarshal(raw, &merged); err != nil {
		t.Fatalf("raw is not JSON: %v", err)
	}
	if len(merged.DataTagEntries) != 101 {
		t.Errorf("raw entries = %d, want 101 (pages merged)", len(merged.DataTagEntries))
	}
}

func TestListWorklogSortsByDateAndFrom(t *testing.T) {
	// Deliberately unordered; a string sort would put 02-10-2026 before
	// 15-12-2025 even though 2025 comes first chronologically.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/task/1/comments/list" {
			_, _ = w.Write([]byte(`{"result":"success","comments":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":"success","dataTagEntries":[
			{"key":1,"commentId":0,
			 "customFieldData":[
				{"field":{"id":456},"value":{"date":"02-10-2026"}},
				{"field":{"id":789},"value":{"from":{"time":"14:00"},"to":{"time":"15:00"}}}
			 ]},
			{"key":2,"commentId":0,
			 "customFieldData":[
				{"field":{"id":456},"value":{"date":"15-12-2025"}},
				{"field":{"id":789},"value":{"from":{"time":"09:00"},"to":{"time":"10:00"}}}
			 ]},
			{"key":3,"commentId":0,
			 "customFieldData":[
				{"field":{"id":456},"value":{"date":"02-10-2026"}},
				{"field":{"id":789},"value":{"from":{"time":"09:00"},"to":{"time":"10:00"}}}
			 ]}
		]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	rows, _, err := c.ListWorklog(context.Background(), 1, worklogTestMeta())
	if err != nil {
		t.Fatalf("ListWorklog() error = %v", err)
	}
	want := []struct{ date, from string }{
		{"15-12-2025", "09:00"},
		{"02-10-2026", "09:00"},
		{"02-10-2026", "14:00"},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %d, want %d", len(rows), len(want))
	}
	for i, w := range want {
		if rows[i].Date != w.date || rows[i].From != w.from {
			t.Errorf("row%d = %s %s, want %s %s", i, rows[i].Date, rows[i].From, w.date, w.from)
		}
	}
}

func TestListWorklogNoEntries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/task/1/comments/list" {
			_, _ = w.Write([]byte(`{"result":"success","comments":[{"id":1,"owner":{"id":"user:3","name":"Ann"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":"success","dataTagEntries":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	rows, _, err := c.ListWorklog(context.Background(), 1, worklogTestMeta())
	if err != nil {
		t.Fatalf("ListWorklog() error = %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("rows = %+v, want none", rows)
	}
}

func TestListWorklogAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"result":"failure","code":5,"message":"Access denied"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	rows, raw, err := c.ListWorklog(context.Background(), 1, worklogTestMeta())
	if err == nil {
		t.Fatal("ListWorklog() error = nil, want API error")
	}
	if rows != nil || raw != nil {
		t.Errorf("rows/raw = %v/%v, want nil/nil on error", rows, raw)
	}
}

func TestListWorklogMinutesField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/task/1/comments/list":
			_, _ = w.Write([]byte(`{"result":"success","comments":[{"id":30,"owner":{"id":"user:3","name":"Ann"}}]}`))
		case "/rest/datatag/123/entry/list":
			_, _ = w.Write([]byte(`{"result":"success","dataTagEntries":[
				{"key":1,"commentId":30,"customFieldData":[
					{"field":{"id":456},"value":{"date":"02-10-2026"}},
					{"field":{"id":789},"value":90}
				]}
			]}`))
		default:
			t.Errorf("unexpected path = %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	meta := worklogTestMeta()
	meta.TimeInMinutes = true
	c := newTestClient(t, srv)
	rows, _, err := c.ListWorklog(context.Background(), 1, meta)
	if err != nil {
		t.Fatalf("ListWorklog() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want one entry", rows)
	}
	r := rows[0]
	if r.Date != "02-10-2026" || r.Hours != 1.5 {
		t.Errorf("row = %+v, want date 02-10-2026 and 1.5 hours", r)
	}
	if r.From != "" || r.To != "" {
		t.Errorf("row interval = %q-%q, want empty for a minutes field", r.From, r.To)
	}
	if r.Author != "Ann" {
		t.Errorf("author = %q, want Ann", r.Author)
	}
}

func TestHoursBetween(t *testing.T) {
	tests := []struct {
		from, to string
		want     float64
	}{
		{"10:00", "12:00", 2},
		{"09:00", "09:30", 0.5},
		{"10:00", "10:00", 0},
		{"23:00", "01:00", 2}, // overnight wrap
		{"", "", 0},
		{"bogus", "12:00", 0},
	}
	for _, tt := range tests {
		if got := hoursBetween(tt.from, tt.to); got != tt.want {
			t.Errorf("hoursBetween(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}
