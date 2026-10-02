// Package worklogtag finds the account's worklog data tag and its field ids.
package worklogtag

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"planfix-cli/internal/config"
	"planfix-cli/internal/planfix"
)

// Field-type codes returned by Planfix. Matching is primarily by field name;
// the type is a soft hint (pass 1: name+type, pass 2: name only) because the
// codes are unverified and may drift between accounts.
const (
	typeDate           = 1
	typePeriodOfTime   = 5
	typeDirectoryEntry = 7
	typeUsersArray     = 10
)

// dateDecoys are name fragments marking a change/creation timestamp rather
// than the worklog date. They must be rejected before the «дата»/«date»
// substring test because they contain it («update» contains «date»).
var dateDecoys = []string{"обновлен", "update", "создан", "create", "validate"}

// MatchName scores a data-tag name as a worklog candidate.
// 2 = priority keyword, 1 = secondary keyword, -1 = skip (planned), 0 = no match.
// Priority keywords are checked before the planned-skip so a tag like
// «Unplanned worklog» still scores as a worklog candidate.
func MatchName(name string) int {
	n := strings.ToLower(name)
	for _, kw := range []string{"фактическ", "actual", "worklog", "work log"} {
		if strings.Contains(n, kw) {
			return 2
		}
	}
	if strings.Contains(n, "планируем") ||
		(strings.Contains(n, "planned") && !strings.Contains(n, "unplanned")) {
		return -1
	}
	for _, kw := range []string{"время работы", "work time", "time tracking", "учет", "учёт"} {
		if strings.Contains(n, kw) {
			return 1
		}
	}
	return 0
}

// isDateDecoy reports whether a lowercased field name looks like a timestamp
// of change/creation rather than the worklog date itself.
func isDateDecoy(n string) bool {
	for _, d := range dateDecoys {
		if strings.Contains(n, d) {
			return true
		}
	}
	return false
}

// dateScore scores a field as the date slot. Higher is better, -1 = reject.
// Exact name («дата»/«date») outranks any substring match; a typeDate field
// is a soft tie-breaker within a tier; ties keep the earlier field.
func dateScore(name string, typ int) int {
	n := strings.ToLower(name)
	if isDateDecoy(n) {
		return -1
	}
	exact := n == "дата" || n == "date"
	if !exact && !strings.Contains(n, "дата") && !strings.Contains(n, "date") {
		return -1
	}
	score := 0
	if exact {
		score += 2
	}
	if typ == typeDate {
		score++
	}
	return score
}

// pickField selects a field by name. When typ > 0 the first pass requires
// name+type (highest confidence); the second pass accepts a name-only match
// so an unverified type code cannot make discovery fail. taken reports field
// ids already assigned to earlier slots and is honoured only in the name-only
// pass.
func pickField(fields []planfix.DataField, nameOK func(string) bool, typ int, taken func(int) bool) planfix.DataField {
	if typ != 0 {
		for _, f := range fields {
			if f.Type == typ && nameOK(strings.ToLower(f.Name)) {
				return f
			}
		}
	}
	for _, f := range fields {
		if nameOK(strings.ToLower(f.Name)) && (taken == nil || !taken(f.ID)) {
			return f
		}
	}
	return planfix.DataField{}
}

// describeFields lists field names and type codes for error messages.
func describeFields(fields []planfix.DataField) string {
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		parts = append(parts, f.Name+" (type "+strconv.Itoa(f.Type)+")")
	}
	return strings.Join(parts, ", ")
}

// Discover finds the worklog data tag and maps its field ids.
func Discover(ctx context.Context, c *planfix.Client) (*config.WorklogMeta, error) {
	list, err := c.ListDataTags(ctx)
	if err != nil {
		return nil, err
	}

	bestID, bestScore := 0, 0
	for _, dt := range list.DataTags {
		score := MatchName(dt.Name)
		if score > bestScore {
			bestScore, bestID = score, dt.ID
		}
	}
	if bestID == 0 {
		return nil, fmt.Errorf("worklog data tag not found on this account")
	}

	tag, err := c.GetDataTag(ctx, bestID)
	if err != nil {
		return nil, err
	}

	meta := &config.WorklogMeta{DataTagID: tag.ID}

	dateBest, dateBestScore := 0, -1
	for _, f := range tag.Fields {
		if s := dateScore(f.Name, f.Type); s > dateBestScore {
			dateBest, dateBestScore = f.ID, s
		}
	}
	meta.FieldDate = dateBest

	taken := func(id int) bool {
		return id != 0 && (id == meta.FieldDate || id == meta.FieldTime ||
			id == meta.FieldWorkType || id == meta.FieldEmployee)
	}
	isTime := func(n string) bool {
		return strings.Contains(n, "время") || strings.Contains(n, "период") ||
			strings.Contains(n, "time") || strings.Contains(n, "period")
	}
	isWorkType := func(n string) bool {
		return strings.Contains(n, "вид") || strings.Contains(n, "work")
	}
	isEmployee := func(n string) bool {
		return strings.Contains(n, "сотрудник") || strings.Contains(n, "employee")
	}

	meta.FieldTime = pickField(tag.Fields, isTime, typePeriodOfTime, taken).ID
	wt := pickField(tag.Fields, isWorkType, typeDirectoryEntry, taken)
	meta.FieldWorkType, meta.WorkTypeDirectory = wt.ID, wt.DirectoryID
	meta.FieldEmployee = pickField(tag.Fields, isEmployee, typeUsersArray, taken).ID

	if meta.FieldDate == 0 || meta.FieldTime == 0 {
		return nil, fmt.Errorf("worklog data tag %d is missing date/time fields (fields: %s)",
			meta.DataTagID, describeFields(tag.Fields))
	}
	return meta, nil
}
