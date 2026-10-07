package cli

// itos draft (slice 96, features/draft.feature): the coordinator's pending
// changes, kept while an agent holds the checkout and promoted once no work
// is going on. The list is drafts.yaml in the drafts folder of itos's folder
// under the git common dir (internal/draft), each change's patch beside it;
// add, drop and promote hold the list's lock from before they read it to
// after they write it (heldDrafts, as itos followup's heldThreads, bug 16).
//
// A change is drafted against HEAD through a scratch index: HEAD read into
// it, the paths added to it as the working tree has them (new files too),
// and the difference written as a binary git patch; then those paths, and
// only those, are put back as HEAD has them, a file HEAD lacks removed.
// promote applies each patch with git apply --index, which refuses the
// whole patch when any of it does not match the tree as it is (the change
// no longer applies), and commits it with its message through the hooks,
// as any commit; a command draft is run by this same itos binary, its
// arguments as kept, and commits itself.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/draft"
	"github.com/donvargax/itos/v6/internal/git"
	"github.com/donvargax/itos/v6/internal/lock"
	"github.com/donvargax/itos/v6/internal/out"
	"github.com/donvargax/itos/v6/internal/value"
	"github.com/donvargax/itos/v6/internal/work"
)

// draftTakes is what draft takes, as a usage error names it.
const draftTakes = "it takes add, promote or drop, else nothing"

// draftCommand is `draft`, `draft add`, `draft promote` and `draft drop`.
func draftCommand(args []string, o Out) (int, error) {
	sub, rest := split(args)
	switch sub {
	case "add":
		return draftAdd(rest, o)
	case "promote":
		return draftPromote(rest, o)
	case "drop":
		return draftDrop(rest, o)
	case "":
		return draftList(o)
	}
	return 0, usage("draft has no subcommand %s: %s", sub, draftTakes)
}

// draftsFile is where the drafts are: drafts.yaml in the drafts folder of
// itos's folder of the git common dir, "" outside a git repository.
func draftsFile() string {
	common, err := git.Output("rev-parse", "--path-format=absolute", "--git-common-dir")
	if common = strings.TrimSpace(common); err != nil || common == "" {
		return ""
	}
	return filepath.Join(common, config.StealthFolder, draft.Folder, draft.FileName)
}

// noRepository says draft needs a git repository, exit 3.
func noRepository(o Out) (int, error) {
	fmt.Fprintln(o.Stderr, "itos: draft keeps its drafts in the git folder, and this is no git repository")
	return ExitMissing, nil
}

// heldDrafts is the drafts' file and its drafts, read under the file's lock,
// which release, never nil and deferred by the caller, gives back. The file
// is "" outside a git repository, where code is 3.
func heldDrafts(o Out) (file string, drafts draft.File, release func(), code int, err error) {
	release = func() {}
	if file = draftsFile(); file == "" {
		code, err = noRepository(o)
		return "", drafts, release, code, err
	}
	held, err := lock.Hold(file)
	if err != nil {
		return "", drafts, release, 0, err
	}
	release = func() {
		if err := held.Release(); err != nil {
			fmt.Fprintf(o.Stderr, "itos: the lock cannot be given back: %s\n", err)
		}
	}
	drafts, err = draft.Load(file)
	return file, drafts, release, 0, err
}

// draftEntry is a draft as draft --json and status --json list it.
type draftEntry struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Header  string   `json:"header,omitempty"`
	Paths   []string `json:"paths,omitempty"`
	Command []string `json:"command,omitempty"`
}

func entryOfDraft(d draft.Draft) draftEntry {
	if d.IsChange() {
		return draftEntry{ID: d.ID, Kind: "change", Header: d.Header(), Paths: d.Paths}
	}
	return draftEntry{ID: d.ID, Kind: "command", Command: d.Command}
}

// draftSummary is what a list says of a draft: a change's header and its
// paths, or a command's line.
func draftSummary(d draft.Draft) string {
	if d.IsChange() {
		return d.Header() + "  (" + strings.Join(d.Paths, ", ") + ")"
	}
	return d.Line()
}

// draftLines are the drafts as lists print them, indented, ids aligned.
func draftLines(drafts []draft.Draft) []string {
	width := 0
	for _, d := range drafts {
		width = max(width, len(d.ID))
	}
	lines := make([]string, len(drafts))
	for i, d := range drafts {
		lines[i] = fmt.Sprintf("  %-*s  %s", width, d.ID, draftSummary(d))
	}
	return lines
}

// draftList is `draft`: the drafts in the order they were added, which is
// the order promote applies them in.
func draftList(o Out) (int, error) {
	file := draftsFile()
	if file == "" {
		return noRepository(o)
	}
	drafts, err := draft.Load(file)
	if err != nil {
		return 0, err
	}
	if o.JSON {
		entries := []draftEntry{}
		for _, d := range drafts.Drafts {
			entries = append(entries, entryOfDraft(d))
		}
		return 0, out.Emit(o.Stdout, out.Field{Key: "drafts", Value: entries})
	}
	if len(drafts.Drafts) == 0 {
		fmt.Fprintln(o.Stdout, "No drafts.")
		return 0, nil
	}
	fmt.Fprintln(o.Stdout, "Drafts, in the order itos draft promote applies them:")
	for _, line := range draftLines(drafts.Drafts) {
		fmt.Fprintln(o.Stdout, line)
	}
	return 0, nil
}

// draftAdd is `draft add <id> -m <message> <path>…`, a change, or
// `draft add <id> -- <itos args>…`, a command line.
func draftAdd(args []string, o Out) (int, error) {
	var pos, command []string
	message, messages := "", 0
	isCommand := false
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--":
			isCommand, command = true, args[i+1:]
			i = len(args)
		case arg == "-m" || arg == "--message":
			if i+1 >= len(args) {
				return 0, usage("draft add %s needs a value", arg)
			}
			i++
			message, messages = args[i], messages+1
		default:
			pos = append(pos, arg)
		}
	}
	if len(pos) == 0 {
		return 0, usage("draft add needs <id> -m <message> <path>…, or <id> -- <itos args>…")
	}
	id, paths := pos[0], pos[1:]
	if !draft.ValidID(id) {
		return 0, usage("draft add: %q is no draft id (letters, digits, '.', '_' and '-', a letter or digit first)", id)
	}
	d := draft.Draft{ID: id}
	switch {
	case messages > 1:
		return 0, usage("draft add takes one -m <message>")
	case isCommand && (messages > 0 || len(paths) > 0):
		return 0, usage("draft add takes -m <message> <path>… or -- <itos args>…, not both")
	case isCommand && len(command) == 0:
		return 0, usage("draft add %s -- needs the itos arguments to run", id)
	case isCommand && command[0] == "draft":
		return 0, usage("draft add: a draft cannot run itos draft")
	case isCommand:
		d.Command = command
	case strings.TrimSpace(message) == "" || len(paths) == 0:
		return 0, usage("draft add %s needs -m <message> and the paths whose change it keeps", id)
	default:
		d.Message = message
	}
	file, drafts, release, code, err := heldDrafts(o)
	defer release()
	if file == "" || err != nil {
		return code, err
	}
	if had := drafts.Find(id); had != nil {
		return refuseWork([]out.Problem{{
			Rule:    "draft-id-taken",
			Message: fmt.Sprintf("%s is already a draft: %s", id, draftSummary(*had)),
			Fix:     "another id adds a new one; itos draft drop " + id + " drops the one there",
		}}, ExitPolicy, o)
	}
	var names []string
	if d.IsChange() {
		var refused *out.Problem
		d.Paths, names, refused, err = takeChange(file, id, paths)
		if err != nil {
			return 0, err
		}
		if refused != nil {
			return refuseWork([]out.Problem{*refused}, ExitPolicy, o)
		}
	}
	drafts.Drafts = append(drafts.Drafts, d)
	if err := draft.Save(file, drafts); err != nil {
		os.Remove(draft.Patch(file, id))
		return 0, err
	}
	if d.IsChange() {
		if err := putBack(names); err != nil {
			return 0, fmt.Errorf("%s is drafted, but its files cannot be put back as HEAD has them: %w", id, err)
		}
	}
	if o.JSON {
		return 0, out.Emit(o.Stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "draft", Value: entryOfDraft(d)})
	}
	if !o.Quiet {
		fmt.Fprintf(o.Stdout, "%s drafted: %s\n", id, draftSummary(d))
	}
	return 0, nil
}

// draftGitEnv is the environment draft's git commands run in: pathspecs
// read literally, so a path is the file it names and never a pattern.
func draftGitEnv(more ...string) []string {
	return append(append(os.Environ(), "GIT_LITERAL_PATHSPECS=1"), more...)
}

// draftGit runs git in env, giving its stdout; its error says what git said
// on stderr.
func draftGit(env []string, args ...string) (string, error) {
	var stdout, stderr strings.Builder
	cmd := exec.Command(git.Bin(), args...)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return stdout.String(), fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	return stdout.String(), nil
}

// topPaths are the typed paths as the repository's top names them, with
// itos at the top; a path outside the work tree is a usage error.
func topPaths(typedPaths []string) ([]string, error) {
	top, err := git.Output("rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("the work tree's top cannot be read: %w", err)
	}
	top = strings.TrimSpace(top)
	if real, err := filepath.EvalSymlinks(top); err == nil {
		top = real
	}
	here, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if real, err := filepath.EvalSymlinks(here); err == nil {
		here = real
	}
	paths := make([]string, len(typedPaths))
	for i, p := range typedPaths {
		full := typed(p)
		if !filepath.IsAbs(full) {
			full = filepath.Join(here, full)
		}
		rel, err := filepath.Rel(top, full)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, usage("draft add: %s is outside the work tree", p)
		}
		paths[i] = filepath.ToSlash(rel)
	}
	if err := os.Chdir(top); err != nil {
		return nil, err
	}
	return paths, nil
}

// takeChange writes the change of the paths against HEAD as the draft's
// patch, beside the list at file, and gives the paths as the top names them
// and the files the change touches. A path that names nothing in the work
// tree or HEAD, a path git ignores, no change at all and no HEAD are
// refusals, nothing written.
func takeChange(file, id string, typedPaths []string) (paths, names []string, refused *out.Problem, err error) {
	paths, err = topPaths(typedPaths)
	if err != nil {
		return nil, nil, nil, err
	}
	if !git.Succeeds("rev-parse", "--verify", "--quiet", "HEAD^{commit}") {
		return nil, nil, &out.Problem{Rule: "draft-no-head", Message: "the repository has no commit yet, and a change is drafted against HEAD",
			Fix: "commit something first"}, nil
	}
	scratch, err := os.MkdirTemp(filepath.Dir(file), ".index-")
	if err != nil {
		return nil, nil, nil, err
	}
	defer os.RemoveAll(scratch)
	env := draftGitEnv("GIT_INDEX_FILE=" + filepath.Join(scratch, "index"))
	if _, err := draftGit(env, "read-tree", "HEAD"); err != nil {
		return nil, nil, nil, fmt.Errorf("git read-tree HEAD: %w", err)
	}
	if _, err := draftGit(env, append([]string{"add", "--all", "--"}, paths...)...); err != nil {
		return nil, nil, &out.Problem{Rule: "draft-paths", Message: fmt.Sprintf("%s cannot be drafted: %s", and(paths), err),
			Fix: "name files the work tree or HEAD has, and git does not ignore"}, nil
	}
	listed, err := draftGit(env, append([]string{"diff", "--cached", "-z", "--name-only", "--no-renames", "HEAD", "--"}, paths...)...)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("git diff --cached: %w", err)
	}
	names = splitNULs(listed)
	if len(names) == 0 {
		return nil, nil, &out.Problem{Rule: "draft-no-change", Message: fmt.Sprintf("%s has no change against HEAD to draft", and(paths)),
			Fix: "change the files first, or draft a command line with -- <itos args>…"}, nil
	}
	patch, err := draftGit(env, append([]string{"diff", "--cached", "--binary", "--full-index", "--no-renames", "--no-color",
		"--no-ext-diff", "HEAD", "--"}, paths...)...)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("git diff --cached --binary: %w", err)
	}
	if err := draft.WriteFile(draft.Patch(file, id), []byte(patch)); err != nil {
		return nil, nil, nil, err
	}
	return paths, names, nil, nil
}

// splitNULs is a NUL-separated list, the empty entries dropped.
func splitNULs(text string) []string {
	var entries []string
	for _, e := range strings.Split(text, "\x00") {
		if e != "" {
			entries = append(entries, e)
		}
	}
	return entries
}

// putBack puts the files, named from the top, back as HEAD has them, in
// the index and the working tree: one HEAD has is checked out of it, one it
// lacks is taken out of the index and removed, with the folders it leaves
// empty. Nothing else is touched.
func putBack(names []string) error {
	var kept, made []string
	for _, name := range names {
		if git.Succeeds("cat-file", "-e", "HEAD:"+name) {
			kept = append(kept, name)
		} else {
			made = append(made, name)
		}
	}
	env := draftGitEnv()
	var failed error
	if len(kept) > 0 {
		if _, err := draftGit(env, append([]string{"checkout", "HEAD", "--"}, kept...)...); err != nil {
			failed = fmt.Errorf("git checkout HEAD -- %s: %w", and(kept), err)
		}
	}
	if len(made) > 0 {
		if _, err := draftGit(env, append([]string{"rm", "-q", "--cached", "--ignore-unmatch", "--"}, made...)...); err != nil {
			failed = errors.Join(failed, fmt.Errorf("git rm --cached -- %s: %w", and(made), err))
		}
	}
	for _, name := range made {
		if err := os.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
			failed = errors.Join(failed, err)
			continue
		}
		for dir := filepath.Dir(name); dir != "." && dir != string(filepath.Separator); dir = filepath.Dir(dir) {
			if os.Remove(dir) != nil {
				break
			}
		}
	}
	return failed
}

// draftDrop is `draft drop <id>`: the draft taken out of the list, and its
// patch removed, nothing applied.
func draftDrop(args []string, o Out) (int, error) {
	if len(args) != 1 {
		return 0, usage("draft drop needs one <id>")
	}
	id := args[0]
	if !draft.ValidID(id) {
		return 0, usage("draft drop: %q is no draft id", id)
	}
	file, drafts, release, code, err := heldDrafts(o)
	defer release()
	if file == "" || err != nil {
		return code, err
	}
	d := drafts.Find(id)
	if d == nil {
		return noDraft(id, o)
	}
	dropped := *d
	drafts.Remove(id)
	if err := draft.Save(file, drafts); err != nil {
		return 0, err
	}
	if err := os.Remove(draft.Patch(file, id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return 0, err
	}
	if o.JSON {
		return 0, out.Emit(o.Stdout, out.Field{Key: "ok", Value: true}, out.Field{Key: "draft", Value: entryOfDraft(dropped)})
	}
	if !o.Quiet {
		fmt.Fprintf(o.Stdout, "%s dropped: %s\n", id, draftSummary(dropped))
	}
	return 0, nil
}

// noDraft refuses an id no draft has, exit 1.
func noDraft(id string, o Out) (int, error) {
	return refuseWork([]out.Problem{{
		Rule:    "draft-none",
		Message: fmt.Sprintf("there is no draft %s", id),
		Fix:     "itos draft lists the drafts",
	}}, ExitPolicy, o)
}

// workGoingOn are the reasons work may be going on in the checkout, each a
// problem: an item of the registry doing (an agent may hold the checkout),
// a tracked file with a change no commit holds, staged or not (someone's
// work in progress).
func workGoingOn() ([]out.Problem, error) {
	cfg, err := config.Load(config.Path())
	if err != nil {
		return nil, err
	}
	registry, err := work.Load(cfg, cfg.Work.Registry)
	if err != nil {
		return nil, err
	}
	var found []out.Problem
	var doing []string
	for _, item := range registry.Items {
		if item.At("status") == "doing" {
			who := value.String(item.At("owner"))
			if who == "" {
				who = "nobody"
			}
			doing = append(doing, value.String(item.At("id"))+" ("+who+")")
		}
	}
	if len(doing) > 0 {
		found = append(found, out.Problem{
			Rule:    "draft-work-doing",
			Message: fmt.Sprintf("work is going on, so no draft is promoted: %s doing, and an agent may hold the checkout", and(doing)),
			Fix:     "promote once the item is done or dropped",
		})
	}
	var changed []string
	for _, line := range git.Changed() {
		if len(line) > 3 {
			changed = append(changed, line[3:])
		}
	}
	if len(changed) > 0 {
		found = append(found, out.Problem{
			Rule:    "draft-tree-changed",
			Message: fmt.Sprintf("%s %s a change no commit holds, so no draft is promoted over it", and(changed), hasHave(len(changed))),
			Fix:     "promote once that work is committed or put back",
		})
	}
	return found, nil
}

func hasHave(n int) string {
	if n == 1 {
		return "has"
	}
	return "have"
}

// draftPromote is `draft promote`: every draft applied and committed in the
// order they were added, while no work is going on (workGoingOn, else exit
// 1 and nothing applied). Each draft promoted leaves the list at once; the
// first that cannot be applied stops it, the tree left as HEAD has it, that
// draft and the ones after it kept, exit 1 naming it.
func draftPromote(args []string, o Out) (int, error) {
	if len(args) > 0 {
		return 0, usage("draft promote takes no arguments: %s", strings.Join(args, " "))
	}
	file, drafts, release, code, err := heldDrafts(o)
	defer release()
	if file == "" || err != nil {
		return code, err
	}
	promoted := []string{}
	report := func(found []out.Problem) (int, error) {
		if o.JSON {
			ok := len(found) == 0
			fields := []out.Field{{Key: "ok", Value: ok}, {Key: "promoted", Value: promoted}}
			if !ok {
				fields = append(fields, out.Field{Key: "problems", Value: found})
			}
			if err := out.Emit(o.Stdout, fields...); err != nil || ok {
				return 0, err
			}
			return ExitPolicy, nil
		}
		if len(found) == 0 {
			return 0, nil
		}
		return refuseWork(found, ExitPolicy, o)
	}
	if len(drafts.Drafts) == 0 {
		if !o.JSON && !o.Quiet {
			fmt.Fprintln(o.Stdout, "No drafts to promote.")
		}
		return report(nil)
	}
	found, err := workGoingOn()
	if err != nil {
		return 0, err
	}
	// The patches name files from the work tree's top, as git apply reads
	// them where it runs.
	if top, err := git.Output("rev-parse", "--show-toplevel"); err == nil {
		if err := os.Chdir(strings.TrimSpace(top)); err != nil {
			return 0, err
		}
	}
	if len(found) > 0 {
		return report(found)
	}
	for _, d := range drafts.Drafts {
		if d.IsChange() {
			if err := hooksReady("commit-msg"); err != nil {
				return 0, err
			}
			break
		}
	}
	for len(drafts.Drafts) > 0 {
		d := drafts.Drafts[0]
		header, why, err := promoteOne(file, d, o)
		if err != nil {
			return 0, err
		}
		if why != "" {
			kept := "it is kept"
			if n := len(drafts.Drafts) - 1; n > 0 {
				kept = fmt.Sprintf("it and the %d after it are kept", n)
			}
			return report([]out.Problem{{
				Rule:    "draft-not-applied",
				Message: fmt.Sprintf("%s cannot be promoted: %s; %s, the tree left as HEAD has it", d.ID, why, kept),
				Fix:     "redo it (itos draft drop " + d.ID + ", then itos draft add) or fix what stopped it, then itos draft promote again",
			}})
		}
		drafts.Drafts = drafts.Drafts[1:]
		if err := draft.Save(file, drafts); err != nil {
			return 0, fmt.Errorf("%s is promoted, but the list cannot be saved without it, so a promote would apply it again: %w", d.ID, err)
		}
		if err := os.Remove(draft.Patch(file, d.ID)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return 0, err
		}
		promoted = append(promoted, d.ID)
		if !o.JSON && !o.Quiet {
			fmt.Fprintf(o.Stdout, "%s promoted: %s\n", d.ID, header)
		}
	}
	return report(nil)
}

// promoteOne applies one draft: a change applied to the index and the work
// tree and committed with its message, through the hooks; a command run by
// this itos. It gives the commit's header line, else why the draft cannot
// be promoted, the tree then as HEAD has it; an error when itos cannot even
// try.
func promoteOne(file string, d draft.Draft, o Out) (header, why string, err error) {
	before, _ := git.Output("rev-parse", "HEAD")
	if !d.IsChange() {
		self, err := os.Executable()
		if err != nil {
			return "", "", fmt.Errorf("this itos cannot find its own binary to run %s: %w", d.ID, err)
		}
		cmd := exec.Command(self, d.Command...)
		cmd.Stdout, cmd.Stderr = o.Stderr, o.Stderr
		if err := cmd.Run(); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return "", fmt.Sprintf("%s exited %d", d.Line(), exit.ExitCode()), nil
			}
			return "", "", err
		}
		return madeSince(before, d.Line()), "", nil
	}
	patch := draft.Patch(file, d.ID)
	if _, err := os.Stat(patch); err != nil {
		return "", "", fmt.Errorf("%s's patch cannot be read: %w", d.ID, err)
	}
	listed, err := draftGit(draftGitEnv(), "apply", "--numstat", "-z", patch)
	if err != nil {
		return "", "", fmt.Errorf("%s's patch cannot be read: %w", d.ID, err)
	}
	names := patchNames(listed)
	if _, err := draftGit(draftGitEnv(), "apply", "--index", "--whitespace=nowarn", patch); err != nil {
		return "", "the change no longer applies to the tree (" + oneLine(err.Error()) + ")", nil
	}
	cmd := exec.Command(git.Bin(), "commit", "-q", "-F", "-")
	cmd.Env = append(withoutFooters(os.Environ()), AmendEnv+"=0")
	cmd.Stdin = strings.NewReader(d.Message)
	cmd.Stdout, cmd.Stderr = o.Stderr, o.Stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			return "", "", errors.Join(err, putBack(names))
		}
		if err := putBack(names); err != nil {
			return "", "", fmt.Errorf("the commit of %s failed, and its files cannot be put back: %w", d.ID, err)
		}
		return "", fmt.Sprintf("its commit failed (git exited %d)", exit.ExitCode()), nil
	}
	return madeSince(before, d.Header()), "", nil
}

// patchNames are the files a patch touches, from git apply --numstat -z:
// each entry added, deleted and the path, renames being off in its diff.
func patchNames(numstat string) []string {
	var names []string
	for _, entry := range splitNULs(numstat) {
		fields := strings.SplitN(entry, "\t", 3)
		if len(fields) == 3 && fields[2] != "" {
			names = append(names, fields[2])
		}
	}
	return names
}

// madeSince is the commit HEAD is now, as a promote reports it: its short
// SHA and header, or what was run when HEAD did not move.
func madeSince(before, what string) string {
	now, err := git.Output("log", "-1", "--format=%h %s")
	after, _ := git.Output("rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(after) == strings.TrimSpace(before) {
		return what + " (no commit)"
	}
	return "committed " + strings.TrimSpace(now)
}
