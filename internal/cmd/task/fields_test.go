package task

import (
	"reflect"
	"testing"

	"planfix-cli/internal/planfix"
)

func TestColumnsFromFields(t *testing.T) {
	tests := []struct {
		name   string
		fields string
		want   []string
	}{
		{"empty", "", nil},
		{"list defaults", "id,name,status,priority", []string{"ID", "NAME", "STATUS", "PRIORITY"}},
		{"dates and spaces", " startDate , endDate ", []string{"STARTDATE", "ENDDATE"}},
		{"nested", "status.name", []string{"STATUS.NAME"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := columnsFromFields(tt.fields)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("columnsFromFields(%q) = %v, want %v", tt.fields, got, tt.want)
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
		{"project", ""},
	}
	for _, tt := range tests {
		if got := fieldValue(task, tt.field); got != tt.want {
			t.Errorf("fieldValue(%q) = %q, want %q", tt.field, got, tt.want)
		}
	}
}
