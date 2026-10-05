// Package worklogtag maps the pinned worklog data tag onto its field ids.
package worklogtag

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/6insanes/planfix-cli/internal/config"
	"github.com/6insanes/planfix-cli/internal/planfix"
)

// Field-type codes from the Planfix REST spec customFieldTypes list.
const (
	typeShortText      = 0
	typeNumber         = 1
	typeMultiLineText  = 2
	typeDate           = 3
	typePeriodOfTime   = 6
	typeList           = 8
	typeDirectoryEntry = 9
	typeEmployee       = 11
	typeListOfUsers    = 14
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

// pickField selects a field by name and type. When several types are given,
// the first pass requires name+type (highest confidence); the second pass
// accepts a name-only match so an unexpected type code cannot make mapping
// fail. taken reports field ids already assigned to earlier slots.
func pickField(fields []planfix.DataField, nameOK func(string) bool, taken func(int) bool, types ...int) planfix.DataField {
	for _, f := range fields {
		if taken != nil && taken(f.ID) {
			continue
		}
		if !nameOK(strings.ToLower(f.Name)) {
			continue
		}
		for _, typ := range types {
			if f.Type == typ {
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

// describeTags lists worklog-like tags for error messages.
func describeTags(tags []planfix.DataTag) string {
	parts := make([]string, 0, len(tags))
	for _, dt := range tags {
		if MatchName(dt.Name) > 0 {
			parts = append(parts, fmt.Sprintf("%d %q", dt.ID, dt.Name))
		}
	}
	if len(parts) == 0 {
		return "(none)"
	}
	return strings.Join(parts, ", ")
}

// Resolve maps the pinned worklog data tag (numeric id or exact tag name)
// onto its field ids. An empty or unknown selector fails with the account's
// worklog-like tags listed: worklog tags cannot be auto-picked by name when
// several departments keep one tag each.
func Resolve(ctx context.Context, c *planfix.Client, selector string) (*config.WorklogMeta, error) {
	if selector == "" {
		list, err := c.ListDataTags(ctx)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf(
			"worklog data tag is not configured; set worklog.datatag (id or exact name) in the config or pass --data-tag; worklog-like tags: %s",
			describeTags(list.DataTags))
	}
	id, err := strconv.Atoi(selector)
	if err != nil {
		list, lerr := c.ListDataTags(ctx)
		if lerr != nil {
			return nil, lerr
		}
		for _, dt := range list.DataTags {
			if strings.EqualFold(dt.Name, selector) {
				id = dt.ID
				break
			}
		}
		if id == 0 {
			return nil, fmt.Errorf("data tag %q not found; worklog-like tags: %s",
				selector, describeTags(list.DataTags))
		}
	}
	tag, err := c.GetDataTag(ctx, id)
	if err != nil {
		return nil, err
	}
	return MetaFromTag(*tag)
}

// MetaFromTag maps one data tag's custom fields onto the worklog slots.
// Matching is primarily by field name; the type code is a tie-breaker within
// a slot. Date and time slots are required.
func MetaFromTag(tag planfix.DataTag) (*config.WorklogMeta, error) {
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
			id == meta.FieldWorkType || id == meta.FieldNote || id == meta.FieldEmployee)
	}
	isTime := func(n string) bool {
		return strings.Contains(n, "время") || strings.Contains(n, "период") ||
			strings.Contains(n, "time") || strings.Contains(n, "period")
	}
	// isMinutesTime matches a spent-minutes number field («Минут потрачено»)
	// as used by worklog tags without a period-of-time field.
	isMinutesTime := func(n string) bool {
		return strings.Contains(n, "минут") || strings.Contains(n, "minute")
	}
	isWorkType := func(n string) bool {
		return strings.Contains(n, "статус") || strings.Contains(n, "status") ||
			strings.Contains(n, "вид") || strings.Contains(n, "тип работ") ||
			strings.Contains(n, "work type") || strings.Contains(n, "type of work")
	}
	isNote := func(n string) bool {
		return strings.Contains(n, "детали") || strings.Contains(n, "подробност") ||
			strings.Contains(n, "описан") || strings.Contains(n, "примечан") ||
			strings.Contains(n, "note") || strings.Contains(n, "detail") ||
			strings.Contains(n, "comment")
	}
	isEmployee := func(n string) bool {
		return strings.Contains(n, "сотрудник") || strings.Contains(n, "employee")
	}

	if tf := pickField(tag.Fields, isTime, taken, typePeriodOfTime); tf.ID != 0 {
		meta.FieldTime = tf.ID
	} else if tf := pickField(tag.Fields, isMinutesTime, taken, typeNumber); tf.ID != 0 {
		meta.FieldTime, meta.TimeInMinutes = tf.ID, true
	}
	wt := pickField(tag.Fields, isWorkType, taken, typeList, typeDirectoryEntry)
	meta.FieldWorkType, meta.WorkTypeDirectory = wt.ID, wt.DirectoryID
	if wt.Type == typeList {
		meta.WorkTypeValues = wt.EnumValues
	}
	meta.FieldNote = pickField(tag.Fields, isNote, taken, typeMultiLineText, typeShortText).ID
	meta.FieldEmployee = pickField(tag.Fields, isEmployee, taken, typeEmployee, typeListOfUsers).ID

	if meta.FieldDate == 0 || meta.FieldTime == 0 {
		return nil, fmt.Errorf("worklog data tag %d is missing date/time fields (fields: %s)",
			meta.DataTagID, describeFields(tag.Fields))
	}
	return meta, nil
}
