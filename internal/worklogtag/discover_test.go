package worklogtag

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/6insanes/planfix-cli/internal/planfix"
)

func TestMatchName(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"Фактическое время", 2},
		{"Фактические часы", 2},
		{"Actual work time", 2},
		{"Worklog", 2},
		{"Unplanned worklog", 2},
		{"Время работы", 1},
		{"Учёт времени", 1},
		{"Планируемое время работы", -1},
		{"Planned time", -1},
		{"Список задач", 0},
	}
	for _, tt := range tests {
		if got := MatchName(tt.name); got != tt.want {
			t.Errorf("MatchName(%q) = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func newTestClient(t *testing.T, srv *httptest.Server) *planfix.Client {
	t.Helper()
	c, err := planfix.New("x", "tok")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = srv.URL + "/rest"
	return c
}

func TestResolveByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/datatag/2":
			if r.Method != http.MethodGet {
				t.Errorf("method = %s, want GET", r.Method)
			}
			_, _ = w.Write([]byte(`{"result":"success","dataTag":{"id":2,"name":"Фактическое время","fields":[
				{"id":10,"name":"Дата","type":3},
				{"id":11,"name":"Время","type":6},
				{"id":12,"name":"Вид работ","type":9,"directoryId":3},
				{"id":13,"name":"Сотрудник","type":11},
				{"id":14,"name":"Детали работ","type":2}
			]}}`))
		default:
			t.Errorf("unexpected path = %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	meta, err := Resolve(context.Background(), newTestClient(t, srv), "2")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if meta.DataTagID != 2 || meta.FieldDate != 10 || meta.FieldTime != 11 ||
		meta.FieldWorkType != 12 || meta.WorkTypeDirectory != 3 ||
		meta.FieldEmployee != 13 || meta.FieldNote != 14 {
		t.Errorf("meta = %+v", meta)
	}
}

func TestResolveByName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/datatag/list":
			_, _ = w.Write([]byte(`{"result":"success","dataTags":[
				{"id":2,"name":"Фактическое время Support"},
				{"id":3,"name":"Фактическое время Classifieds"}
			]}`))
		case "/rest/datatag/3":
			_, _ = w.Write([]byte(`{"result":"success","dataTag":{"id":3,"name":"Фактическое время Classifieds","fields":[
				{"id":10,"name":"Дата","type":3},
				{"id":11,"name":"Минут потрачено","type":1}
			]}}`))
		default:
			t.Errorf("unexpected path = %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Exact name (any case) resolves the tag; no fuzzy best-match guessing.
	meta, err := Resolve(context.Background(), newTestClient(t, srv), "ФАКТИЧЕСКОЕ ВРЕМЯ classifieds")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if meta.DataTagID != 3 {
		t.Errorf("DataTagID = %d, want 3", meta.DataTagID)
	}
}

func TestResolveEmptySelectorListsCandidates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","dataTags":[
			{"id":1,"name":"Список задач"},
			{"id":2,"name":"Фактическое время Support"},
			{"id":3,"name":"Фактическое время Classifieds"}
		]}`))
	}))
	defer srv.Close()

	_, err := Resolve(context.Background(), newTestClient(t, srv), "")
	if err == nil {
		t.Fatal("Resolve(\"\") error = nil, want failure")
	}
	for _, want := range []string{"not configured", "2 \"Фактическое время Support\"", "3 \"Фактическое время Classifieds\""} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want mention of %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "Список задач") {
		t.Errorf("err = %v, want non-worklog tags omitted", err)
	}
}

func TestResolveUnknownName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","dataTags":[
			{"id":2,"name":"Фактическое время Support"}
		]}`))
	}))
	defer srv.Close()

	_, err := Resolve(context.Background(), newTestClient(t, srv), "Такого тега нет")
	if err == nil {
		t.Fatal("unknown name: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "not found") || !strings.Contains(err.Error(), "Фактическое время Support") {
		t.Errorf("err = %v, want not-found with candidates", err)
	}
}

func TestMetaFromTag(t *testing.T) {
	tag := planfix.DataTag{ID: 2, Name: "Фактическое время Support", Fields: []planfix.DataField{
		{ID: 10, Name: "Дата", Type: typeDate},
		{ID: 11, Name: "Минут потрачено", Type: typeNumber},
		{ID: 12, Name: "Статус", Type: typeList, EnumValues: []string{"Работа", "Bugfix"}},
		{ID: 13, Name: "Детали работ", Type: typeMultiLineText},
		{ID: 14, Name: "Домен", Type: typeDirectoryEntry, DirectoryID: 21854},
		{ID: 15, Name: "Сотрудник", Type: typeEmployee},
	}}
	meta, err := MetaFromTag(tag)
	if err != nil {
		t.Fatalf("MetaFromTag() error = %v", err)
	}
	if meta.FieldDate != 10 || meta.FieldTime != 11 || !meta.TimeInMinutes {
		t.Errorf("date/time = %d/%d (minutes %v), want 10/11 true", meta.FieldDate, meta.FieldTime, meta.TimeInMinutes)
	}
	if meta.FieldWorkType != 12 || meta.WorkTypeDirectory != 0 {
		t.Errorf("work type = %d (dir %d), want list field 12", meta.FieldWorkType, meta.WorkTypeDirectory)
	}
	if len(meta.WorkTypeValues) != 2 || meta.WorkTypeValues[0] != "Работа" {
		t.Errorf("WorkTypeValues = %v, want the list enum values", meta.WorkTypeValues)
	}
	if meta.FieldNote != 13 {
		t.Errorf("FieldNote = %d, want 13", meta.FieldNote)
	}
	if meta.FieldEmployee != 15 {
		t.Errorf("FieldEmployee = %d, want 15", meta.FieldEmployee)
	}
}

func TestMetaFromTagDateIgnoresDecoys(t *testing.T) {
	tests := []struct {
		name   string
		fields []planfix.DataField
		want   int
	}{
		{
			"decoy Updated after real date",
			[]planfix.DataField{{ID: 10, Name: "Дата", Type: typeDate}, {ID: 11, Name: "Updated", Type: typeDate}},
			10,
		},
		{
			"decoy Validate date with date type",
			[]planfix.DataField{{ID: 10, Name: "Дата", Type: typeDate}, {ID: 11, Name: "Validate date", Type: typeDate}},
			10,
		},
		{
			"exact name beats substring with date type",
			[]planfix.DataField{{ID: 10, Name: "Фактическая дата", Type: typeDate}, {ID: 11, Name: "Дата", Type: typeDate}},
			11,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, err := MetaFromTag(planfix.DataTag{ID: 2, Fields: append(tt.fields,
				planfix.DataField{ID: 20, Name: "Минут потрачено", Type: typeNumber})})
			if err != nil {
				t.Fatalf("MetaFromTag() error = %v", err)
			}
			if meta.FieldDate != tt.want {
				t.Errorf("FieldDate = %d, want %d", meta.FieldDate, tt.want)
			}
		})
	}
}

func TestMetaFromTagWrongTypeFallsBackToName(t *testing.T) {
	meta, err := MetaFromTag(planfix.DataTag{ID: 2, Fields: []planfix.DataField{
		{ID: 10, Name: "Дата", Type: 99},
		{ID: 11, Name: "Время", Type: 99},
	}})
	if err != nil {
		t.Fatalf("MetaFromTag() error = %v", err)
	}
	if meta.FieldDate != 10 || meta.FieldTime != 11 {
		t.Errorf("meta = %+v, want name-only fallback to fields 10/11", meta)
	}
}

func TestMetaFromTagMinutesTimeField(t *testing.T) {
	// A worklog tag without a period-of-time field: time is recorded as a
	// spent-minutes number (the 4px account's schema).
	meta, err := MetaFromTag(planfix.DataTag{ID: 2, Fields: []planfix.DataField{
		{ID: 10, Name: "Дата", Type: typeDate},
		{ID: 11, Name: "Минут потрачено", Type: typeNumber},
		{ID: 12, Name: "Статус", Type: typeList},
		{ID: 13, Name: "Детали работ", Type: typeMultiLineText},
	}})
	if err != nil {
		t.Fatalf("MetaFromTag() error = %v", err)
	}
	if meta.FieldDate != 10 || meta.FieldTime != 11 {
		t.Errorf("meta = %+v, want FieldDate 10 and FieldTime 11", meta)
	}
	if !meta.TimeInMinutes {
		t.Errorf("TimeInMinutes = false, want true for %q", "Минут потрачено")
	}
}

func TestMetaFromTagPeriodTimeFieldWinsOverMinutes(t *testing.T) {
	meta, err := MetaFromTag(planfix.DataTag{ID: 2, Fields: []planfix.DataField{
		{ID: 10, Name: "Дата", Type: typeDate},
		{ID: 11, Name: "Минут потрачено", Type: typeNumber},
		{ID: 12, Name: "Время", Type: typePeriodOfTime},
	}})
	if err != nil {
		t.Fatalf("MetaFromTag() error = %v", err)
	}
	if meta.FieldTime != 12 || meta.TimeInMinutes {
		t.Errorf("meta = %+v, want period field 12 without TimeInMinutes", meta)
	}
}

func TestMetaFromTagMissingTimeField(t *testing.T) {
	_, err := MetaFromTag(planfix.DataTag{ID: 2, Fields: []planfix.DataField{
		{ID: 10, Name: "Дата", Type: typeDate},
	}})
	if err == nil {
		t.Fatal("MetaFromTag() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "missing date/time fields") {
		t.Errorf("err = %v, want missing date/time fields", err)
	}
	if !strings.Contains(err.Error(), "Дата (type 3)") {
		t.Errorf("err = %v, want field names/types listed", err)
	}
}
