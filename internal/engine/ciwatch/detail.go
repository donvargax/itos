package ciwatch

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/providers"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
)

// Detail is a run that completed with what a reader would otherwise look for
// by hand (slice 172, features/watch-failures.feature): the tags the remote
// has at its head commit, an annotated tag by the commit it points to, and
// for each job that did not succeed the step it failed at and what its log
// names. A log is asked for once per failed job, never retried, and not at
// all for a run that succeeded; one that cannot be read leaves the job's step
// and says so, and never changes the watch's result.
func Detail(ctx context.Context, wr providers.Watcher, v vcsport.VCS, remote string, run providers.Run) providers.Run {
	run.Tags = tagsAt(ctx, v, remote, run.HeadSHA)
	if run.Conclusion == "success" {
		return run
	}
	run.Jobs = slices.Clone(run.Jobs)
	for i, j := range run.Jobs {
		if failedJob(j) {
			run.Jobs[i] = explain(wr.Log, j)
		}
	}
	return run
}

// explain is the failed job with its step and, when log gives it, what the
// log names.
func explain(log providers.JobLog, j providers.Job) providers.Job {
	j.Step = j.FailedStep()
	if log == nil {
		return j
	}
	text, err := log(j.ID)
	if err != nil {
		j.Log = providers.LogUnreadable
		return j
	}
	j.Log = providers.LogRead
	j.Failures, j.MoreFailures = readLog(text)
	return j
}

// tagsAt are the remote's tags at the commit, in byte order: those whose
// commit it is, an annotated tag by what it points to, which ls-remote gives
// beside the tag object as the ref with ^{}. None when the remote cannot be
// read.
func tagsAt(ctx context.Context, v vcsport.VCS, remote, sha string) []string {
	ctx, cancel := context.WithTimeout(ctx, FetchTimeout)
	defer cancel()
	refs, err := v.RemoteRefs(ctx, remote, []string{"refs/tags/*"}, vcsport.Talk{NoPrompt: true})
	if err != nil {
		return nil
	}
	at := map[string]string{}
	for _, peeled := range []bool{false, true} {
		for ref, object := range refs {
			name, isPeeled := strings.CutSuffix(ref, "^{}")
			if tag, ok := strings.CutPrefix(name, "refs/tags/"); ok && isPeeled == peeled {
				at[tag] = object
			}
		}
	}
	var tags []string
	for tag, object := range at {
		if object == sha {
			tags = append(tags, tag)
		}
	}
	slices.Sort(tags)
	return tags
}

// describe says in words what the run's jobs name, after the run's end line:
// each job that failed, its step, what its log names, each failure with its
// first lines, and what was left out.
func describe(w io.Writer, run providers.Run) {
	for _, j := range run.Jobs {
		if j.Step == "" && j.Log == "" {
			continue
		}
		head := j.Name + ": failed"
		if j.Step != "" {
			head += fmt.Sprintf(" at the step %q", j.Step)
		}
		if j.Log == providers.LogUnreadable {
			head += "; its log could not be read"
		}
		fmt.Fprintln(w, head)
		for _, f := range j.Failures {
			fmt.Fprintln(w, "  "+heading(f))
			if f.Kind == "tail" && f.MoreLines > 0 {
				fmt.Fprintf(w, "    ... %d more lines before these\n", f.MoreLines)
			}
			for _, l := range f.Lines {
				fmt.Fprintln(w, "    "+l)
			}
			if f.Kind != "tail" && f.MoreLines > 0 {
				fmt.Fprintf(w, "    ... %d more lines\n", f.MoreLines)
			}
		}
		if j.MoreFailures > 0 {
			fmt.Fprintf(w, "  ... %d more failures\n", j.MoreFailures)
		}
	}
}

// heading is the line that introduces a failure: its kind and name, a Go
// test's package after them, the last lines of the log for the tail, which
// has no name.
func heading(f providers.Failure) string {
	switch {
	case f.Kind == "tail":
		return "the last lines of the log:"
	case f.Package != "":
		return fmt.Sprintf("%s %s (%s):", f.Kind, f.Name, f.Package)
	}
	return fmt.Sprintf("%s %s:", f.Kind, f.Name)
}
