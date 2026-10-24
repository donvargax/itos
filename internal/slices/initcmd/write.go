package initcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/release"
	"github.com/donvargax/itos/v8/internal/tests"
	"github.com/donvargax/itos/v8/internal/value"
)

// write writes the starter, pins the newest release, installs the hooks
// and makes the plugin's offer, the git shim's and that of the rules for
// agents; its exit code is hook install's, else 1 when claude failed at
// installing the plugin asked for, the git shim asked for could not be
// linked or the rules asked for could not be written.
func (h Handler) write(ctx context.Context, stealth, initialized bool, offer Offer, log io.Writer, o Out) (int, error) {
	file := "itos.yaml"
	if stealth {
		l, err := h.ports.Locate(ctx, "")
		if err != nil {
			return 0, err
		}
		file = filepath.Join(l.Common, config.StealthFolder, "itos.yaml")
	}
	beside := func(p string) string {
		if stealth {
			return filepath.Join(filepath.Dir(file), p)
		}
		return p
	}
	since := ""
	if head, err := h.ports.Resolve(ctx, "HEAD"); err == nil {
		since = head
	}
	s := starter{stealth: stealth, since: since, scenarios: hasFeatureFiles(h.path(ctx, starterFeatures))}
	version, sums, pinErr := h.ports.Release(ctx, "")
	if pinErr == nil {
		s.pin = &[2]string{version, release.SHA256(sums)}
	}

	var files []writtenFile
	write := func(p, text string) error {
		action := "kept"
		if !exists(h.path(ctx, p)) {
			if err := os.MkdirAll(filepath.Dir(h.path(ctx, p)), 0o777); err != nil {
				return named(err, filepath.Dir(p))
			}
			if err := h.ports.WriteFile(ctx, p, []byte(text), 0o666); err != nil {
				return err
			}
			action = "wrote"
		}
		files = append(files, writtenFile{filepath.ToSlash(p), action})
		return nil
	}
	// The scenarios are listed by the config about to be written, or the one
	// there, before anything is written. With none carrying an ID tag the
	// footer is required of no type, and the config written says so.
	smoke, smokeIDs := "", 0
	if s.scenarios {
		cfg, err := h.fromText(ctx, file, s.config(), stealth)
		if exists(h.path(ctx, file)) {
			cfg, err = h.loadConfigFile(ctx, file)
		}
		if err != nil {
			return 0, err
		}
		list, err := tests.ListTests(cfg, "scenario", "worktree", nil)
		if err != nil {
			return 0, err
		}
		s.untagged = len(list.Tests) == 0 && !exists(h.path(ctx, file))
		smoke, smokeIDs = starterSmokeSet(list)
	}
	if err := write(file, s.config()); err != nil {
		return 0, err
	}
	if err := write(beside(starterLedger), s.ledger()); err != nil {
		return 0, err
	}
	if err := write(beside(starterRegistry), starterRegistryText); err != nil {
		return 0, err
	}
	if s.scenarios {
		if err := write(beside(starterSmoke), smoke); err != nil {
			return 0, err
		}
	}

	if !o.JSON {
		say := func(format string, a ...any) { fmt.Fprintf(log, format+"\n", a...) }
		for _, f := range files {
			say("%s %s", f.Action, f.Path)
		}
		switch {
		case since != "":
			say("commits.since is HEAD, %s: no commit before it is judged.", since[:7])
		default:
			say("The repository has no commit, so the config has no commits.since: every commit is judged.")
		}
		switch {
		case s.untagged:
			say("No scenario under %s carries an ID tag, so the smoke set names none and no feat or fix needs a "+
				"Scenarios footer yet: tag a scenario with its ID, such as @ID-PAGE-01 on the line above it, then "+
				"set commits.footers.Scenarios.required_for to [feat, fix] in %s.", starterFeatures, filepath.ToSlash(file))
		case s.scenarios:
			say("The smoke set names %s, one of each feature file with a live one; "+
				"a feat or a fix names the ones it turns green (itos commit --scenarios).", count(smokeIDs, "scenario"))
		}
		if pinErr == nil {
			say("Pinned itos %s, the newest release.", version)
		} else {
			say("Pinned no itos: %s. Run itos pin when the release server can be reached.", pinErr)
		}
	}

	hooksCode, hooks, err := h.installHooks(ctx, o)
	if err != nil {
		return 0, err
	}
	offered := h.offers(ctx, offer, log, o)
	code := hooksCode
	if code == 0 {
		code = offered.Code
	}
	if o.JSON {
		var at, pin any
		if since != "" {
			at = since
		}
		fields := []out.Field{{Key: "config", Value: filepath.ToSlash(file)}, {Key: "action", Value: "initialized"},
			{Key: "git_init", Value: initialized}, {Key: "since", Value: at}, {Key: "files", Value: files}}
		if s.pin != nil {
			pin = map[string]string{"version": s.pin[0], "checksums": s.pin[1]}
		}
		fields = append(fields, out.Field{Key: "pin", Value: pin})
		if pinErr != nil {
			fields = append(fields, out.Field{Key: "pin_problem", Value: pinErr.Error()})
		}
		fields = append(fields, out.Field{Key: "hooks", Value: hooks}, out.Field{Key: "plugin", Value: offered.Plugin},
			out.Field{Key: "git_shim", Value: offered.Shim}, out.Field{Key: "agent_rules", Value: offered.Rules})
		return code, out.Emit(o.Stdout, fields...)
	}
	if hooksCode != 0 {
		return hooksCode, nil
	}
	if stealth {
		fmt.Fprintln(log, "Nothing the project tracks changed: commit as ever, with itos commit, and push with itos push.")
	} else {
		var paths []string
		task := "<a task of the ledger>"
		for _, f := range files {
			if f.Action == "wrote" {
				paths = append(paths, f.Path)
				if f.Path == starterLedger {
					task = "T-1"
				}
			}
		}
		// The plugin installed for the project is declared in its settings,
		// which the commit carries.
		if offered.ProjectPlugin {
			if status, _ := h.ports.PathStatus(ctx, projectSettings); len(status) > 0 {
				paths = append(paths, projectSettings)
			}
		}
		paths = append(paths, offered.RulesWritten...)
		fmt.Fprintf(log, "Commit what init wrote with the task that adopts itos: git add %s, then itos commit --task %s -m 'chore: adopt itos'\n",
			strings.Join(paths, " "), task)
	}
	return code, nil
}

// offers makes the three offers on a first run, saying what they do on log,
// or nothing under --json.
func (h Handler) offers(ctx context.Context, offer Offer, log io.Writer, o Out) Offered {
	if o.JSON {
		log = io.Discard
	}
	return h.makeOffers(ctx, offer, log)
}

// installHooks runs hook install on the config just written; under --json
// its object, without its schema, for init's own.
func (h Handler) installHooks(ctx context.Context, o Out) (int, any, error) {
	if !o.JSON {
		code, err := h.ports.InstallHooks(ctx, o.Stdout, false)
		return code, nil, err
	}
	var buf bytes.Buffer
	code, err := h.ports.InstallHooks(ctx, &buf, true)
	if err != nil || buf.Len() == 0 {
		return code, nil, err
	}
	// Its keys in the order it wrote them, its schema left out (out.Emit
	// writes it first).
	hooks := json.RawMessage(bytes.Replace(buf.Bytes(), []byte(`"schema": 1,`), nil, 1))
	if !json.Valid(hooks) {
		return 0, nil, fmt.Errorf("hook install wrote no JSON object: %s", buf.Bytes())
	}
	return code, hooks, nil
}

// hasFeatureFiles is whether a folder holds a feature file, at any depth.
func hasFeatureFiles(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".feature") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// starterSmokeSet is a smoke set naming each feature file's first live test, and
// how many it names.
func starterSmokeSet(list tests.List) (string, int) { return smokeSetOf(list, "@") }

// smokeSetOf is a smoke set naming each file's first live test, its ID
// written after the kind's tag prefix, and how many it names.
func smokeSetOf(list tests.List, prefix string) (string, int) {
	var b strings.Builder
	b.WriteString(starterSmokeHeader)
	seen := map[string]bool{}
	n := 0
	for _, t := range list.Tests {
		if !t.Live || seen[t.File] {
			continue
		}
		seen[t.File] = true
		n++
		fmt.Fprintf(&b, "- file: %s\n  scenarios:\n    - id: %s\n      why: picked by itos init, the file's first live scenario\n",
			t.File, value.Quote(prefix+t.ID))
	}
	if n == 0 {
		b.WriteString("[]\n")
	}
	return b.String(), n
}

// count is a number of things, the noun in the plural unless there is one.
func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
