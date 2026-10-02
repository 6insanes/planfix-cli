// Package worklogtag finds the account's worklog data tag and its field ids.
package worklogtag

import (
	"context"
	"fmt"
	"strings"

	"planfix-cli/internal/config"
	"planfix-cli/internal/planfix"
)

// Field-type codes returned by Planfix. Matching is primarily by field name;
// the type is a secondary hint to disambiguate time/work-type/employee fields.
const (
	typeDate           = 1
	typePeriodOfTime   = 5
	typeDirectoryEntry = 7
	typeUsersArray     = 10
)

// MatchName scores a data-tag name as a worklog candidate.
// 2 = priority keyword, 1 = secondary keyword, -1 = skip (planned), 0 = no match.
func MatchName(name string) int {
	n := strings.ToLower(name)
	if strings.Contains(n, "планируем") || strings.Contains(n, "planned") {
		return -1
	}
	for _, kw := range []string{"фактическое", "actual", "worklog", "work log"} {
		if strings.Contains(n, kw) {
			return 2
		}
	}
	for _, kw := range []string{"время работы", "work time", "time tracking"} {
		if strings.Contains(n, kw) {
			return 1
		}
	}
	return 0
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
	for _, f := range tag.Fields {
		n := strings.ToLower(f.Name)
		switch {
		case strings.Contains(n, "дата") || strings.Contains(n, "date"):
			meta.FieldDate = f.ID
		case (strings.Contains(n, "время") || strings.Contains(n, "период") ||
			strings.Contains(n, "time") || strings.Contains(n, "period")) && f.Type == typePeriodOfTime:
			meta.FieldTime = f.ID
		case (strings.Contains(n, "вид") || strings.Contains(n, "work")) && f.Type == typeDirectoryEntry:
			meta.FieldWorkType = f.ID
			meta.WorkTypeDirectory = f.DirectoryID
		case (strings.Contains(n, "сотрудник") || strings.Contains(n, "employee")) && f.Type == typeUsersArray:
			meta.FieldEmployee = f.ID
		}
	}
	if meta.FieldDate == 0 || meta.FieldTime == 0 {
		return nil, fmt.Errorf("worklog data tag %d is missing date/time fields", meta.DataTagID)
	}
	return meta, nil
}
