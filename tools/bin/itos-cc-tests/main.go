// Command itos-cc-tests is itos-cc's listed-test protocol over this repository's scenarios, the
// command itos-cc.yaml's mutation.tests.list runs (the push proof's and the nightly's mutation
// runs):
//
//	go run ./tools/bin/itos-cc-tests
//
// prints one live scenario a line, its ID, a tab, then its feature file's path from the
// repository's top. `tools/bin/itos tests list scenario --json`, run from the working folder, is
// the source of truth; this only reshapes its JSON for itos-cc. It exits with itos's code, its
// stderr passed on, when itos fails, and 1 when itos's JSON is not a tests list of tests each
// with a string id, a string file and a boolean live, having printed the live tests before the
// first invalid one. It was tools/bin/itos-cc-tests.ts until T-146.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	repoprogram "github.com/donvargax/itos/v8/tools/bin/repo-program"
)

func main() {
	os.Exit(run(os.Stdout, os.Stderr))
}

// run lists the live scenarios to stdout and returns the exit code.
func run(stdout, stderr io.Writer) int {
	name, args := repoprogram.Command("tools/bin/itos", "tests", "list", "scenario", "--json")
	cmd := exec.Command(name, args...)
	var out, errs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errs
	if err := cmd.Run(); err != nil {
		code := 1
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() > 0 {
			code = exit.ExitCode()
		}
		if errs.Len() > 0 {
			stderr.Write(errs.Bytes())
		} else {
			fmt.Fprintf(stderr, "tools/bin/itos tests list scenario exited %d: %v\n", code, err)
		}
		return code
	}
	return list(out.Bytes(), stdout, stderr)
}

// list writes the live tests of itos's JSON report to stdout, a line each, and returns the exit
// code.
func list(report []byte, stdout, stderr io.Writer) int {
	var parsed any
	if err := json.Unmarshal(report, &parsed); err != nil {
		fmt.Fprintf(stderr, "tools/bin/itos tests list scenario returned invalid JSON: %v\n", err)
		return 1
	}
	object, _ := parsed.(map[string]any)
	tests, ok := object["tests"].([]any)
	if !ok {
		fmt.Fprintln(stderr, "tools/bin/itos tests list scenario returned no tests list")
		return 1
	}
	for _, t := range tests {
		test, _ := t.(map[string]any)
		id, idOK := test["id"].(string)
		file, fileOK := test["file"].(string)
		live, liveOK := test["live"].(bool)
		if !idOK || !fileOK || !liveOK {
			fmt.Fprintln(stderr, "tools/bin/itos tests list scenario returned an invalid test")
			return 1
		}
		if live {
			fmt.Fprintf(stdout, "%s\tfeatures/%s\n", id, file)
		}
	}
	return 0
}
