package cli

import (
	"encoding/json"
	"strings"
	"testing"
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
	code, err := verifyRange("", "HEAD", Out{JSON: true, Stdout: &stdout, Stderr: &stderr})
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
