// Command nightly-base prints the commit the nightly's changed-sites mutation step judges from
// (T-161, ADR-0049): the head commit of the last run of nightly.yml on main that concluded
// success, so every Go function changed since the last green night is judged, strictly covered,
// until a green night moves the base.
//
//	base=$(go run ./tools/bin/nightly-base) && tools/bin/pinned itos-cc mutation run --since "$base" …
//
// The run is read with gh (gh run list --workflow nightly.yml --branch main --status success
// --limit 1), with the token gh has: the nightly's step hands it github.token, whose actions: read
// lets it list the workflow's runs; by hand, a person's own login. Its head commit must be HEAD or
// an ancestor of HEAD in this clone (the nightly checks out the whole history).
//
// With no green nightly in reach (gh failing, no green run, or a head commit that is not in
// HEAD's history), it says so on stderr and prints the base of the last 24 hours' first-parent
// range instead: the newest commit on HEAD's first-parent line committed 24 hours ago or earlier,
// HEAD itself when nothing newer was committed (a range with no site, which itos-cc passes as not
// applicable), or the line's oldest commit when all of it is younger. It never prints nothing, so
// the step never judges every function of the tree. Stderr names the base it chose and why, so
// the night's log says what was judged.
//
// It imports nothing but the standard library, as the other gate programs do, and runs git and gh
// by name.
//
// Exit status: 0 a base printed, 2 a usage error or a git that could not read HEAD's history.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// window is how far back the fallback range reaches: one night's commits.
const window = 24 * time.Hour

// runner runs a program by name and returns its stdout; an error carries its stderr.
type runner func(name string, args ...string) (string, error)

func main() {
	os.Exit(run(os.Args[1:], execRun, time.Now(), os.Stdout, os.Stderr))
}

func run(args []string, cmd runner, now time.Time, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("nightly-base", flag.ContinueOnError)
	flags.SetOutput(stderr)
	workflow := flags.String("workflow", "nightly.yml", "the workflow whose last green run is the base")
	branch := flags.String("branch", "main", "the branch its runs are read on")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "nightly-base: unexpected argument %q\n", flags.Arg(0))
		return 2
	}
	base, note, err := find(cmd, now, *workflow, *branch)
	if err != nil {
		fmt.Fprintf(stderr, "nightly-base: %v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "nightly-base: %s\n", note)
	fmt.Fprintln(stdout, base)
	return 0
}

// greenRun is what gh run list says of a run, as much as this reads.
type greenRun struct {
	DatabaseID int64  `json:"databaseId"`
	HeadSha    string `json:"headSha"`
	CreatedAt  string `json:"createdAt"`
}

// find is the base and a sentence saying where it came from: the last green run's head commit,
// or, with none in reach, the last 24 hours' first-parent base. An error only when git cannot
// read HEAD's first-parent line.
func find(cmd runner, now time.Time, workflow, branch string) (string, string, error) {
	r, why := lastGreen(cmd, workflow, branch)
	if why == "" {
		return r.HeadSha, fmt.Sprintf("the base is %s, the head commit of the last green run of %s on %s (run %d, %s)",
			r.HeadSha, workflow, branch, r.DatabaseID, r.CreatedAt), nil
	}
	base, err := dayAgo(cmd, now)
	if err != nil {
		return "", "", err
	}
	return base, fmt.Sprintf("no green nightly in reach (%s); judging the last 24 hours' first-parent range instead, since %s", why, base), nil
}

// lastGreen is the last green run of workflow on branch, or why there is none in reach.
func lastGreen(cmd runner, workflow, branch string) (greenRun, string) {
	out, err := cmd("gh", "run", "list", "--workflow", workflow, "--branch", branch, "--status", "success",
		"--limit", "1", "--json", "databaseId,headSha,createdAt")
	if err != nil {
		return greenRun{}, "gh run list failed: " + firstLine(err.Error())
	}
	var runs []greenRun
	if err := json.Unmarshal([]byte(out), &runs); err != nil {
		return greenRun{}, "gh run list said no JSON: " + firstLine(out)
	}
	if len(runs) == 0 || runs[0].HeadSha == "" {
		return greenRun{}, fmt.Sprintf("no run of %s on %s concluded success", workflow, branch)
	}
	r := runs[0]
	if _, err := cmd("git", "merge-base", "--is-ancestor", r.HeadSha, "HEAD"); err != nil {
		return greenRun{}, fmt.Sprintf("the last green run's head commit %s is not in HEAD's history", r.HeadSha)
	}
	return r, ""
}

// dayAgo is the newest commit on HEAD's first-parent line committed at least 24 hours before now,
// or the line's oldest when every commit on it is younger.
func dayAgo(cmd runner, now time.Time) (string, error) {
	out, err := cmd("git", "log", "--first-parent", "--format=%H %ct", "HEAD")
	if err != nil {
		return "", fmt.Errorf("git could not read HEAD's first-parent history: %s", firstLine(err.Error()))
	}
	cutoff := now.Add(-window).Unix()
	oldest := ""
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		sha, at, ok := strings.Cut(line, " ")
		seconds, err := strconv.ParseInt(at, 10, 64)
		if !ok || err != nil {
			return "", fmt.Errorf("git log said %q, not a commit and its time", line)
		}
		if seconds <= cutoff {
			return sha, nil
		}
		oldest = sha
	}
	if oldest == "" {
		return "", errors.New("HEAD has no commits")
	}
	return oldest, nil
}

// execRun runs name with args, returning its stdout, or an error naming its stderr.
func execRun(name string, args ...string) (string, error) {
	c := exec.Command(name, args...)
	var stderr strings.Builder
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return string(out), fmt.Errorf("%s: %w: %s", name, err, msg)
		}
		return string(out), fmt.Errorf("%s: %w", name, err)
	}
	return string(out), nil
}

// firstLine is s's first non-empty line, trimmed.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return "(nothing)"
}
