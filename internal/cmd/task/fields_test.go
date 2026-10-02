package task

import (
	"reflect"
	"testing"

	"planfix-cli/internal/planfix"
)

func TestFieldColumns(t *testing.T) {
	tests := []struct {
		name        string
		fields      string
		wantNames   []string
		wantHeaders []string
	}{
		{"empty", "", nil, nil},
		{"list defaults", "id,name,status,priority", []string{"id", "name", "status", "priority"}, []string{"ID", "NAME", "STATUS", "PRIORITY"}},
		{"dates and spaces", " startDate , endDate ", []string{"startDate", "endDate"}, []string{"STARTDATE", "ENDDATE"}},
		{"nested", "status.name", []string{"status.name"}, []string{"STATUS.NAME"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names, headers := fieldColumns(tt.fields)
			if !reflect.DeepEqual(names, tt.wantNames) {
				t.Errorf("fieldColumns(%q) names = %v, want %v", tt.fields, names, tt.wantNames)
			}
			if !reflect.DeepEqual(headers, tt.wantHeaders) {
				t.Errorf("fieldColumns(%q) headers = %v, want %v", tt.fields, headers, tt.wantHeaders)
			}
		})
	}
}

func TestFieldValue(t *testing.T) {
	task := planfix.Task{
		ID:          7,
		Name:        "Write tests",
		Description: "cover the fields",
		Status:      planfix.Status{Name: "In progress"},
		Priority:    "high",
		StartDate:   "2026-02-01",
		EndDate:     "2026-02-10",
	}
	task.Assignees.Users = []planfix.PersonRef{{ID: 1, Name: "Ann"}, {ID: 2, Name: "Bob"}}
	tests := []struct {
		field string
		want  string
	}{
		{"id", "7"},
		{"name", "Write tests"},
		{"status", "In progress"},
		{"status.name", "In progress"},
		{"priority", "high"},
		{"description", "cover the fields"},
		{"startDate", "2026-02-01"},
		{"endDate", "2026-02-10"},
		{"assignees", "Ann, Bob"},
		{"project", ""},
	}
	for _, tt := range tests {
		if got := fieldValue(task, tt.field); got != tt.want {
			t.Errorf("fieldValue(%q) = %q, want %q", tt.field, got, tt.want)
		}
	}
}

func TestFieldValueAssigneesEmpty(t *testing.T) {
	if got := fieldValue(planfix.Task{}, "assignees"); got != "" {
		t.Errorf("fieldValue(assignees) on empty task = %q, want empty", got)
	}
}

func TestDefaultViewKV(t *testing.T) {
	full := planfix.Task{
		ID:          42,
		Name:        "Ship it",
		Description: "long text",
		Status:      planfix.Status{Name: "Done"},
		Priority:    "urgent",
		StartDate:   "2026-02-01",
		EndDate:     "2026-02-10",
	}
	full.Assignees.Users = []planfix.PersonRef{{ID: 5, Name: "Ann"}}
	kv := defaultViewKV(full)
	want := [][2]string{
		{"ID", "42"},
		{"NAME", "Ship it"},
		{"STATUS", "Done"},
		{"PRIORITY", "urgent"},
		{"START", "2026-02-01"},
		{"END", "2026-02-10"},
		{"DESCRIPTION", "long text"},
		{"ASSIGNEES", "Ann"},
	}
	if !reflect.DeepEqual(kv, want) {
		t.Errorf("defaultViewKV(full) = %v, want %v", kv, want)
	}

	bare := planfix.Task{ID: 1, Name: "Minimal", Status: planfix.Status{Name: "New"}}
	kv = defaultViewKV(bare)
	want = [][2]string{
		{"ID", "1"},
		{"NAME", "Minimal"},
		{"STATUS", "New"},
		{"PRIORITY", ""},
	}
	if !reflect.DeepEqual(kv, want) {
		t.Errorf("defaultViewKV(bare) = %v, want %v", kv, want)
	}
}
