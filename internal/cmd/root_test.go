package cmd

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"planfix-cli/internal/buildinfo"
)

// execute runs an isolated root command tree and returns its output.
func execute(t *testing.T, args ...string) string {
	t.Helper()
	root := NewRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute(%q) error = %v", args, err)
	}
	return buf.String()
}

func TestRootHelpListsGlobalFlags(t *testing.T) {
	out := execute(t, "--help")
	for _, flag := range []string{"--json", "--fields", "--quiet", "-q", "--profile"} {
		if !strings.Contains(out, flag) {
			t.Errorf("help output missing %q\n--- output ---\n%s", flag, out)
		}
	}
}

func TestRootNoArgsPrintsHelp(t *testing.T) {
	out := execute(t)
	if !strings.Contains(out, "Usage:") {
		t.Errorf("no-args output missing usage section:\n%s", out)
	}
}

func TestGlobalFlagsParse(t *testing.T) {
	execute(t, "--json", "--fields", "id,name", "-q", "--profile", "work")
	want := GlobalOpts{JSON: true, Fields: "id,name", Quiet: true, Profile: "work"}
	if globalOpts != want {
		t.Errorf("globalOpts = %+v, want %+v", globalOpts, want)
	}
}

func TestVersionFlagPrintsVersionString(t *testing.T) {
	out := strings.TrimSpace(execute(t, "--version"))
	if want := VersionString(); out != want {
		t.Errorf("--version output = %q, want %q", out, want)
	}
}

func TestVersionStringFormat(t *testing.T) {
	got := VersionString()
	const pattern = `^planfix \S+ \(commit \S+, built .+\)$`
	if ok := regexp.MustCompile(pattern).MatchString(got); !ok {
		t.Errorf("VersionString() = %q, want format matching %q", got, pattern)
	}
	for _, part := range []string{buildinfo.Version, buildinfo.Commit, buildinfo.Date} {
		if !strings.Contains(got, part) {
			t.Errorf("VersionString() = %q, missing value %q", got, part)
		}
	}
}
