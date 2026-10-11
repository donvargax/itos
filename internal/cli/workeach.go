package cli

// Several ids (slice 118, features/work.feature): work edit, drop, defer,
// resume and queue take one or more ids. One id runs as it always has; with
// several the change is made to each by internal/work's *Each writers,
// every id judged before anything is written, refused whole (exit 1) naming
// every id refused and why, and committed once, the registry alone, through
// writeRegistry. A change that is one item's own, work edit --title, is a
// usage error with several ids, as is an id named twice.

import (
	"fmt"
	"slices"
	"strings"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/out"
	"github.com/donvargax/itos/v7/internal/value"
	"github.com/donvargax/itos/v7/internal/work"
)

// workIDs reads the arguments of a registry command that takes one or more
// ids: the ids, in the order given, each once, and the flags as workArgsEmpty
// reads them.
func workIDs(sub string, args []string, empty []string, flags ...string) ([]string, map[string]string, error) {
	ids, values, err := workArgsParsed(sub, args, empty, flags...)
	if err != nil {
		return nil, nil, err
	}
	if len(ids) == 0 {
		return nil, nil, usage("work %s needs one <id> or more", sub)
	}
	for i, id := range ids {
		if slices.Contains(ids[:i], id) {
			return nil, nil, usage("work %s names %s twice", sub, id)
		}
	}
	return ids, values, nil
}

// eachWrite is a registry command's change of several ids: what
// internal/work's *Each writers give.
type eachWrite func(cfg *config.Loaded, r work.Registry, text string) (work.Several, []string, []out.Problem, error)

// writeEach makes a change of several ids and reports it: the registry
// sound, the change judged for every id, refused whole when any id is, then
// written and committed once (writeRegistry). says gives a line for each
// item, the report's last line the commit; under --json, ok, the items in
// the order given, the extra fields and the commit.
func writeEach(o Out, sub string, write eachWrite, says func(work.Several) []string, extra func(work.Several) []out.Field) (int, error) {
	cfg, registry, text, release, code, err := soundRegistry(o)
	defer release()
	if cfg == nil {
		return code, err
	}
	s, refused, found, err := write(cfg, registry, text)
	if err != nil {
		return 0, uneditable(cfg.Work.Registry, err)
	}
	if len(found) > 0 {
		return refuseEach(sub, refused, found, o)
	}
	sha := ""
	if !s.Unchanged {
		sha, code, err = writeRegistry(cfg, text, work.Change{Text: s.Text, Header: s.Header, Body: s.Body}, o)
		if err != nil || code != 0 {
			return code, err
		}
	}
	if o.JSON {
		items := make([]any, len(s.Items))
		for i, item := range s.Items {
			items[i] = item
		}
		var commit any
		if sha != "" {
			commit = sha
		}
		fields := []out.Field{{Key: "ok", Value: true}, {Key: "items", Value: items}}
		if extra != nil {
			fields = append(fields, extra(s)...)
		}
		return 0, out.Emit(o.Stdout, append(fields, out.Field{Key: "commit", Value: commit})...)
	}
	if o.Quiet {
		return 0, nil
	}
	lines := says(s)
	if s.Unchanged {
		lines = append(lines, "nothing to change")
	} else {
		lines = append(lines, committed(sha, s.Header))
	}
	fmt.Fprintln(o.Stdout, strings.Join(lines, "\n"))
	return 0, nil
}

// refuseEach reports a change of several ids refused, exit 1: a line naming
// every id refused, then each problem, as refuseWork reports them; under
// --json, {"ok": false, "refused": […], "problems": […]}.
func refuseEach(sub string, refused []string, found []out.Problem, o Out) (int, error) {
	if o.JSON {
		ids := make([]any, len(refused))
		for i, id := range refused {
			ids[i] = id
		}
		return ExitPolicy, out.Emit(o.Stdout, out.Field{Key: "ok", Value: false}, out.Field{Key: "refused", Value: ids},
			out.Field{Key: "problems", Value: found})
	}
	fmt.Fprintf(o.Stderr, "work %s refuses %s, so nothing is written:\n", sub, and(refused))
	return refuseWork(found, ExitPolicy, o)
}

// eachLine is a line for each item of a change of several ids: done, when
// its change wrote something, else unchanged.
func eachLine(done func(id string, c work.Change) string, unchanged string) func(work.Several) []string {
	return func(s work.Several) []string {
		lines := make([]string, len(s.Each))
		for i, c := range s.Each {
			id := value.String(c.Item.At("id"))
			lines[i] = id + " " + unchanged
			if !c.Unchanged {
				lines[i] = done(id, c)
			}
		}
		return lines
	}
}

// itemIs is the line of an item a change of several ids changed: "<id> is
// <what>".
func itemIs(what string) func(string, work.Change) string {
	return func(id string, _ work.Change) string { return id + " is " + what }
}
