package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/out"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Under --json verify prints one object: the range, commits.since when the
// config names one, each commit's result, the count and the range checks,
// its logs on stderr.
func TestVerifyRangeJSON(t *testing.T) {
	notesRepo(t)
	since := strings.TrimSpace(gitIn(t, "rev-parse", "HEAD"))
	writeFile(t, "itos.yaml", "version: 1\ncommits:\n  types: [docs]\n  since: "+since+"\n")
	gitIn(t, "add", "itos.yaml")
	gitIn(t, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "docs: add the config")
	head := strings.TrimSpace(gitIn(t, "rev-parse", "HEAD"))
	var stdout, stderr strings.Builder
	code, err := runVerify([]string{"", "HEAD"}, Out{JSON: true, Stdout: &stdout, Stderr: &stderr})
	if err != nil || code != 0 {
		t.Fatalf("exit %d, %v\n%s", code, err, stderr.String())
	}
	var got struct {
		Range   span   `json:"range"`
		Since   string `json:"since"`
		Commits []struct {
			SHA string `json:"sha"`
			OK  bool   `json:"ok"`
		} `json:"commits"`
		Passed      int  `json:"passed"`
		Total       int  `json:"total"`
		RangeChecks bool `json:"range_checks"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &got); err != nil {
		t.Fatalf("%v: %s", err, stdout.String())
	}
	if got.Range != (span{"", "HEAD"}) || got.Since != since || len(got.Commits) != 1 || got.Commits[0].SHA != head ||
		!got.Commits[0].OK || got.Passed != 1 || got.Total != 1 || !got.RangeChecks {
		t.Errorf("got %+v", got)
	}
	if !strings.Contains(stderr.String(), "1/1 commits pass the commit rules\n") {
		t.Errorf("stderr %q", stderr.String())
	}
}

// The ports bind the stealth check to the config's path: with no range, a
// project's config is a usage error, and the stealth one judges the
// commits on no remote branch, every one here.
func TestVerifyPortsAskTheStealthConfig(t *testing.T) {
	_, gitDir := notesRepo(t)
	writeFile(t, "itos.yaml", "version: 1\ncommits:\n  types: [docs]\n")
	var stdout, stderr strings.Builder
	_, err := runVerify(nil, Out{Stdout: &stdout, Stderr: &stderr})
	var u out.UsageError
	if !errors.As(err, &u) || !strings.Contains(err.Error(), "verify needs <from> <to>") {
		t.Fatalf("a project's config: %v", err)
	}
	if err := os.Remove("itos.yaml"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(gitDir, "itos", "itos.yaml"), "version: 1\ncommits:\n  types: [docs]\n")
	code, err := runVerify(nil, Out{JSON: true, Stdout: &stdout, Stderr: &stderr})
	if err != nil || code != 0 {
		t.Fatalf("exit %d, %v\n%s", code, err, stderr.String())
	}
	var got struct {
		Range span `json:"range"`
		Total int  `json:"total"`
	}
	if err := json.Unmarshal([]byte(stdout.String()), &got); err != nil {
		t.Fatalf("%v: %s", err, stdout.String())
	}
	if got.Range != (span{vcsport.Unpushed, "HEAD"}) || got.Total != 1 {
		t.Errorf("got %+v", got)
	}
}

// A range given with only its start is a usage error, and verify exits 0
// beside it: the dispatcher reads the error, never the code.
func TestVerifyWithOneEndOfARangeIsAUsageError(t *testing.T) {
	var stdout, stderr strings.Builder
	code, err := runVerify([]string{"HEAD"}, Out{Stdout: &stdout, Stderr: &stderr})
	var u out.UsageError
	if code != 0 || !errors.As(err, &u) || !strings.Contains(err.Error(), "verify needs <from> <to>") {
		t.Errorf("exit %d, %v, want 0 and the usage error", code, err)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Errorf("stdout %q, stderr %q, want none", stdout.String(), stderr.String())
	}
}
