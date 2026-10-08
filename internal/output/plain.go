package output

import (
	"html"
	"regexp"
	"strings"
)

// Regexp set for StripHTML: comments and style/script blocks are dropped
// first, then list items gain a bullet, block and cell tags become
// separators, and any leftover tag is removed. RE2 has no backreferences,
// so style and script get one pattern each.
var (
	reHTMLComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	reHTMLStyle   = regexp.MustCompile(`(?is)<style\b.*?</style>`)
	reHTMLScript  = regexp.MustCompile(`(?is)<script\b.*?</script>`)
	// Source line breaks between adjacent tags (newlines and indentation)
	// collapse to one space: table cells and list items stay on one line
	// and words between inline tags stay apart.
	reHTMLGap       = regexp.MustCompile(`(?s)>[ \t]*(?:\r?\n[ \t]*)+<`)
	reHTMLLIOpen    = regexp.MustCompile(`(?is)<li\b[^>]*>`)
	reHTMLBlock     = regexp.MustCompile(`(?is)</?(?:div|p|h[1-6]|blockquote|pre|table|tbody|thead|tr|ul|ol|section|article|hr|br)\b[^>]*>`)
	reHTMLCellOpen  = regexp.MustCompile(`(?is)<(?:td|th)\b[^>]*>`)
	reHTMLCellClose = regexp.MustCompile(`(?is)</(?:td|th)\b[^>]*>`)
	// The tag body must start with a letter or / ! ? so that comparison
	// text like "x < y && z > w" survives stripping.
	reHTMLTag = regexp.MustCompile(`(?s)<[/!?]?[A-Za-z][^>]*>`)
	// spaceRun matches horizontal whitespace runs, collapsed the way a
	// browser would render normal white-space.
	spaceRun = regexp.MustCompile(`[ \t]+`)
)

// StripHTML renders an HTML fragment as plain text: markup and style
// blocks are removed, block tags become line breaks, <li> gains a bullet,
// cells are joined with spaces, entities are decoded, and blank lines are
// collapsed to one. Text without markup passes through with only entity
// decoding and line trimming.
func StripHTML(s string) string {
	s = reHTMLComment.ReplaceAllString(s, "")
	s = reHTMLStyle.ReplaceAllString(s, "")
	s = reHTMLScript.ReplaceAllString(s, "")
	s = reHTMLGap.ReplaceAllString(s, "> <")
	s = reHTMLLIOpen.ReplaceAllString(s, "\n- ")
	s = reHTMLBlock.ReplaceAllString(s, "\n")
	// Only the closing cell tag leaves a separator so a row does not
	// double it at every cell boundary.
	s = reHTMLCellOpen.ReplaceAllString(s, "")
	s = reHTMLCellClose.ReplaceAllString(s, " ")
	s = reHTMLTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return collapseBlankLines(s)
}

// collapseBlankLines trims each line, keeps at most one blank line
// between text lines, and drops leading and trailing blank lines.
func collapseBlankLines(s string) string {
	var out []string
	blank := true
	for _, ln := range strings.Split(s, "\n") {
		ln = strings.TrimSpace(ln)
		// Runs of spaces and tabs collapse: the gap rule above can leave a
		// doubled space at a cell boundary, and a tab would break tables.
		ln = spaceRun.ReplaceAllString(ln, " ")
		if ln == "" {
			if blank {
				continue
			}
			blank = true
			out = append(out, "")
			continue
		}
		blank = false
		out = append(out, ln)
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

// StripRows removes HTML from every cell when plain is set; otherwise
// the rows are returned unchanged.
func StripRows(rows [][]string, plain bool) [][]string {
	if !plain {
		return rows
	}
	for _, row := range rows {
		for i, cell := range row {
			row[i] = StripHTML(cell)
		}
	}
	return rows
}

// StripKV removes HTML from every detail value when plain is set;
// otherwise the pairs are returned unchanged.
func StripKV(kv [][2]string, plain bool) [][2]string {
	if !plain {
		return kv
	}
	for i, pair := range kv {
		kv[i][1] = StripHTML(pair[1])
	}
	return kv
}
