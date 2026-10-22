package ciwatch

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	"github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// The --json keys: ci alone with no run; with one, its jobs a list even
// when it has none, and the cancelled run's address when one was followed.
func TestFieldsNameTheRunAndTheSupersededRun(t *testing.T) {
	keys := func(fields []out.Field) []string {
		var named []string
		for _, f := range fields {
			named = append(named, f.Key)
		}
		return named
	}
	if got := keys(Watched{Outcome: "error"}.Fields()); strings.Join(got, ",") != "ci" {
		t.Fatalf("no run: %v", got)
	}
	fields := Watched{Outcome: "success", Run: &providers.Run{URL: "u"}, Superseded: "c"}.Fields()
	if got := keys(fields); strings.Join(got, ",") != "ci,run,superseded" {
		t.Fatalf("a followed run: %v", got)
	}
	if run := fields[1].Value.(providers.Run); run.Jobs == nil || fields[2].Value != "c" {
		t.Fatalf("a followed run: %+v", fields)
	}
}

// Under --json the progress and the end go to stderr; under -q the
// progress and a success's line go nowhere.
func TestJSONAndQuietMoveTheProgress(t *testing.T) {
	noSleep(t)
	look := func(string) (providers.Run, bool, error) {
		return providers.Run{URL: "u", Status: "completed", Conclusion: "success"}, true, nil
	}
	var stdout, stderr strings.Builder
	w := watchRun(watchConfig(0, 60), providers.Watcher{Look: look}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr, JSON: true})
	if w.Code != 0 || stdout.Len() > 0 || stderr.String() != "CI run: u\nCI passed: u\n" {
		t.Fatalf("--json: code %d\n%q\n%q", w.Code, stdout.String(), stderr.String())
	}
	stderr.Reset()
	w = watchRun(watchConfig(0, 60), providers.Watcher{Look: look}, "abc", "", Out{Stdout: &stdout, Stderr: &stderr, Quiet: true})
	if w.Code != 0 || stdout.Len() > 0 || stderr.Len() > 0 {
		t.Fatalf("-q: code %d\n%q\n%q", w.Code, stdout.String(), stderr.String())
	}
}

// A list of the branch's runs that fails is a look that failed: a
// temporary failure is looked past, any other ends the watch with 3, while
// the commit has no run as after a cancelled one.
func TestAFailingListOfRunsIsAFailingLook(t *testing.T) {
	noSleep(t)
	cancelled := providers.Run{URL: "u", Status: "completed", Conclusion: "cancelled", ID: 1, HeadSHA: "abc", Branch: "main"}
	broken := func(string) ([]providers.Run, error) { return nil, errors.New("401") }
	var stdout, stderr strings.Builder
	o := Out{Stdout: &stdout, Stderr: &stderr}
	none := func(string) (providers.Run, bool, error) { return providers.Run{}, false, nil }
	w := Watch(context.Background(), watchConfig(0, 60), providers.Watcher{Look: none, Runs: broken}, &porttest.VCS{}, "abc", "", "main", o)
	if w.Code != exitMissing || w.Outcome != "error" || !strings.Contains(stderr.String(), "itos: 401; itos ci watch abc") {
		t.Fatalf("no run: code %d, outcome %s\n%s", w.Code, w.Outcome, stderr.String())
	}
	stderr.Reset()
	w = watchRun(watchConfig(0, 60), providers.Watcher{Look: func(string) (providers.Run, bool, error) { return cancelled, true, nil }, Runs: broken}, "abc", "", o)
	if w.Code != exitMissing || w.Outcome != "error" || w.Run == nil || !strings.Contains(stderr.String(), "itos: 401") {
		t.Fatalf("cancelled: code %d, outcome %s\n%s", w.Code, w.Outcome, stderr.String())
	}

	lists := 0
	blip := func(string) ([]providers.Run, error) {
		lists++
		return nil, kind.Wrap(kind.Temporary, errors.New("502"))
	}
	looks := 0
	later := func(string) (providers.Run, bool, error) {
		if looks++; looks == 1 {
			return providers.Run{}, false, nil
		}
		return providers.Run{URL: "u", Status: "completed", Conclusion: "success"}, true, nil
	}
	w = Watch(context.Background(), watchConfig(0, 60), providers.Watcher{Look: later, Runs: blip}, &porttest.VCS{}, "abc", "", "main", o)
	if w.Code != 0 || lists != 1 || looks != 2 {
		t.Fatalf("a blip: code %d, lists %d, looks %d", w.Code, lists, looks)
	}
}

// fetches is a repository that counts its fetches.
type fetches struct {
	*porttest.VCS
	remotes []string
}

func (f *fetches) Fetch(ctx context.Context, remote, refspec string, talk port.Talk) (string, error) {
	f.remotes = append(f.remotes, remote)
	return f.VCS.Fetch(ctx, remote, refspec, talk)
}

// With no runs or no branch nothing covers the commit; a head the clone
// lacks is fetched for once, from the remote when there is one, and still
// lacking is passed over.
func TestCoveringFetchesAMissingHeadOnce(t *testing.T) {
	ctx := context.Background()
	repo := &fetches{VCS: &porttest.VCS{History: []porttest.Commit{{Name: "abc"}}}}
	if _, ok, err := Covering(ctx, nil, repo, "main", "abc", "origin", nil, ""); ok || err != nil {
		t.Fatalf("no runs: %v %v", ok, err)
	}
	missing := func(string) ([]providers.Run, error) {
		return []providers.Run{{ID: 2, HeadSHA: "new"}, {ID: 1, HeadSHA: "newer"}}, nil
	}
	if _, ok, err := Covering(ctx, missing, repo, "", "abc", "origin", nil, ""); ok || err != nil {
		t.Fatalf("no branch: %v %v", ok, err)
	}
	if _, ok, _ := Covering(ctx, missing, repo, "main", "abc", "origin", nil, ""); ok || strings.Join(repo.remotes, ",") != "origin" {
		t.Fatalf("missing heads: %v, fetched %v", ok, repo.remotes)
	}
	repo.remotes = nil
	if _, ok, _ := Covering(ctx, missing, repo, "main", "abc", "", nil, ""); ok || len(repo.remotes) > 0 {
		t.Fatalf("no remote: %v, fetched %v", ok, repo.remotes)
	}
}
