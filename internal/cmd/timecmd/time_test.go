package timecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/6insanes/planfix-cli/internal/config"
	"github.com/6insanes/planfix-cli/internal/planfix"
)

func jsonDecode(r *http.Request, dst any) error {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

// entryFields digs the created entry's customFieldData out of a
// /task/{id}/datatags/ request body.
func entryFields(t *testing.T, body map[string]any) []any {
	t.Helper()
	items, _ := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %#v, want one entry", body["items"])
	}
	cfd, _ := items[0].(map[string]any)["customFieldData"].([]any)
	return cfd
}

// stubClient returns a ClientFunc pointing at srv.
func stubClient(srv *httptest.Server) ClientFunc {
	return func() (*planfix.Client, error) {
		c, err := planfix.New("example.com", "tok")
		if err != nil {
			return nil, err
		}
		c.BaseURL = srv.URL
		return c, nil
	}
}

// stubMeta returns a MetaFunc returning meta, recording the meta options.
func stubMeta(meta *config.WorklogMeta, optsSeen *MetaOpts) MetaFunc {
	return func(ctx context.Context, c *planfix.Client, opts MetaOpts) (*config.WorklogMeta, error) {
		if optsSeen != nil {
			*optsSeen = opts
		}
		if meta == nil {
			return nil, io.ErrUnexpectedEOF
		}
		return meta, nil
	}
}

func testMeta() *config.WorklogMeta {
	return &config.WorklogMeta{
		DataTagID:         123,
		FieldDate:         456,
		FieldTime:         789,
		FieldWorkType:     101,
		WorkTypeDirectory: 3,
	}
}

// exec runs cmd with args and returns captured stdout.
func exec(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	buf := new(bytes.Buffer)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func testCmd(srv *httptest.Server, opts Options, meta *config.WorklogMeta) *cobra.Command {
	return NewCmd(stubClient(srv), func() Options { return opts }, stubMeta(meta, nil))
}

func TestNewCmdListsSubcommands(t *testing.T) {
	cmd := NewCmd(nil, func() Options { return Options{} }, nil)
	out, err := exec(t, cmd, "--help")
	if err != nil {
		t.Fatalf("help error = %v", err)
	}
	for _, want := range []string{"add", "list"} {
		if !strings.Contains(out, want) {
			t.Errorf("time help missing %q:\n%s", want, out)
		}
	}
}

func TestParseHours(t *testing.T) {
	tests := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{"1.5", 1.5, false},
		{"2", 2, false},
		{"0.25", 0.25, false},
		{"1.999", 2, false}, // rounded to whole minutes
		{"-1", 0, true},
		{"0", 0, true},
		{"abc", 0, true},
		{"0.0001", 0, true}, // under a minute
		{"24", 0, true},
		{"48", 0, true},
	}
	for _, tt := range tests {
		got, err := ParseHours(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseHours(%q) = %v, nil; want error", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseHours(%q) error = %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseHours(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}

	if _, err := ParseHours("24"); err == nil || !strings.Contains(err.Error(), "log one entry per day") {
		t.Errorf("ParseHours(24) error = %v, want mention of one entry per day", err)
	}
}

func TestParseFromTo(t *testing.T) {
	from, to, err := ParseFromTo("2026-10-02 10:00", "2026-10-02 12:00")
	if err != nil {
		t.Fatalf("ParseFromTo() error = %v", err)
	}
	if d := to.Sub(from); d != 2*time.Hour {
		t.Errorf("duration = %v, want 2h", d)
	}
	if from.Format("2006-01-02 15:04") != "2026-10-02 10:00" {
		t.Errorf("from = %v", from)
	}

	if _, _, err := ParseFromTo("2026-10-02 12:00", "2026-10-02 10:00"); err == nil {
		t.Error("to < from: error = nil, want failure")
	}
	if _, _, err := ParseFromTo("2026-10-02 10:00", "2026-10-02 10:00"); err == nil {
		t.Error("to == from: error = nil, want failure")
	}
	if _, _, err := ParseFromTo("yesterday", "2026-10-02 10:00"); err == nil {
		t.Error("bad from format: error = nil, want failure")
	}
	if _, _, err := ParseFromTo("2026-10-02 10:00", "10/02/2026"); err == nil {
		t.Error("bad to format: error = nil, want failure")
	}

	for _, tt := range []struct{ from, to string }{
		{"2026-10-02 10:00", "2026-10-03 10:00"}, // exactly 24h
		{"2026-10-02 10:00", "2026-10-04 12:00"}, // multi-day
	} {
		if _, _, err := ParseFromTo(tt.from, tt.to); err == nil {
			t.Errorf("span %s..%s: error = nil, want 24h rejection", tt.from, tt.to)
		} else if !strings.Contains(err.Error(), "log one entry per day") {
			t.Errorf("span %s..%s: error = %q, want mention of one entry per day", tt.from, tt.to, err)
		}
	}

	// Overnight but under 24h is still a valid single entry.
	if _, _, err := ParseFromTo("2026-10-02 22:00", "2026-10-03 05:00"); err != nil {
		t.Errorf("overnight under 24h: error = %v, want success", err)
	}
}

// noCalls returns a server that fails the test if it is ever hit.
func noCalls(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server must not be called, got %s %s", r.Method, r.URL.Path)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAddRejectsHoursWithFromTo(t *testing.T) {
	srv := noCalls(t)
	cmd := testCmd(srv, Options{}, testMeta())
	out, err := exec(t, cmd, "add", "1", "--hours", "1.5", "--from", "2026-10-02 10:00")
	if err == nil {
		t.Fatal("hours+from: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want mention of mutually exclusive", err)
	}
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
}

func TestAddRequiresInterval(t *testing.T) {
	srv := noCalls(t)
	cmd := testCmd(srv, Options{}, testMeta())
	_, err := exec(t, cmd, "add", "1")
	if err == nil {
		t.Fatal("no interval: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "--hours") {
		t.Errorf("error = %q, want mention of --hours", err)
	}
}

func TestAddRequiresFromAndToTogether(t *testing.T) {
	srv := noCalls(t)
	cmd := testCmd(srv, Options{}, testMeta())
	_, err := exec(t, cmd, "add", "1", "--from", "2026-10-02 10:00")
	if err == nil {
		t.Fatal("from without to: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "together") {
		t.Errorf("error = %q, want mention of togetherness", err)
	}
}

func TestAddRejectsHoursOf24OrMore(t *testing.T) {
	srv := noCalls(t)
	cmd := testCmd(srv, Options{}, testMeta())
	_, err := exec(t, cmd, "add", "1", "--hours", "24")
	if err == nil {
		t.Fatal("--hours 24: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "log one entry per day") {
		t.Errorf("error = %q, want mention of one entry per day", err)
	}
}

func TestAddRejectsMultiDayFromTo(t *testing.T) {
	srv := noCalls(t)
	cmd := testCmd(srv, Options{}, testMeta())
	_, err := exec(t, cmd, "add", "1",
		"--from", "2026-10-02 10:00", "--to", "2026-10-04 12:00")
	if err == nil {
		t.Fatal("multi-day span: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "log one entry per day") {
		t.Errorf("error = %q, want mention of one entry per day", err)
	}
}

func TestAddRejectsNonNumericDirectoryWorkType(t *testing.T) {
	srv := noCalls(t)
	cmd := testCmd(srv, Options{}, testMeta())
	_, err := exec(t, cmd, "add", "1", "--hours", "1", "--work-type=-1")
	if err == nil {
		t.Fatal("--work-type -1: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "work type") || !strings.Contains(err.Error(), "numeric key") {
		t.Errorf("error = %q, want mention of work type and numeric key", err)
	}
}

func TestAddRejectsUnknownListWorkType(t *testing.T) {
	srv := noCalls(t)
	meta := testMeta()
	meta.WorkTypeDirectory = 0
	meta.WorkTypeValues = []string{"Работа", "Bugfix"}
	cmd := testCmd(srv, Options{}, meta)
	_, err := exec(t, cmd, "add", "1", "--hours", "1", "--work-type", "Прочее")
	if err == nil {
		t.Fatal("unknown list value: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "allowed values") {
		t.Errorf("error = %q, want allowed values listed", err)
	}
}

func TestAddWorkTypeWithoutFieldFails(t *testing.T) {
	srv := noCalls(t) // rejected before any API call
	meta := testMeta()
	meta.FieldWorkType = 0
	cmd := testCmd(srv, Options{}, meta)
	_, err := exec(t, cmd, "add", "1", "--hours", "1", "--work-type", "5")
	if err == nil {
		t.Fatal("work type without field: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "work type") {
		t.Errorf("error = %q, want mention of work type", err)
	}
}

func TestAddRoundsHoursToWholeMinutes(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = jsonDecode(r, &body)
		_, _ = w.Write([]byte(`{"result":"success","keys":[77],"commentId":77}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{}, testMeta())
	if _, err := exec(t, cmd, "add", "1", "--hours", "1.907"); err != nil {
		t.Fatalf("add error = %v", err)
	}
	cfd := entryFields(t, body)
	timeVal := cfd[1].(map[string]any)["value"].(map[string]any)
	fromStr := timeVal["from"].(map[string]any)["time"].(string)
	toStr := timeVal["to"].(map[string]any)["time"].(string)
	from, err := time.Parse("15:04", fromStr)
	if err != nil {
		t.Fatalf("parse from %q: %v", fromStr, err)
	}
	to, err := time.Parse("15:04", toStr)
	if err != nil {
		t.Fatalf("parse to %q: %v", toStr, err)
	}
	mins := to.Sub(from).Minutes()
	if mins < 0 {
		mins += 24 * 60
	}
	// 1.907h = 114.42min, rounded to 114 whole minutes.
	if mins != 114 {
		t.Errorf("interval %s-%s = %v minutes, want 114 (rounded)", fromStr, toStr, mins)
	}
}

func TestAddHoursComputesInterval(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = jsonDecode(r, &body)
		_, _ = w.Write([]byte(`{"result":"success","keys":[77],"commentId":77}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{}, testMeta())
	out, err := exec(t, cmd, "add", "1", "--hours", "1.5", "--date", "2026-10-02")
	if err != nil {
		t.Fatalf("add error = %v", err)
	}
	tag, _ := body["dataTag"].(map[string]any)
	if tag["id"] != float64(123) {
		t.Errorf("dataTag = %v, want id 123", body["dataTag"])
	}
	cfd := entryFields(t, body)
	if len(cfd) != 2 {
		t.Fatalf("customFieldData len = %d, want 2 (no work type): %v", len(cfd), cfd)
	}
	dateVal := cfd[0].(map[string]any)["value"].(map[string]any)
	if dateVal["date"] != "02-10-2026" {
		t.Errorf("date = %v, want 02-10-2026", dateVal["date"])
	}
	timeVal := cfd[1].(map[string]any)["value"].(map[string]any)
	fromStr := timeVal["from"].(map[string]any)["time"].(string)
	toStr := timeVal["to"].(map[string]any)["time"].(string)
	from, err := time.Parse("15:04", fromStr)
	if err != nil {
		t.Fatalf("parse from %q: %v", fromStr, err)
	}
	to, err := time.Parse("15:04", toStr)
	if err != nil {
		t.Fatalf("parse to %q: %v", toStr, err)
	}
	mins := to.Sub(from).Minutes()
	if mins < 0 {
		mins += 24 * 60
	}
	if mins != 90 {
		t.Errorf("interval %s-%s = %v minutes, want 90", fromStr, toStr, mins)
	}
	if !strings.Contains(out, "Logged worklog 77") {
		t.Errorf("output = %q, want \"Logged worklog 77\"", out)
	}
}

func TestAddFromToUsesGivenInterval(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = jsonDecode(r, &body)
		_, _ = w.Write([]byte(`{"result":"success","keys":[77],"commentId":77}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{}, testMeta())
	_, err := exec(t, cmd, "add", "1",
		"--from", "2026-10-02 10:00", "--to", "2026-10-02 12:00")
	if err != nil {
		t.Fatalf("add error = %v", err)
	}
	cfd := entryFields(t, body)
	if len(cfd) != 2 {
		t.Fatalf("customFieldData len = %d, want 2: %v", len(cfd), cfd)
	}
	dateVal := cfd[0].(map[string]any)["value"].(map[string]any)
	if dateVal["date"] != "02-10-2026" {
		t.Errorf("date = %v, want 02-10-2026 (derived from --from)", dateVal["date"])
	}
	timeVal := cfd[1].(map[string]any)["value"].(map[string]any)
	from := timeVal["from"].(map[string]any)["time"]
	to := timeVal["to"].(map[string]any)["time"]
	if from != "10:00" || to != "12:00" {
		t.Errorf("period = %v-%v, want 10:00-12:00", from, to)
	}
}

func TestAddWorkTypeInBody(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = jsonDecode(r, &body)
		_, _ = w.Write([]byte(`{"result":"success","keys":[77],"commentId":77}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{}, testMeta())
	if _, err := exec(t, cmd, "add", "1", "--hours", "1", "--work-type", "5"); err != nil {
		t.Fatalf("add error = %v", err)
	}
	cfd := entryFields(t, body)
	if len(cfd) != 3 {
		t.Fatalf("customFieldData len = %d, want 3", len(cfd))
	}
	wt := cfd[2].(map[string]any)
	if wt["field"].(map[string]any)["id"] != float64(101) {
		t.Errorf("work type field = %v, want 101", wt["field"])
	}
	if wt["value"].(map[string]any)["id"] != float64(5) {
		t.Errorf("work type value = %v, want 5", wt["value"])
	}
}

func TestAddNotePostsFollowUpComment(t *testing.T) {
	var bodies []map[string]any
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = jsonDecode(r, &b)
		bodies = append(bodies, b)
		queries = append(queries, r.URL.RawQuery)
		_, _ = w.Write([]byte(`{"result":"success","keys":[77],"commentId":77}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{}, testMeta())
	if _, err := exec(t, cmd, "add", "1", "--hours", "1", "--note", "did stuff"); err != nil {
		t.Fatalf("add error = %v", err)
	}
	if len(bodies) != 2 {
		t.Fatalf("requests = %d, want 2 (entry + note)", len(bodies))
	}
	tag, _ := bodies[0]["dataTag"].(map[string]any)
	if tag["id"] != float64(123) {
		t.Errorf("first request dataTag = %v, want id 123", bodies[0]["dataTag"])
	}
	if bodies[1]["description"] != "did stuff" {
		t.Errorf("note description = %v, want \"did stuff\"", bodies[1]["description"])
	}
	if queries[1] != "silent=true" {
		t.Errorf("note query = %q, want silent=true", queries[1])
	}
}

func TestAddNoteWritesNoteField(t *testing.T) {
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = jsonDecode(r, &b)
		bodies = append(bodies, b)
		_, _ = w.Write([]byte(`{"result":"success","keys":[77],"commentId":77}`))
	}))
	defer srv.Close()

	meta := testMeta()
	meta.FieldNote = 202
	cmd := testCmd(srv, Options{}, meta)
	if _, err := exec(t, cmd, "add", "1", "--hours", "1", "--note", "did stuff"); err != nil {
		t.Fatalf("add error = %v", err)
	}
	if len(bodies) != 1 {
		t.Fatalf("requests = %d, want 1 (note lives in the entry)", len(bodies))
	}
	cfd := entryFields(t, bodies[0])
	if len(cfd) != 3 {
		t.Fatalf("customFieldData len = %d, want 3", len(cfd))
	}
	note := cfd[2].(map[string]any)
	if note["field"].(map[string]any)["id"] != float64(202) {
		t.Errorf("note field = %v, want 202", note["field"])
	}
	if note["value"] != "did stuff" {
		t.Errorf("note value = %v, want \"did stuff\"", note["value"])
	}
}

func TestAddNoteFailureSurfacesWorklogID(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_, _ = w.Write([]byte(`{"result":"success","keys":[77],"commentId":77}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"result":"failure","code":3,"message":"boom"}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{}, testMeta())
	out, err := exec(t, cmd, "add", "1", "--hours", "1", "--note", "did stuff")
	if err == nil {
		t.Fatal("note failure: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "worklog 77 created; note failed") {
		t.Errorf("error = %q, want mention of created worklog 77", err)
	}
	if out != "" {
		t.Errorf("output = %q, want empty (no success line)", out)
	}
}

func TestAddQuietPrintsIDOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","keys":[77],"commentId":77}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{Quiet: true}, testMeta())
	out, err := exec(t, cmd, "add", "1", "--hours", "1")
	if err != nil {
		t.Fatalf("add error = %v", err)
	}
	if out != "77\n" {
		t.Errorf("quiet output = %q, want \"77\\n\"", out)
	}
}

func TestAddJSONOutputsPrettyRaw(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","keys":[77],"commentId":77}`))
	}))
	defer srv.Close()

	cmd := testCmd(srv, Options{JSON: true}, testMeta())
	out, err := exec(t, cmd, "add", "1", "--hours", "1")
	if err != nil {
		t.Fatalf("add error = %v", err)
	}
	if !strings.Contains(out, "{\n  \"result\"") || !strings.Contains(out, "\"commentId\": 77") {
		t.Errorf("json output not pretty-printed:\n%s", out)
	}
}

func TestAddRefreshFlagReachesMetaFunc(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":"success","keys":[77],"commentId":77}`))
	}))
	defer srv.Close()

	var seen MetaOpts
	cmd := NewCmd(stubClient(srv), func() Options { return Options{} }, stubMeta(testMeta(), &seen))
	if _, err := exec(t, cmd, "add", "1", "--hours", "1", "--refresh-worklog-meta", "--data-tag", "9"); err != nil {
		t.Fatalf("add error = %v", err)
	}
	if !seen.Refresh {
		t.Error("--refresh-worklog-meta did not reach MetaFunc")
	}
	if seen.DataTag != "9" {
		t.Errorf("MetaOpts.DataTag = %q, want \"9\"", seen.DataTag)
	}
}

const listCommentsResponse = `{"result":"success","comments":[
	{"id":1,"owner":{"id":"user:3","name":"X"}},
	{"id":3,"owner":{"id":"user:3","name":"Ann"}}
]}`

const listEntriesResponse = `{"result":"success","dataTagEntries":[
	{"key":1,"commentId":3,"customFieldData":[
		{"field":{"id":456},"value":{"date":"02-10-2026"}},
		{"field":{"id":789},"value":{"from":{"time":"10:00"},"to":{"time":"12:00"}}},
		{"field":{"id":101},"value":{"id":1,"name":"Dev"}}
	]}
]}`

func listServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/task/1/comments/list":
			_, _ = w.Write([]byte(listCommentsResponse))
		case "/datatag/123/entry/list":
			_, _ = w.Write([]byte(listEntriesResponse))
		default:
			t.Errorf("unexpected path = %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestListRendersTable(t *testing.T) {
	cmd := testCmd(listServer(t), Options{}, testMeta())
	out, err := exec(t, cmd, "list", "1")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	for _, want := range []string{
		"DATE", "FROM–TO", "HOURS", "WORK TYPE", "AUTHOR",
		"02-10-2026", "10:00–12:00", "2", "Dev", "Ann",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %q:\n%s", want, out)
		}
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("line count = %d, want 2 (header + row):\n%s", len(lines), out)
	}
}

func TestListQuietDropsHeader(t *testing.T) {
	cmd := testCmd(listServer(t), Options{Quiet: true}, testMeta())
	out, err := exec(t, cmd, "list", "1")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if strings.Contains(out, "AUTHOR") || strings.Contains(out, "DATE") {
		t.Errorf("quiet list must drop the header:\n%s", out)
	}
	if !strings.Contains(out, "02-10-2026") || !strings.Contains(out, "Ann") {
		t.Errorf("quiet list must keep data rows:\n%s", out)
	}
}

func TestListJSONOutputsPrettyRaw(t *testing.T) {
	cmd := testCmd(listServer(t), Options{JSON: true}, testMeta())
	out, err := exec(t, cmd, "list", "1")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if !strings.Contains(out, "{\n  \"result\"") || !strings.Contains(out, "\"dataTagEntries\"") {
		t.Errorf("json output not pretty-printed:\n%s", out)
	}
}

// discoverServer stubs the worklog discovery endpoints and counts hits.
func discoverServer(t *testing.T, hits *int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hits++
		switch r.URL.Path {
		case "/datatag/list":
			_, _ = w.Write([]byte(`{"result":"success","dataTags":[
				{"id":2,"name":"Фактическое время"}
			]}`))
		case "/datatag/2":
			_, _ = w.Write([]byte(`{"result":"success","dataTag":{"id":2,"name":"Фактическое время","fields":[
				{"id":10,"name":"Дата","type":3},
				{"id":11,"name":"Время","type":6},
				{"id":12,"name":"Вид работ","type":9,"directoryId":3},
				{"id":13,"name":"Сотрудник","type":11}
			]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDefaultMetaFuncReturnsProfileWorklog(t *testing.T) {
	srv := noCalls(t)
	c, err := planfix.New("example.com", "tok")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = srv.URL

	want := &config.WorklogMeta{DataTag: "7", DataTagID: 7, FieldDate: 8, FieldTime: 9}
	fn := DefaultMetaFunc(func() (string, *config.Profile, error) {
		return "default", &config.Profile{Worklog: want}, nil
	})
	got, err := fn(context.Background(), c, MetaOpts{})
	if err != nil {
		t.Fatalf("MetaFunc() error = %v", err)
	}
	if got != want {
		t.Errorf("meta = %+v, want profile copy %+v", got, want)
	}
}

func TestDefaultMetaFuncResolvesPinCachesAndRefreshes(t *testing.T) {
	t.Setenv("PLANFIX_CONFIG", filepath.Join(t.TempDir(), "config.yml"))
	path := config.ResolvePath()
	if err := config.Save(path, &config.Config{
		Profiles: map[string]*config.Profile{
			"default": {Domain: "example.com", Token: "tok"},
		},
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	var hits int
	srv := discoverServer(t, &hits)
	c, err := planfix.New("example.com", "tok")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = srv.URL

	// Profile pin without a resolved schema: later calls must be cached.
	fn := DefaultMetaFunc(func() (string, *config.Profile, error) {
		return "default", &config.Profile{Worklog: &config.WorklogMeta{DataTag: "2"}}, nil
	})

	got, err := fn(context.Background(), c, MetaOpts{})
	if err != nil {
		t.Fatalf("first call error = %v", err)
	}
	if got.DataTag != "2" || got.DataTagID != 2 || got.FieldDate != 10 || got.FieldTime != 11 ||
		got.FieldWorkType != 12 || got.WorkTypeDirectory != 3 {
		t.Errorf("resolved meta = %+v", got)
	}
	if hits != 1 {
		t.Fatalf("hits after resolve = %d, want 1 (GET /datatag/2)", hits)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	persisted := cfg.Profiles["default"].Worklog
	if persisted == nil || persisted.DataTagID != 2 || persisted.DataTag != "2" {
		t.Errorf("persisted worklog = %+v, want pin 2 resolved to tag 2", persisted)
	}

	if _, err := fn(context.Background(), c, MetaOpts{}); err != nil {
		t.Fatalf("second call error = %v", err)
	}
	if hits != 1 {
		t.Errorf("hits after cached call = %d, want 1 (served from cache)", hits)
	}

	if _, err := fn(context.Background(), c, MetaOpts{Refresh: true}); err != nil {
		t.Fatalf("refresh call error = %v", err)
	}
	if hits != 2 {
		t.Errorf("hits after refresh = %d, want 2 (re-resolved)", hits)
	}
}

func TestDefaultMetaFuncReResolvesOnPinChange(t *testing.T) {
	t.Setenv("PLANFIX_CONFIG", filepath.Join(t.TempDir(), "config.yml"))

	var hits int
	srv := discoverServer(t, &hits)
	c, err := planfix.New("example.com", "tok")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = srv.URL

	// Cached block from another tag plus a changed pin: the stale field ids
	// must not be served.
	stale := &config.WorklogMeta{DataTag: "2", DataTagID: 5, FieldDate: 50, FieldTime: 51}
	fn := DefaultMetaFunc(func() (string, *config.Profile, error) {
		return "default", &config.Profile{Worklog: stale}, nil
	})
	got, err := fn(context.Background(), c, MetaOpts{})
	if err != nil {
		t.Fatalf("MetaFunc() error = %v", err)
	}
	if got.DataTagID != 2 || got.FieldDate != 10 {
		t.Errorf("meta = %+v, want re-resolved schema of tag 2", got)
	}
}

func TestDefaultMetaFuncNoPinFailsListingCandidates(t *testing.T) {
	var hits int
	srv := discoverServer(t, &hits)
	c, err := planfix.New("example.com", "tok")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = srv.URL

	fn := DefaultMetaFunc(func() (string, *config.Profile, error) {
		return "default", &config.Profile{}, nil
	})
	_, err = fn(context.Background(), c, MetaOpts{})
	if err == nil {
		t.Fatal("no pin: error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "not configured") || !strings.Contains(err.Error(), "Фактическое время") {
		t.Errorf("err = %v, want guidance with candidate tags", err)
	}
}

func TestDefaultMetaFuncDataTagOverrideSkipsCache(t *testing.T) {
	var hits int
	srv := discoverServer(t, &hits)
	c, err := planfix.New("example.com", "tok")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = srv.URL

	fn := DefaultMetaFunc(func() (string, *config.Profile, error) {
		return "default", &config.Profile{}, nil
	})
	got, err := fn(context.Background(), c, MetaOpts{DataTag: "2"})
	if err != nil {
		t.Fatalf("override call error = %v", err)
	}
	if got.DataTagID != 2 {
		t.Errorf("meta = %+v, want tag 2 from the override", got)
	}
}

func TestDefaultMetaFuncWarnsAndContinuesWhenPersistFails(t *testing.T) {
	// A directory path makes config.Load fail, so persistMeta errors.
	t.Setenv("PLANFIX_CONFIG", t.TempDir())

	var hits int
	srv := discoverServer(t, &hits)
	c, err := planfix.New("example.com", "tok")
	if err != nil {
		t.Fatal(err)
	}
	c.BaseURL = srv.URL

	fn := DefaultMetaFunc(func() (string, *config.Profile, error) {
		return "default", &config.Profile{Worklog: &config.WorklogMeta{DataTag: "2"}}, nil
	})

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stderr
	os.Stderr = w
	got, callErr := fn(context.Background(), c, MetaOpts{})
	os.Stderr = old
	_ = w.Close()
	var warn bytes.Buffer
	_, _ = io.Copy(&warn, r)
	_ = r.Close()

	if callErr != nil {
		t.Fatalf("MetaFunc() error = %v, want success with warning", callErr)
	}
	if got == nil || got.DataTagID != 2 {
		t.Fatalf("meta = %+v, want resolved meta despite persist failure", got)
	}
	if !strings.Contains(warn.String(), "warning") {
		t.Errorf("stderr = %q, want persist warning", warn.String())
	}
}
