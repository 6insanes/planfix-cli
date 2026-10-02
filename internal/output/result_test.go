package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteResultDefaultUsesVerbNounID(t *testing.T) {
	for _, tt := range []struct {
		verb, noun string
		id         int
		want       string
	}{
		{"Created", "task", 99, "Created task 99\n"},
		{"Updated", "task", 5, "Updated task 5\n"},
		{"Added", "comment", 55, "Added comment 55\n"},
	} {
		var buf bytes.Buffer
		if err := WriteResult(&buf, WriteOpts{}, tt.verb, tt.noun, tt.id, []byte(`{"id":1}`)); err != nil {
			t.Fatalf("WriteResult(%s %s) error = %v", tt.verb, tt.noun, err)
		}
		if buf.String() != tt.want {
			t.Errorf("WriteResult(%s %s, %d) = %q, want %q", tt.verb, tt.noun, tt.id, buf.String(), tt.want)
		}
	}
}

func TestWriteResultQuietPrintsIDOnly(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteResult(&buf, WriteOpts{Quiet: true}, "Added", "comment", 55, []byte(`{"id":55}`)); err != nil {
		t.Fatalf("WriteResult() error = %v", err)
	}
	if buf.String() != "55\n" {
		t.Errorf("quiet output = %q, want \"55\\n\"", buf.String())
	}
}

func TestWriteResultJSONPrettyPrintsRaw(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteResult(&buf, WriteOpts{JSON: true}, "Added", "comment", 55, []byte(`{"id":55}`)); err != nil {
		t.Fatalf("WriteResult() error = %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "{\n  \"id\": 55\n}\n") {
		t.Errorf("json output = %q, want pretty-printed", got)
	}
}
