package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
)

// parseFlags parses args into fs. Help requests (-h, -help, --help) print the
// flag usage to stdout and report help=true; parse errors are written to
// stderr. Callers return 0 for help and 2 for any other error.
func parseFlags(fs *flag.FlagSet, usageLine string, args []string) (help bool, err error) {
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	fs.Usage = func() {
		fmt.Fprintf(&buf, "Usage: %s\n\nFlags:\n", usageLine)
		fs.PrintDefaults()
	}
	err = fs.Parse(args)
	switch {
	case err == nil:
		return false, nil
	case errors.Is(err, flag.ErrHelp):
		_, _ = buf.WriteTo(os.Stdout)
		return true, nil
	default:
		_, _ = buf.WriteTo(os.Stderr)
		return false, err
	}
}

func runSummary(args []string) int {
	fs := flag.NewFlagSet("summary", flag.ContinueOnError)
	input := fs.String("input", "", "path to a JSONL log file")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")

	if help, err := parseFlags(fs, "runlens summary -input FILE [-json]", args); err != nil {
		return 2
	} else if help {
		return 0
	}
	if *input == "" {
		fmt.Fprintln(os.Stderr, "-input is required")
		return 2
	}

	events, err := loadJSONL(*input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	summary := summarize(events)

	if *jsonOutput {
		body, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(string(body))
		return 0
	}

	printSummary(summary)
	return 0
}

func runDiagnose(args []string) int {
	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	input := fs.String("input", "", "path to a JSONL log file")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")

	if help, err := parseFlags(fs, "runlens diagnose -input FILE [-json]", args); err != nil {
		return 2
	} else if help {
		return 0
	}
	if *input == "" {
		fmt.Fprintln(os.Stderr, "-input is required")
		return 2
	}

	events, err := loadJSONL(*input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	report := diagnose(events)

	if *jsonOutput {
		body, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(string(body))
		return 0
	}

	printDiagnose(report)
	return 0
}
