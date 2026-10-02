package planfix

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"planfix-cli/internal/config"
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
		_, _ = w.Write([]byte(`{"result":"success","id":77}`))
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
	if gotMethod != http.MethodPost || gotPath != "/rest/task/1/comment" {
		t.Errorf("request = %s %s, want POST /rest/task/1/comment", gotMethod, gotPath)
	}
	if body["type"] != "DataTag" {
		t.Errorf("type = %v, want DataTag", body["type"])
	}
	tag, _ := body["dataTag"].(map[string]any)
	if tag["id"] != float64(123) {
		t.Errorf("dataTag id = %v, want 123", tag["id"])
	}
	cfd, _ := body["customFieldData"].([]any)
	if len(cfd) != 3 {
		t.Fatalf("customFieldData len = %d, want 3: %v", len(cfd), body["customFieldData"])
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
		t.Errorf("id = %d, want 77", id)
	}
	if !strings.Contains(string(raw), `"id":77`) {
		t.Errorf("raw = %s", raw)
	}
}

func TestCreateWorklogEntryOmitsWorkType(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = w.Write([]byte(`{"result":"success","id":5}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	meta := worklogTestMeta()
	meta.FieldWorkType = 0 // discovered without a work-type field
	if _, _, err := c.CreateWorklogEntry(context.Background(), 1, meta, WorklogEntry{
		Date: "02-10-2026", From: "10:00", To: "12:00", WorkTypeKey: 1,
	}); err != nil {
		t.Fatalf("CreateWorklogEntry() error = %v", err)
	}
	cfd, _ := body["customFieldData"].([]any)
	if len(cfd) != 2 {
		t.Errorf("customFieldData len = %d, want 2 (work type omitted)", len(cfd))
	}

	// WorkTypeKey 0 with a mapped field must also omit the entry.
	body = nil
	if _, _, err := c.CreateWorklogEntry(context.Background(), 1, worklogTestMeta(), WorklogEntry{
		Date: "02-10-2026", From: "10:00", To: "12:00",
	}); err != nil {
		t.Fatalf("CreateWorklogEntry() error = %v", err)
	}
	cfd, _ = body["customFieldData"].([]any)
	if len(cfd) != 2 {
		t.Errorf("customFieldData len = %d, want 2 (WorkTypeKey 0)", len(cfd))
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

func TestListWorklog(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/task/1/comment/list" {
			t.Errorf("path = %s, want /rest/task/1/comment/list", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_, _ = w.Write([]byte(`{"result":"success","comments":[
			{"id":1,"text":"plain comment","type":"comment","author":{"id":3,"name":"Ann"}},
			{"id":2,"type":"DataTag","dataTag":{"id":999},"author":{"id":4,"name":"Other"},
			 "customFieldData":[{"field":{"id":456},"value":{"date":"01-10-2026"}}]},
			{"id":3,"type":"DataTag","dataTag":{"id":123},"author":{"id":3,"name":"Ann"},
			 "customFieldData":[
				{"field":{"id":456},"value":{"date":"02-10-2026"}},
				{"field":{"id":789},"value":{"from":{"time":"10:00"},"to":{"time":"12:00"}}},
				{"field":{"id":101},"value":{"id":1,"name":"Dev"}}
			]},
			{"id":4,"type":"DataTag","dataTag":{"id":123},"author":{"id":4,"name":"Bob"},
			 "customFieldData":[
				{"field":{"id":456},"value":{"date":"03-10-2026"}},
				{"field":{"id":789},"value":{"from":{"time":"09:00"},"to":{"time":"09:30"}}},
				{"field":{"id":101},"value":{"id":7}}
			]}
		]}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	rows, raw, err := c.ListWorklog(context.Background(), 1, worklogTestMeta())
	if err != nil {
		t.Fatalf("ListWorklog() error = %v", err)
	}
	fields, _ := body["fields"].(string)
	for _, want := range []string{"dataTag", "customFieldData", "author"} {
		if !strings.Contains(fields, want) {
			t.Errorf("fields = %q, want mention of %q", fields, want)
		}
	}
	if !strings.Contains(string(raw), `"comments"`) {
		t.Errorf("raw = %s", raw)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want 2 entries (plain + foreign tag filtered)", rows)
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

func TestListWorklogNoMatchingEntries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","comments":[
			{"id":1,"text":"plain","type":"comment"}
		]}`))
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
