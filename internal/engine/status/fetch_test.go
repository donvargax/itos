package status

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	vcsfake "github.com/donvargax/itos/v8/internal/vcs/port/porttest"
)

// fetches records how the remote was fetched from, and blocks a fetch until
// its time is up when it is told to.
type fetches struct {
	*vcsfake.VCS
	asked    []string
	talks    []vcsport.Talk
	deadline []time.Duration
	hang     bool
}

func (f *fetches) Fetch(ctx context.Context, remote, refspec string, talk vcsport.Talk) (string, error) {
	f.asked = append(f.asked, remote+" "+refspec)
	f.talks = append(f.talks, talk)
	if d, ok := ctx.Deadline(); ok {
		f.deadline = append(f.deadline, time.Until(d))
	}
	if f.hang {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return f.VCS.Fetch(ctx, remote, refspec, talk)
}

// The branch the status reads is fetched into its remote-tracking ref alone,
// never prompting, within 20 s.
func TestFetchesTheBranchItReads(t *testing.T) {
	v := &fetches{VCS: world()}
	cfg := configOf(t, v, configText, nil)
	st := &Standing{}
	st.readBranch(t.Context(), cfg, "origin", "refs/heads/main", "main", inOf(v, nil))
	if len(v.asked) != 1 || v.asked[0] != "origin +refs/heads/main:refs/remotes/origin/main" || !v.talks[0].NoPrompt {
		t.Errorf("fetched %q with %+v", v.asked, v.talks)
	}
	if got := v.deadline[0]; got <= 19*time.Second || got > 20*time.Second {
		t.Errorf("the fetch had %s", got)
	}
	if fetchTimeout != 20*time.Second {
		t.Errorf("the fetch's time is %s", fetchTimeout)
	}
}

// From a detached HEAD the remote's own refspecs are fetched.
func TestFetchesFromADetachedHeadWithTheRemotesRefspecs(t *testing.T) {
	v := &fetches{VCS: world()}
	cfg := configOf(t, v, configText, nil)
	st := &Standing{}
	st.readBranch(t.Context(), cfg, "origin", "HEAD", "HEAD", inOf(v, nil))
	if len(v.asked) != 1 || v.asked[0] != "origin " {
		t.Errorf("fetched %q", v.asked)
	}
}

// Nothing is fetched under a stealth config, nor where the remote answers
// without the branch.
func TestFetchesNothingWhere(t *testing.T) {
	for _, c := range []struct {
		name   string
		change func(v *fetches, cfgStealth *bool)
		line   string
	}{
		{"stealth", func(_ *fetches, stealth *bool) { *stealth = true }, ""},
		{"the remote lacks the branch", func(v *fetches, _ *bool) { delete(v.Remote["origin"].Refs, "refs/heads/main") },
			"origin has no branch main yet, so there is no CI run to read."},
	} {
		t.Run(c.name, func(t *testing.T) {
			v := &fetches{VCS: world()}
			stealth := false
			c.change(v, &stealth)
			cfg := configOf(t, v, configText, nil)
			cfg.Stealth = stealth
			st := &Standing{}
			st.readBranch(t.Context(), cfg, "origin", "refs/heads/main", "main", inOf(v, nil))
			if len(v.asked) != 0 || st.Behind != nil || st.pull != "" {
				t.Errorf("fetched %q, behind %v, told %q", v.asked, st.Behind, st.pull)
			}
			if c.line != "" && (len(st.Unread) != 1 || st.Unread[0] != c.line) {
				t.Errorf("unread %q", st.Unread)
			}
			if strings.Contains(strings.Join(st.Unread, "\n"), "cannot be fetched") {
				t.Errorf("unread %q", st.Unread)
			}
		})
	}
}

// A fetch with no answer in its time is said so, and is not waited for
// longer.
func TestFetchOutOfTime(t *testing.T) {
	defer func(was time.Duration) { fetchTimeout = was }(fetchTimeout)
	fetchTimeout = time.Millisecond
	v := &fetches{VCS: world(), hang: true}
	cfg := configOf(t, v, configText, nil)
	st := &Standing{}
	st.readBranch(t.Context(), cfg, "origin", "refs/heads/main", "main", inOf(v, nil))
	if len(st.Unread) == 0 || st.Unread[0] != "origin/main cannot be fetched: no answer within 0 s" {
		t.Errorf("unread %q", st.Unread)
	}
	if st.Behind != nil {
		t.Errorf("behind %d after no fetch", *st.Behind)
	}
}

// The reason is git's first line as git writes it; a failure with no words
// of its own gives the error's first line.
func TestWhyNot(t *testing.T) {
	expired, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()
	for _, c := range []struct {
		name    string
		expired bool
		err     error
		want    string
	}{
		{"git's first line", false, &vcsport.Failed{Command: "git fetch", Code: 128, Words: "\nfatal: no\nhint: more\n"}, "fatal: no"},
		{"no words", false, &vcsport.Failed{Command: "git fetch o", Code: 1}, "git fetch o: exit status 1:"},
		{"not git's", false, errors.New("first\nsecond"), "first"},
		{"out of time", true, errors.New("killed"), "no answer within 20 s"},
	} {
		ctx := t.Context()
		if c.expired {
			ctx = expired
		}
		if got := whyNot(ctx, c.err); got != c.want {
			t.Errorf("%s: %q, not %q", c.name, got, c.want)
		}
	}
}

// After a fetch that worked the commits of the head HEAD lacks are counted,
// said before the work when there are any: with the branch, its commits
// singular or plural, and from a detached HEAD with the advice to switch.
func TestReadBehind(t *testing.T) {
	for _, c := range []struct {
		name   string
		at     string
		ref    string
		behind int
		pull   string
	}{
		{"up to date", c3, "refs/heads/main", 0, ""},
		{"plural", c1, "refs/heads/main", 2, "main is 2 commits behind origin/main: git pull before reading the work below."},
		{"singular", c2, "refs/heads/main", 1, "main is 1 commit behind origin/main: git pull before reading the work below."},
		{"detached", c1, "HEAD", 2, "HEAD is 2 commits behind origin/HEAD: switch to a branch and pull before reading the work below."},
	} {
		t.Run(c.name, func(t *testing.T) {
			v := world()
			v.Named["HEAD"] = c.at
			cfg := configOf(t, v, configText, nil)
			st := &Standing{}
			st.readBehind(t.Context(), cfg.VCS, "origin", c.ref, strings.TrimPrefix(c.ref, "refs/heads/"), c3)
			if st.Behind == nil || *st.Behind != c.behind || st.pull != c.pull {
				t.Errorf("behind %v, told %q; want %d, %q", st.Behind, st.pull, c.behind, c.pull)
			}
		})
	}
}

// A head that cannot be counted from HEAD says nothing of being behind.
func TestReadBehindUncounted(t *testing.T) {
	v := world()
	cfg := configOf(t, v, configText, nil)
	st := &Standing{}
	st.readBehind(t.Context(), cfg.VCS, "origin", "refs/heads/main", "main", c3)
	if st.Behind != nil || st.pull != "" {
		t.Errorf("behind %v, told %q", st.Behind, st.pull)
	}
}

// The pull line comes before the work, "behind" is in --json only after a
// fetch that worked, 0 when up to date.
func TestPullLineAndBehindField(t *testing.T) {
	behind := 2
	st := &Standing{Behind: &behind, pull: "main is 2 commits behind origin/main: git pull before reading the work below.",
		Doing: []any{}, Next: []any{}}
	var text bytes.Buffer
	st.Print(&text)
	if !strings.Contains(text.String(), "\nmain is 2 commits behind origin/main: git pull before reading the work below.\n\nWorking for nobody") {
		t.Errorf("the status:\n%s", text.String())
	}
	fields := map[string]any{}
	for _, f := range st.Fields() {
		fields[f.Key] = f.Value
	}
	if fields["behind"] != 2 {
		t.Errorf("behind %v", fields["behind"])
	}
	st.Behind = nil
	for _, f := range st.Fields() {
		if f.Key == "behind" {
			t.Errorf("behind %v after no fetch", f.Value)
		}
	}
}
