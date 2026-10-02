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

// splitFields splits a comma-separated --fields value into trimmed,
// non-empty field names.
func splitFields(fields string) []string {
	parts := strings.Split(fields, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if f := strings.TrimSpace(p); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// columnsFromFields maps API field names to display column headers,
// e.g. "id" -> "ID", "status" -> "STATUS" (or "status.name" for nested).
func columnsFromFields(fields string) []string {
	names := splitFields(fields)
	cols := make([]string, 0, len(names))
	for _, name := range names {
		cols = append(cols, strings.ToUpper(name))
	}
	return cols
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
