package cli

// The commands that write the work registry (slice 52, features/work.feature):
// work take and work promote. The registry is written by commands, never by
// hand, and each commits its own change (the user's calls, 2026-10-03), so a
// coordinator neither edits YAML nor words a registry commit. The edit is
// work's (internal/work/write.go); what every such command shares is here:
// a sound registry before anything is written, and writeRegistry, which
// writes the text and commits the registry alone, through the hooks as any
// commit, whatever else the person has staged (git commit --only), with the
// header and body the change gives. Under a stealth config the registry is
// in the git folder, tracked by nothing, so it is written and nothing is
// committed.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/git"
	"github.com/donvargax/itos/v2/internal/ledger"
	"github.com/donvargax/itos/v2/internal/out"
	"github.com/donvargax/itos/v2/internal/providers"
	"github.com/donvargax/itos/v2/internal/work"
)

// bodyWidth is the width a registry commit's body is wrapped at: the header
// lint's body-max-line-length.
const bodyWidth = 100

// workArgs reads a registry command's arguments: one id, and the valued
// flags it takes, each given at most once, "--flag value" or "--flag=value".
// Anything else is a usage error.
func workArgs(sub string, args []string, flags ...string) (string, map[string]string, error) {
	values := map[string]string{}
	var ids []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			ids = append(ids, arg)
			continue
		}
		name, value, joined := strings.Cut(arg, "=")
		known := false
		for _, f := range flags {
			known = known || f == name
		}
		if !known {
			return "", nil, usage("work %s does not take %s", sub, name)
		}
		if !joined {
			if i+1 >= len(args) {
				return "", nil, usage("work %s %s needs a value", sub, name)
			}
			i++
			value = args[i]
		}
		if _, twice := values[name]; twice || value == "" {
			return "", nil, usage("work %s takes one %s with a value", sub, name)
		}
		values[name] = value
	}
	if len(ids) != 1 {
		return "", nil, usage("work %s needs one <id>", sub)
	}
	return ids[0], values, nil
}

// refuseWork reports a registry command's problems and exits with code: on
// stderr, or under --json as {"ok": false, "problems": […]} on stdout.
func refuseWork(found []out.Problem, code int, o Out) (int, error) {
	if o.JSON {
		return code, out.Emit(o.Stdout, out.Field{Key: "ok", Value: false}, out.Field{Key: "problems", Value: found})
	}
	for _, p := range found {
		fmt.Fprintln(o.Stderr, p.Message)
		if p.Fix != "" {
			fmt.Fprintf(o.Stderr, "  fix: %s\n", p.Fix)
		}
	}
	return code, nil
}

// soundRegistry is the config, the registry and its text, when the registry
// is there and sound: a command writes only a registry work check passes.
// Otherwise its problems are reported, exit 1, and the config is nil.
func soundRegistry(o Out) (*config.Loaded, work.Registry, string, int, error) {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return nil, work.Registry{}, "", 0, err
	}
	found, err := work.Problems(cfg)
	if err != nil {
		return nil, work.Registry{}, "", 0, err
	}
	if len(found) > 0 {
		code, err := refuseWork(found, ExitPolicy, o)
		return nil, work.Registry{}, "", code, err
	}
	file := cfg.Work.Registry
	text, err := os.ReadFile(file)
	if err != nil {
		return nil, work.Registry{}, "", 0, err
	}
	registry, err := work.Load(cfg, file)
	if err != nil {
		return nil, work.Registry{}, "", 0, err
	}
	return cfg, registry, string(text), 0, nil
}

// uncommitted is the problem of a registry git does not hold as HEAD has it
// (untracked, or changed, staged or not), which a commit of it would sweep
// in beside the command's own change; nil when it is clean.
func uncommitted(file string) *out.Problem {
	tracked := git.Succeeds("ls-files", "--error-unmatch", "--", file)
	status, err := git.Output("status", "--porcelain", "--", file)
	if tracked && err == nil && strings.TrimSpace(status) == "" {
		return nil
	}
	return &out.Problem{
		Rule:    "work-registry-uncommitted",
		Message: file + " has changes no commit holds, which the registry's commit would sweep in",
		Fix:     "commit or drop the changes to " + file + " first",
	}
}

// writeRegistry writes the change to the registry and, in a project,
// commits the registry alone: git commit --only, with the change's header
// and body, through the hooks, what else is staged left staged. Its result
// is the new commit's SHA, "" under a stealth config, which commits nothing.
// A registry with changes no commit holds is refused before anything is
// written (exit 1); a commit that fails (a hook refusing it) puts the
// registry back as it was, file and index, and exits 1.
func writeRegistry(cfg *config.Loaded, old string, change work.Change, o Out) (string, int, error) {
	file := cfg.Work.Registry
	if !cfg.Stealth {
		if p := uncommitted(file); p != nil {
			code, err := refuseWork([]out.Problem{*p}, ExitPolicy, o)
			return "", code, err
		}
	}
	mode := fs.FileMode(0o644)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(file, []byte(change.Text), mode); err != nil {
		return "", 0, err
	}
	if cfg.Stealth {
		return "", 0, nil
	}
	argv := []string{"commit", "--only", "-m", change.Header, "-m", work.Wrap(change.Body, bodyWidth)}
	if o.Quiet {
		argv = append(argv, "--quiet")
	}
	argv = append(argv, "--", file)
	cmd := exec.Command(git.Bin(), argv...)
	cmd.Env = append(withoutFooters(os.Environ()), AmendEnv+"=0")
	// stdout is the command's own result; git's and the hooks' words go to
	// stderr.
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, o.Stderr, o.Stderr
	runErr := cmd.Run()
	var exit *exec.ExitError
	if runErr != nil && !errors.As(runErr, &exit) {
		return "", 0, fmt.Errorf("cannot run git: %w", runErr)
	}
	if runErr != nil {
		restoreErr := os.WriteFile(file, []byte(old), mode)
		git.Succeeds("reset", "-q", "--", file)
		if restoreErr != nil {
			return "", 0, fmt.Errorf("the commit failed, and %s cannot be put back: %w", file, restoreErr)
		}
		code, err := refuseWork([]out.Problem{{
			Rule:    "work-commit-failed",
			Message: fmt.Sprintf("the commit of %s failed (git exited %d), so %s is as it was", file, exit.ExitCode(), file),
			Fix:     "fix what git or the hook above says, then run the command again",
		}}, ExitPolicy, o)
		return "", code, err
	}
	sha, err := git.Output("rev-parse", "HEAD")
	if err != nil {
		return "", 0, err
	}
	return strings.TrimSpace(sha), 0, nil
}

// committed is how a registry command's text output ends: the commit it
// made, or why there is none.
func committed(sha, header string) string {
	if sha == "" {
		return "the registry is the stealth config's, so nothing is committed"
	}
	return "committed " + sha[:min(7, len(sha))] + " " + header
}

// reportWork prints a registry command's result: the line, unless -q; under
// --json, ok, the item, the fields given and the commit (null when none).
func reportWork(line string, change work.Change, sha string, extra []out.Field, o Out) (int, error) {
	if o.JSON {
		var commit any
		if sha != "" {
			commit = sha
		}
		fields := append([]out.Field{{Key: "ok", Value: true}, {Key: "item", Value: change.Item}}, extra...)
		return 0, out.Emit(o.Stdout, append(fields, out.Field{Key: "commit", Value: commit})...)
	}
	if !o.Quiet {
		fmt.Fprintln(o.Stdout, line)
	}
	return 0, nil
}

// workTake is `work take <id> [--as <handle>]`: the item set in progress for
// the person and the registry committed (work.Take). The person is --as,
// else who the identity provider says, as for work; one that is nobody, or
// not among the people, exits 3. Under a stealth config with no --as nobody
// is asked: every item is the session's, and its owner is left as it is.
func workTake(args []string, o Out) (int, error) {
	id, flags, err := workArgs("take", args, "--as")
	if err != nil {
		return 0, err
	}
	cfg, registry, text, code, err := soundRegistry(o)
	if cfg == nil {
		return code, err
	}
	as := flags["--as"]
	every := cfg.Stealth && as == ""
	person := ""
	if !every {
		listedIn := cfg.Work.People.File
		who := work.Whoami(registry, listedIn, as, providers.IdentityProvider(cfg, o.Stderr))
		switch {
		case who.Problem != "":
			return refuseWork([]out.Problem{{Rule: "work-no-person", Message: who.Problem, Fix: "pass --as <handle>"}}, ExitMissing, o)
		case !who.Listed:
			return refuseWork([]out.Problem{{
				Rule:    "work-no-person",
				Message: fmt.Sprintf("%s is not in %s, so cannot own an item", who.Handle, listedIn),
				Fix:     "add " + who.Handle + " to " + listedIn + ", or pass --as <handle>",
			}}, ExitMissing, o)
		}
		person = who.Handle
	}
	change, problem, err := work.Take(registry, text, id, person, every)
	if err != nil {
		return 0, fmt.Errorf("%s cannot be edited in place: %w", cfg.Work.Registry, err)
	}
	if problem != nil {
		return refuseWork([]out.Problem{*problem}, ExitPolicy, o)
	}
	forWhom := ""
	if owner, ok := change.Item.At("owner").(string); ok && owner != "" {
		forWhom = " for " + owner
	}
	if change.Unchanged {
		return reportWork(id+" is already in progress"+forWhom+"; nothing to change", change, "", nil, o)
	}
	sha, code, err := writeRegistry(cfg, text, change, o)
	if err != nil || code != 0 {
		return code, err
	}
	return reportWork(fmt.Sprintf("%s is in progress%s: %s", id, forWhom, committed(sha, change.Header)), change, sha, nil, o)
}

// promoteKinds are what work promote makes of an idea.
var promoteKinds = []string{"slice", "task"}

// workPromote is `work promote <idea> --as <id> --kind slice|task`: the idea
// renamed, given the kind, "Was <idea>." before its why, every depends_on
// naming it renamed too, and the registry committed (work.Promote). A task's
// id must match the ledger's ledger.id.
func workPromote(args []string, o Out) (int, error) {
	id, flags, err := workArgs("promote", args, "--as", "--kind")
	if err != nil {
		return 0, err
	}
	newID, kind := flags["--as"], flags["--kind"]
	if newID == "" {
		return 0, usage("work promote needs --as <id>, the slice's or the task's id")
	}
	if kind != "slice" && kind != "task" {
		return 0, usage("work promote needs --kind %s", strings.Join(promoteKinds, "|"))
	}
	cfg, registry, text, code, err := soundRegistry(o)
	if cfg == nil {
		return code, err
	}
	change, problem, err := work.Promote(registry, text, id, newID, kind, ledger.IDPattern(cfg))
	if err != nil {
		return 0, fmt.Errorf("%s cannot be edited in place: %w", cfg.Work.Registry, err)
	}
	if problem != nil {
		return refuseWork([]out.Problem{*problem}, ExitPolicy, o)
	}
	sha, code, err := writeRegistry(cfg, text, change, o)
	if err != nil || code != 0 {
		return code, err
	}
	line := fmt.Sprintf("%s is the %s %s", id, kind, newID)
	if len(change.Rewritten) > 0 {
		line += ", named now by " + strings.Join(change.Rewritten, ", ")
	}
	rewritten := make([]any, len(change.Rewritten))
	for i, r := range change.Rewritten {
		rewritten[i] = r
	}
	extra := []out.Field{{Key: "was", Value: id}, {Key: "rewritten", Value: rewritten}}
	return reportWork(line+": "+committed(sha, change.Header), change, sha, extra, o)
}
