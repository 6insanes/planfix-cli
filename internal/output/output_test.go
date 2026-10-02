package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestTable(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, []string{"ID", "NAME"}, [][]string{
		{"1", "alpha"},
		{"22", "beta"},
	})
	got := buf.String()
	if !strings.Contains(got, "ID") || !strings.Contains(got, "NAME") {
		t.Errorf("header missing:\n%s", got)
	}
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "beta") {
		t.Errorf("rows missing:\n%s", got)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("line count = %d, want 3:\n%s", len(lines), got)
	}
	// Columns are aligned: both rows must start their second cell at the same offset.
	if !aligned(got) {
		t.Errorf("columns not aligned:\n%s", got)
	}
}

func TestTableNoHeader(t *testing.T) {
	var buf bytes.Buffer
	Table(&buf, nil, [][]string{{"1"}})
	if strings.Contains(buf.String(), "\t") {
		t.Errorf("output contains tab: %q", buf.String())
	}
	if got := strings.TrimSpace(buf.String()); got != "1" {
		t.Errorf("output = %q, want %q", got, "1")
	}
}

func aligned(s string) bool {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	want := -1
	for _, ln := range lines {
		i := strings.Index(ln, "  ")
		if i < 0 {
			return false
		}
		for i < len(ln) && ln[i] == ' ' {
			i++
		}
		if want == -1 {
			want = i
		} else if i != want {
			return false
		}
	}
	return want > 0
}

func TestDetail(t *testing.T) {
	var buf bytes.Buffer
	Detail(&buf, [][2]string{
		{"ID", "1"},
		{"NAME", "Fix bug"},
	})
	got := buf.String()
	for _, want := range []string{"ID:", "1", "NAME:", "Fix bug"} {
		if !strings.Contains(got, want) {
			t.Errorf("detail missing %q:\n%s", want, got)
		}
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("line count = %d, want 2:\n%s", len(lines), got)
	}
	// Values must line up across rows.
	idx := strings.Index(lines[0], "1")
	idx2 := strings.Index(lines[1], "Fix")
	if idx < 0 || idx2 < 0 || idx != idx2 {
		t.Errorf("values not aligned: %q / %q", lines[0], lines[1])
	}
}

func TestJSONValid(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, []byte(`{"a":1}`)); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	got := buf.String()
	if got != "{\n  \"a\": 1\n}\n" {
		t.Errorf("JSON() = %q, want pretty-printed with trailing newline", got)
	}
}

func TestJSONInvalidFallsBackToRaw(t *testing.T) {
	var buf bytes.Buffer
	raw := []byte("not json")
	if err := JSON(&buf, raw); err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	if buf.String() != "not json" {
		t.Errorf("JSON() = %q, want raw passthrough", buf.String())
	}
}
