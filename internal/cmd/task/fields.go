package task

import (
	"strconv"
	"strings"

	"planfix-cli/internal/planfix"
)

// Default field lists requested from the API when --fields is empty.
const (
	defaultListFields = "id,name,status,priority"
	defaultViewFields = "id,name,description,status,priority,startDate,endDate,assignees"
)

// fieldColumns splits a comma-separated --fields value into trimmed,
// non-empty API field names and their display headers ("id" -> "ID",
// "status.name" -> "STATUS.NAME"), preserving order. Empty input yields
// nil slices.
func fieldColumns(fields string) (names, headers []string) {
	parts := strings.Split(fields, ",")
	for _, p := range parts {
		f := strings.TrimSpace(p)
		if f == "" {
			continue
		}
		names = append(names, f)
		headers = append(headers, strings.ToUpper(f))
	}
	return names, headers
}

// defaultViewRows is the curated default view: display label, API field
// name, and whether the row is skipped when the value is empty.
var defaultViewRows = []struct {
	label    string
	field    string
	optional bool
}{
	{"ID", "id", false},
	{"NAME", "name", false},
	{"STATUS", "status", false},
	{"PRIORITY", "priority", false},
	{"START", "startDate", true},
	{"END", "endDate", true},
	{"DESCRIPTION", "description", true},
	{"ASSIGNEES", "assignees", true},
}

// defaultViewKV renders the curated default view rows for one task via
// fieldValue, keeping key order and skipping empty optional values.
func defaultViewKV(t planfix.Task) [][2]string {
	kv := make([][2]string, 0, len(defaultViewRows))
	for _, r := range defaultViewRows {
		v := fieldValue(t, r.field)
		if r.optional && v == "" {
			continue
		}
		kv = append(kv, [2]string{r.label, v})
	}
	return kv
}

// fieldValue renders one task field as its display string. Unknown fields
// render empty so output stays aligned with the requested field list.
func fieldValue(t planfix.Task, field string) string {
	switch field {
	case "id":
		return strconv.Itoa(t.ID)
	case "name":
		return t.Name
	case "status", "status.name":
		return t.Status.Name
	case "priority":
		return t.Priority
	case "description":
		return t.Description
	case "startDate":
		return t.StartDate
	case "endDate":
		return t.EndDate
	case "assignees":
		names := make([]string, 0, len(t.Assignees.Users))
		for _, u := range t.Assignees.Users {
			names = append(names, u.Name)
		}
		return strings.Join(names, ", ")
	}
	return ""
}
