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

func TestDiscover(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		switch r.URL.Path {
		case "/rest/datatag":
			_, _ = w.Write([]byte(`{"result":"success","dataTags":[
				{"id":1,"name":"Планируемое время"},
				{"id":2,"name":"Фактическое время"}
			]}`))
		case "/rest/datatag/2":
			_, _ = w.Write([]byte(`{"result":"success","dataTag":{"id":2,"name":"Фактическое время","fields":[
				{"id":10,"name":"Дата","type":1},
				{"id":11,"name":"Время","type":5},
				{"id":12,"name":"Вид работ","type":7,"directoryId":3},
				{"id":13,"name":"Сотрудник","type":10}
			]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c, err := planfix.New("x", "tok")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = srv.URL + "/rest"

	meta, err := Discover(context.Background(), c)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if meta.DataTagID != 2 || meta.FieldDate != 10 || meta.FieldTime != 11 ||
		meta.FieldWorkType != 12 || meta.WorkTypeDirectory != 3 || meta.FieldEmployee != 13 {
		t.Errorf("meta = %+v", meta)
	}
}

func TestDiscoverDateIgnoresDecoys(t *testing.T) {
	tests := []struct {
		name   string
		fields string
		want   int
	}{
		{
			"decoy Updated after real date",
			`{"id":10,"name":"Дата","type":1},{"id":11,"name":"Updated","type":5}`,
			10,
		},
		{
			"decoy Validate date with date type",
			`{"id":10,"name":"Дата","type":1},{"id":11,"name":"Validate date","type":1}`,
			10,
		},
		{
			"exact name beats substring with date type",
			`{"id":10,"name":"Фактическая дата","type":1},{"id":11,"name":"Дата","type":3}`,
			11,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/rest/datatag":
					_, _ = w.Write([]byte(`{"result":"success","dataTags":[{"id":2,"name":"Фактическое время"}]}`))
				case "/rest/datatag/2":
					_, _ = w.Write([]byte(`{"result":"success","dataTag":{"id":2,"name":"Фактическое время","fields":[` +
						tt.fields + `,{"id":20,"name":"Время","type":5}]}}`))
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			c, err := planfix.New("x", "tok")
			if err != nil {
				t.Fatal(err)
			}
			c.BaseURL = srv.URL + "/rest"

			meta, err := Discover(context.Background(), c)
			if err != nil {
				t.Fatalf("Discover() error = %v", err)
			}
			if meta.FieldDate != tt.want {
				t.Errorf("FieldDate = %d, want %d", meta.FieldDate, tt.want)
			}
		})
	}
}

func TestDiscoverWrongTypeTimeFallsBackToName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/datatag":
			_, _ = w.Write([]byte(`{"result":"success","dataTags":[{"id":2,"name":"Фактическое время"}]}`))
		case "/rest/datatag/2":
			_, _ = w.Write([]byte(`{"result":"success","dataTag":{"id":2,"name":"Фактическое время","fields":[
				{"id":10,"name":"Дата","type":1},
				{"id":11,"name":"Время","type":99}
			]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c, err := planfix.New("x", "tok")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = srv.URL + "/rest"

	meta, err := Discover(context.Background(), c)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if meta.FieldTime != 11 {
		t.Errorf("FieldTime = %d, want 11 (name-only fallback)", meta.FieldTime)
	}
}

func TestDiscoverNoWorklogTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","dataTags":[
			{"id":1,"name":"Список задач"}
		]}`))
	}))
	defer srv.Close()

	c, err := planfix.New("x", "tok")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = srv.URL + "/rest"

	meta, err := Discover(context.Background(), c)
	if err == nil {
		t.Fatalf("Discover() = %+v, want error", meta)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want mention of not found", err)
	}
}

func TestDiscoverMissingTimeField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/datatag":
			_, _ = w.Write([]byte(`{"result":"success","dataTags":[
				{"id":2,"name":"Фактическое время"}
			]}`))
		case "/rest/datatag/2":
			_, _ = w.Write([]byte(`{"result":"success","dataTag":{"id":2,"name":"Фактическое время","fields":[
				{"id":10,"name":"Дата","type":1}
			]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c, err := planfix.New("x", "tok")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = srv.URL + "/rest"

	meta, err := Discover(context.Background(), c)
	if err == nil {
		t.Fatalf("Discover() = %+v, want error", meta)
	}
	if !strings.Contains(err.Error(), "missing date/time fields") {
		t.Errorf("err = %v, want missing date/time fields", err)
	}
	if !strings.Contains(err.Error(), "Дата (type 1)") {
		t.Errorf("err = %v, want field names/types listed", err)
	}
}
