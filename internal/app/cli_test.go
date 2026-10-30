package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests swap os.Stdout/os.Stderr, so none of them call t.Parallel.

func runCapturing(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	oldOut, oldErr := os.Stdout, os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = outW, errW
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()

	code = Run(args)

	os.Stdout, os.Stderr = oldOut, oldErr
	_ = outW.Close()
	_ = errW.Close()
	read := func(f *os.File) string {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, err := f.Read(buf)
			sb.Write(buf[:n])
			if err != nil {
				break
			}
		}
		_ = f.Close()
		return sb.String()
	}
	return code, read(outR), read(errR)
}

func TestHelpExitsZeroWithUsageOnStdout(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-h"}, "Usage:\n  runlens summary"},
		{[]string{"-help"}, "Usage:\n  runlens summary"},
		{[]string{"--help"}, "Usage:\n  runlens summary"},
		{[]string{"help"}, "Usage:\n  runlens summary"},
		{[]string{"help", "-h"}, "Usage:\n  runlens summary"},
		{[]string{"help", "--help"}, "Usage:\n  runlens summary"},
		{[]string{"help", "help"}, "Usage:\n  runlens summary"},
		{[]string{"help", "summary"}, "Usage: runlens summary"},
		{[]string{"help", "diagnose"}, "Usage: runlens diagnose"},
		{[]string{"help", "tui"}, "Usage: runlens [tui]"},
		{[]string{"help", "interactive"}, "Usage: runlens [tui]"},
		{[]string{"summary", "-h"}, "Usage: runlens summary"},
		{[]string{"summary", "-help"}, "Usage: runlens summary"},
		{[]string{"summary", "--help"}, "Usage: runlens summary"},
		{[]string{"summary", "help"}, "Usage: runlens summary"},
		{[]string{"diagnose", "-h"}, "Usage: runlens diagnose"},
		{[]string{"diagnose", "--help"}, "Usage: runlens diagnose"},
		{[]string{"diagnose", "help"}, "Usage: runlens diagnose"},
		{[]string{"tui", "-h"}, "Usage: runlens [tui]"},
		{[]string{"tui", "--help"}, "Usage: runlens [tui]"},
		{[]string{"tui", "help"}, "Usage: runlens [tui]"},
		{[]string{"interactive", "--help"}, "Usage: runlens [tui]"},
	}
	for _, tc := range cases {
		code, stdout, stderr := runCapturing(t, tc.args...)
		if code != 0 {
			t.Errorf("%v: exit code = %d, want 0 (stderr %q)", tc.args, code, stderr)
		}
		if !strings.Contains(stdout, tc.want) {
			t.Errorf("%v: stdout missing %q:\n%s", tc.args, tc.want, stdout)
		}
		if stderr != "" {
			t.Errorf("%v: unexpected stderr %q", tc.args, stderr)
		}
	}
}

func TestVersionExitsZero(t *testing.T) {
	cases := [][]string{
		{"version"}, {"-version"}, {"--version"},
		{"summary", "version"}, {"summary", "-version"}, {"summary", "--version"},
		{"diagnose", "version"}, {"diagnose", "--version"},
		{"tui", "version"}, {"tui", "--version"},
		{"interactive", "--version"},
	}
	for _, args := range cases {
		code, stdout, stderr := runCapturing(t, args...)
		if code != 0 {
			t.Errorf("%v: exit code = %d, want 0 (stderr %q)", args, code, stderr)
		}
		if !strings.HasPrefix(stdout, "runlens ") || strings.Count(stdout, "\n") != 1 {
			t.Errorf("%v: stdout = %q, want one line starting with %q", args, stdout, "runlens ")
		}
		if stderr != "" {
			t.Errorf("%v: unexpected stderr %q", args, stderr)
		}
	}
}

func TestHelpContent(t *testing.T) {
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"--help"}, []string{"runlens summary", "runlens diagnose", "must not", "runlens version"}},
		{[]string{"summary", "-h"}, []string{"Usage: runlens summary", "-input", "-json"}},
		{[]string{"diagnose", "-h"}, []string{"Usage: runlens diagnose", "-input", "-json"}},
		{[]string{"tui", "-h"}, []string{"Usage: runlens [tui]", "terminal"}},
	}
	for _, tc := range cases {
		_, stdout, _ := runCapturing(t, tc.args...)
		for _, want := range tc.want {
			if !strings.Contains(stdout, want) {
				t.Errorf("%v: help is missing %q:\n%s", tc.args, want, stdout)
			}
		}
	}
}

func TestSubcommandHelpWordMatchesFlag(t *testing.T) {
	for _, name := range []string{"summary", "diagnose", "tui"} {
		_, viaFlag, _ := runCapturing(t, name, "-h")
		_, viaWord, _ := runCapturing(t, name, "help")
		_, viaTop, _ := runCapturing(t, "help", name)
		if viaFlag == "" || viaFlag != viaWord || viaFlag != viaTop {
			t.Errorf("%s help differs between -h, help, and help %s:\n%q\n%q\n%q", name, name, viaFlag, viaWord, viaTop)
		}
	}
}

func TestVersionUsesInjectedVersion(t *testing.T) {
	old := Version
	Version = "1.2.3"
	defer func() { Version = old }()
	for _, args := range [][]string{{"--version"}, {"summary", "version"}} {
		if _, stdout, _ := runCapturing(t, args...); strings.TrimSpace(stdout) != "runlens 1.2.3" {
			t.Errorf("%v: version output = %q", args, stdout)
		}
	}
}

func TestFlagValuesNamedHelpOrVersionAreNotHijacked(t *testing.T) {
	// "help" / "version" as a flag value is a path, not a request for help.
	for _, sub := range []string{"summary", "diagnose"} {
		for _, value := range []string{"help", "version", "-h", "--version"} {
			args := []string{sub, "-input", value}
			code, stdout, stderr := runCapturing(t, args...)
			if code == 0 || stdout != "" || !strings.Contains(stderr, "open log file") {
				t.Errorf("%v: code %d, stdout %q, stderr %q; want a load failure on the path", args, code, stdout, stderr)
			}
		}
	}
}

func TestRealErrorsKeepExitCodes(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.jsonl")
	cases := []struct {
		args       []string
		want       int
		wantStderr string
	}{
		{[]string{"bogus"}, 2, "unknown subcommand"},
		{[]string{"-bogus"}, 2, "unknown subcommand"},
		{[]string{"help", "bogus"}, 2, "unknown subcommand"},
		{[]string{"summary"}, 2, "-input is required"},
		{[]string{"diagnose"}, 2, "-input is required"},
		{[]string{"summary", "-json"}, 2, "-input is required"},
		{[]string{"summary", "-bogus"}, 2, "flag provided but not defined"},
		{[]string{"diagnose", "-bogus"}, 2, "flag provided but not defined"},
		{[]string{"summary", "-input", missing}, 1, "open log file"},
		{[]string{"diagnose", "-input", missing}, 1, "open log file"},
	}
	for _, tc := range cases {
		code, stdout, stderr := runCapturing(t, tc.args...)
		if code != tc.want {
			t.Errorf("%v: exit code = %d, want %d", tc.args, code, tc.want)
		}
		if stdout != "" {
			t.Errorf("%v: errors must not print to stdout, got %q", tc.args, stdout)
		}
		if !strings.Contains(stderr, tc.wantStderr) {
			t.Errorf("%v: stderr %q does not contain %q", tc.args, stderr, tc.wantStderr)
		}
	}
}

func TestUnknownSubcommandPrintsUsageToStderr(t *testing.T) {
	code, stdout, stderr := runCapturing(t, "bogus")
	if code != 2 || stdout != "" {
		t.Errorf("code %d, stdout %q; want 2 and empty stdout", code, stdout)
	}
	for _, want := range []string{`unknown subcommand "bogus"`, "Usage:", "runlens summary"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr missing %q:\n%s", want, stderr)
		}
	}
}

func TestSummaryAndDiagnoseStillRunOnExample(t *testing.T) {
	example := filepath.Join("..", "..", "examples", "run.jsonl")
	for _, sub := range []string{"summary", "diagnose"} {
		code, stdout, stderr := runCapturing(t, sub, "-input", example)
		if code != 0 || strings.TrimSpace(stdout) == "" || stderr != "" {
			t.Errorf("%s: code %d, stdout %q, stderr %q", sub, code, stdout, stderr)
		}
		code, stdout, _ = runCapturing(t, sub, "-input", example, "-json")
		if code != 0 || !strings.HasPrefix(stdout, "{") {
			t.Errorf("%s -json: code %d, stdout %q", sub, code, stdout)
		}
	}
}
