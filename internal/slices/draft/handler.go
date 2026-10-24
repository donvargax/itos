package draft

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/donvargax/itos/v8/internal/config"
	domain "github.com/donvargax/itos/v8/internal/draft"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/out"
	privateport "github.com/donvargax/itos/v8/internal/store/private/port"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	"github.com/donvargax/itos/v8/internal/work"
)

// Query is what draft asks: Sub, "" for the list, or add, edit, promote or
// drop; Draft, the draft add or edit keeps (its id, and its message or its
// command line), or the id drop names; Paths, add's and edit's paths as
// typed; and Origin, the folder itos was started in, which they are read
// from.
type Query struct {
	Sub    string
	Draft  domain.Draft
	Paths  []string
	Origin string
}

// Ports are what draft reaches the world through, which internal/cli binds.
// The VCS is the repository, which the drafts' folder is located in, HEAD
// read, the checkout judged, a change committed and files put back as HEAD
// has them. Hold is the private store's lock of a file in itos's folder of
// the common dir. The patches are git's, which the VCS port leaves to its
// adapter: TreeEntries, Blob, PatchFiles, Apply and a Scratch index.
// HooksReady refuses a commit itos's hooks would not see. A command draft
// is run by this itos's binary, Self, through Exec, which gives its exit
// code; one whose id was minted when drafted, in this process through
// RunLine. Minting reads a drafted line as its command would (Minting).
// CreateTemp and Save are os.CreateTemp and domain.Save, ports so a test
// makes them fail, as no failure of theirs comes alike from every OS. At
// is the ports of the same run in the folder dir, which a command that
// moves to the top partway through reads and runs from on: the VCS's
// Folder is the one it runs in, and the process's never changes.
type Ports interface {
	vcsport.VCS
	Minting
	Hold(ctx context.Context, name string) (privateport.Held, error)
	TreeEntries(ctx context.Context, commit string, paths []string) (map[string]TreeEntry, error)
	Blob(ctx context.Context, object string) (string, error)
	PatchFiles(ctx context.Context, patch string) (string, error)
	Apply(ctx context.Context, patch string) error
	Scratch(ctx context.Context, index string) Index
	HooksReady(ctx context.Context, event string) error
	Self(ctx context.Context) (string, error)
	Exec(ctx context.Context, self string, args []string, w io.Writer) (int, error)
	RunLine(ctx context.Context, args []string, w io.Writer, reserved string) int
	CreateTemp(ctx context.Context, dir, pattern string) (*os.File, error)
	Save(ctx context.Context, file string, drafts domain.File) error
	At(ctx context.Context, dir string) Ports
}

// Minting is how a drafted command line that mints an item's id is read
// and its id minted, through the code of the commands that mint one, which
// stays in internal/cli. Globals are the line's global flags as itos reads
// them, ApplyGlobals sets what they set in this process's environment and
// gives the Minting of the folder they move to, and ReadLine is the line
// as the command table reads it, the command's name and its
// arguments. WorkAddKind and WorkPromoteKind are the kind work add and work
// promote would make of their arguments, TaskAdd refuses what task add
// would; each a usage error for a line the command refuses. Config is the
// config, Sound the work registry read sound, its lock held, and Mint mints
// the kind's next id past registryIDs.
type Minting interface {
	Globals(ctx context.Context, command []string) Line
	ApplyGlobals(ctx context.Context, command []string) (Minting, error)
	ReadLine(ctx context.Context, command []string) ([]string, error)
	WorkAddKind(ctx context.Context, args []string) (string, error)
	WorkPromoteKind(ctx context.Context, args []string) (string, error)
	TaskAdd(ctx context.Context, cfg *config.Loaded, args []string) error
	Config(ctx context.Context) (*config.Loaded, error)
	Sound(ctx context.Context) (Sound, error)
	Mint(ctx context.Context, cfg *config.Loaded, kind string, registryIDs []string) (string, error)
}

// Line is an itos command line's global flags as itos reads them: whether
// it asks for help, and Rest, the rest of its arguments.
type Line struct {
	Help bool
	Rest []string
}

// Sound is the work registry as Minting.Sound read it: the config and the
// registry, and Release, never nil, which gives back the lock held. A nil
// Cfg is a registry not sound, its problems reported, Code the exit.
type Sound struct {
	Cfg      *config.Loaded
	Registry work.Registry
	Release  func()
	Code     int
}

// TreeEntry is one entry of a commit's tree: its mode, its kind (blob,
// tree, commit) and its object.
type TreeEntry struct{ Mode, Kind, Object string }

// Index is an index of its own at a file, which a draft's change is staged
// in, apart from the repository's: ReadTree sets it to a commit's tree,
// AddAll stages the paths as the work tree has them, a file it lacks taken
// out, Remove takes a path out, Store writes a file's content as a blob and
// gives its object, Set sets a path's entry, Changed are the paths of those
// given that it changes against base, and Diff is its change of the paths
// against base as a binary patch git apply takes.
type Index interface {
	ReadTree(ctx context.Context, rev string) error
	AddAll(ctx context.Context, paths []string) error
	Remove(ctx context.Context, path string) error
	Store(ctx context.Context, file string) (string, error)
	Set(ctx context.Context, mode, object, path string) error
	Changed(ctx context.Context, base string, paths []string) ([]string, error)
	Diff(ctx context.Context, base string, paths []string) (string, error)
}

// Handler is draft's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// at is the handler over the ports in the folder dir (Ports.At).
func (h Handler) at(ctx context.Context, dir string) Handler {
	return Handler{ports: h.ports.At(ctx, dir)}
}

// Progress is where promote reports as it goes: Stderr, which a command
// draft's output and a commit's words go to, and Promoted, told of each
// draft promoted, with its commit, as soon as it is.
type Progress struct {
	Stderr   io.Writer
	Promoted func(id, header string)
}

// Answer is what Handle found or did: no repository, so no drafts; Code, a
// refusal Ports reported; the Problems found, nothing written; the Drafts
// listed; the Draft added, edited or dropped, and an edit draft's Copies;
// the drafts Promoted; and the lock's failure to be given back, Released.
type Answer struct {
	NoRepository bool
	Code         int
	Problems     []out.Problem
	Drafts       []domain.Draft
	Draft        domain.Draft
	Copies       []string
	Promoted     []string
	Released     error
}

// Handle answers q. add, edit, drop and promote hold the list's lock from
// before they read it to after they write it (as the followup slice's
// Handle, bug 16).
func (h Handler) Handle(ctx context.Context, q Query, p Progress) (a Answer, err error) {
	file := domain.Path(ctx, h.ports)
	if file == "" {
		return Answer{NoRepository: true}, nil
	}
	if q.Sub == "" {
		drafts, err := domain.Load(file)
		return Answer{Drafts: drafts.Drafts}, err
	}
	held, err := h.ports.Hold(ctx, filepath.Join(domain.Folder, domain.FileName))
	if err != nil {
		return a, err
	}
	defer func() { a.Released = held.Release(ctx) }()
	drafts, err := domain.Load(file)
	if err != nil {
		return a, err
	}
	switch q.Sub {
	case "add":
		return h.add(ctx, q, file, drafts, p)
	case "edit":
		return h.edit(ctx, q, file, drafts)
	case "drop":
		return h.drop(ctx, q, file, drafts)
	}
	return h.promote(ctx, file, drafts, p)
}

// idTaken refuses a new draft the id of the one there.
func idTaken(had domain.Draft) Answer {
	return Answer{Problems: []out.Problem{{
		Rule:    "draft-id-taken",
		Message: fmt.Sprintf("%s is already a draft: %s", had.ID, domain.Summary(had)),
		Fix:     "another id adds a new one; itos draft drop " + had.ID + " drops the one there",
	}}}
}

// add is `draft add`: a change, its patch written beside the list at file
// and its files put back as HEAD has them, or a command line, whose item's
// id is minted now when it mints one (mint.go).
func (h Handler) add(ctx context.Context, q Query, file string, drafts domain.File, p Progress) (Answer, error) {
	d := q.Draft
	if had := drafts.Find(d.ID); had != nil {
		return idTaken(*had), nil
	}
	if !d.IsChange() {
		minted, code, err := h.mint(ctx, d.Command)
		if code != 0 || err != nil {
			return Answer{Code: code}, err
		}
		d.Minted = minted
	}
	var names []string
	if d.IsChange() {
		var refused *out.Problem
		var err error
		d.Paths, names, h, refused, err = h.takeChange(ctx, file, d.ID, q.Paths, q.Origin)
		if err != nil {
			return Answer{}, err
		}
		if refused != nil {
			return Answer{Problems: []out.Problem{*refused}}, nil
		}
	}
	drafts.Drafts = append(drafts.Drafts, d)
	if err := h.ports.Save(ctx, file, drafts); err != nil {
		os.Remove(domain.Patch(file, d.ID))
		return Answer{}, err
	}
	if d.IsChange() {
		if err := h.putBack(ctx, names); err != nil {
			return Answer{}, fmt.Errorf("%s is drafted, but its files cannot be put back as HEAD has them: %w", d.ID, err)
		}
	}
	return Answer{Draft: d}, nil
}

// edit is `draft edit`: a change drafted as a copy of each path to edit in
// the draft's own folder, HEAD's content or empty for a file HEAD lacks,
// the draft's base HEAD. The working tree is neither read nor written.
func (h Handler) edit(ctx context.Context, q Query, file string, drafts domain.File) (Answer, error) {
	id := q.Draft.ID
	if had := drafts.Find(id); had != nil {
		return idTaken(*had), nil
	}
	paths, h, err := h.topPaths(ctx, "draft edit", q.Paths, q.Origin)
	if err != nil {
		return Answer{}, err
	}
	seen := map[string]bool{}
	paths = slices.DeleteFunc(paths, func(p string) bool {
		had := seen[p]
		seen[p] = true
		return had
	})
	base, err := h.ports.Resolve(ctx, "HEAD")
	if err != nil {
		return Answer{Problems: []out.Problem{{Rule: "draft-no-head", Message: "the repository has no commit yet, and a copy is taken from HEAD",
			Fix: "commit something first"}}}, nil
	}
	had, err := h.treeEntries(ctx, base, paths)
	if err != nil {
		return Answer{}, err
	}
	var folders []string
	for _, p := range paths {
		if e, ok := had[p]; p == "." || ok && e.Kind != "blob" {
			folders = append(folders, p)
		}
	}
	if len(folders) > 0 {
		return Answer{Problems: []out.Problem{{Rule: "draft-paths",
			Message: fmt.Sprintf("%s %s no file in HEAD, but a folder or a submodule, and draft edit copies files", and(folders), isAre(len(folders))),
			Fix:     "name the files in it to edit"}}}, nil
	}
	copies := domain.Copies(file, id)
	if err := os.RemoveAll(copies); err != nil {
		return Answer{}, err
	}
	written := make([]string, len(paths))
	for i, p := range paths {
		var content string
		if e, ok := had[p]; ok {
			if content, err = h.ports.Blob(ctx, e.Object); err != nil {
				os.RemoveAll(copies)
				return Answer{}, fmt.Errorf("git cat-file blob HEAD:%s: %w", p, err)
			}
		}
		written[i] = domain.Copy(file, id, p)
		if err := domain.WriteFile(written[i], []byte(content)); err != nil {
			os.RemoveAll(copies)
			return Answer{}, err
		}
	}
	d := domain.Draft{ID: id, Message: q.Draft.Message, Paths: paths, Base: base}
	drafts.Drafts = append(drafts.Drafts, d)
	if err := h.ports.Save(ctx, file, drafts); err != nil {
		os.RemoveAll(copies)
		return Answer{}, err
	}
	return Answer{Draft: d, Copies: written}, nil
}

func isAre(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

// drop is `draft drop`: the draft taken out of the list, and its patch
// removed, nothing applied.
func (h Handler) drop(ctx context.Context, q Query, file string, drafts domain.File) (Answer, error) {
	id := q.Draft.ID
	d := drafts.Find(id)
	if d == nil {
		return noDraft(id), nil
	}
	dropped := *d
	drafts.Remove(id)
	if err := h.ports.Save(ctx, file, drafts); err != nil {
		return Answer{}, err
	}
	if err := removeDraftFiles(file, id); err != nil {
		return Answer{}, err
	}
	return Answer{Draft: dropped}, nil
}

// removeDraftFiles removes what a draft kept beside the list: a change's
// patch, an edit draft's copies.
func removeDraftFiles(file, id string) error {
	if err := os.Remove(domain.Patch(file, id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.RemoveAll(domain.Copies(file, id))
}

// noDraft refuses an id no draft has.
func noDraft(id string) Answer {
	return Answer{Problems: []out.Problem{{
		Rule:    "draft-none",
		Message: fmt.Sprintf("there is no draft %s", id),
		Fix:     "itos draft lists the drafts",
	}}}
}

// checkoutBusy are the reasons the checkout is not clean, each a problem:
// a tracked file with a change no commit holds, staged or not (someone's
// work in progress), and a merge or a rebase stopped part way, which a
// commit would conclude. An item's status is no reason (the user's call,
// 2026-10-06): items stay doing for the coordinator's own work and for
// agents that have gone, and what collides with an agent is its index and
// work tree.
func (h Handler) checkoutBusy(ctx context.Context) []out.Problem {
	var found []out.Problem
	var changed []string
	for _, line := range h.ports.Changes(ctx) {
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
	var stopped []string
	if h.ports.Rebasing(ctx) {
		stopped = append(stopped, "a rebase")
	}
	if len(h.ports.MergeHeads(ctx)) > 0 {
		stopped = append(stopped, "a merge")
	}
	if len(stopped) > 0 {
		found = append(found, out.Problem{
			Rule:    "draft-checkout-busy",
			Message: fmt.Sprintf("%s is in progress, which a draft's commit would conclude, so no draft is promoted", and(stopped)),
			Fix:     "promote once it is finished or aborted",
		})
	}
	return found
}

func hasHave(n int) string {
	if n == 1 {
		return "has"
	}
	return "have"
}

// promote is `draft promote`: every draft applied and committed in the
// order they were added, while the checkout is clean (checkoutBusy, else
// refused and nothing applied). Each draft promoted leaves the list at
// once; the first that cannot be applied stops it, the tree left as HEAD
// has it, that draft and the ones after it kept, a problem naming it.
func (h Handler) promote(ctx context.Context, file string, drafts domain.File, p Progress) (Answer, error) {
	a := Answer{Promoted: []string{}}
	if len(drafts.Drafts) == 0 {
		return a, nil
	}
	// The patches name files from the work tree's top, as git apply reads
	// them where it runs.
	if l, err := h.ports.Locate(ctx, ""); err == nil && l.Top != "" {
		h = h.at(ctx, l.Top)
	}
	if found := h.checkoutBusy(ctx); len(found) > 0 {
		a.Problems = found
		return a, nil
	}
	for _, d := range drafts.Drafts {
		if d.IsChange() {
			if err := h.ports.HooksReady(ctx, "commit-msg"); err != nil {
				return Answer{}, err
			}
			break
		}
	}
	for len(drafts.Drafts) > 0 {
		d := drafts.Drafts[0]
		header, why, err := h.promoteOne(ctx, file, d, p.Stderr)
		if err != nil {
			return Answer{}, err
		}
		if why != "" {
			kept := "it is kept"
			if n := len(drafts.Drafts) - 1; n > 0 {
				kept = fmt.Sprintf("it and the %d after it are kept", n)
			}
			a.Problems = []out.Problem{{
				Rule:    "draft-not-applied",
				Message: fmt.Sprintf("%s cannot be promoted: %s; %s, the tree left as HEAD has it", d.ID, why, kept),
				Fix:     redoFix(file, d),
			}}
			return a, nil
		}
		drafts.Drafts = drafts.Drafts[1:]
		if err := h.ports.Save(ctx, file, drafts); err != nil {
			return Answer{}, fmt.Errorf("%s is promoted, but the list cannot be saved without it, so a promote would apply it again: %w", d.ID, err)
		}
		if err := removeDraftFiles(file, d.ID); err != nil {
			return Answer{}, err
		}
		a.Promoted = append(a.Promoted, d.ID)
		p.Promoted(d.ID, header)
	}
	return a, nil
}

// redoFix is how a draft that cannot be promoted is redone.
func redoFix(file string, d domain.Draft) string {
	if d.IsEdit() {
		return "redo it from HEAD (keep what you need of its copies in " + domain.Copies(file, d.ID) + ", itos draft drop " + d.ID +
			", then itos draft edit) or fix what stopped it, then itos draft promote again"
	}
	return "redo it (itos draft drop " + d.ID + ", then itos draft add) or fix what stopped it, then itos draft promote again"
}

// promoteOne applies one draft: a change applied to the index and the work
// tree and committed with its message, through the hooks; a command run by
// this itos, its output on w. It gives the commit's header line, else why
// the draft cannot be promoted, the tree then as HEAD has it; an error when
// itos cannot even try.
func (h Handler) promoteOne(ctx context.Context, file string, d domain.Draft, w io.Writer) (header, why string, err error) {
	before, _ := h.ports.Resolve(ctx, "HEAD")
	if d.Minted != "" {
		if code := h.runReserved(ctx, d, w); code != 0 {
			return "", fmt.Sprintf("%s exited %d", d.Line(), code), nil
		}
		return h.madeSince(ctx, before, d.Line()), "", nil
	}
	if !d.IsChange() {
		self, err := h.ports.Self(ctx)
		if err != nil {
			return "", "", fmt.Errorf("this itos cannot find its own binary to run %s: %w", d.ID, err)
		}
		code, err := h.ports.Exec(ctx, self, d.Command, w)
		if err != nil {
			return "", "", err
		}
		if code != 0 {
			return "", fmt.Sprintf("%s exited %d", d.Line(), code), nil
		}
		return h.madeSince(ctx, before, d.Line()), "", nil
	}
	patch := domain.Patch(file, d.ID)
	if d.IsEdit() {
		made, why, err := h.editPatch(ctx, file, d)
		if err != nil || why != "" {
			return "", why, err
		}
		defer os.Remove(made)
		patch = made
	} else if _, err := os.Stat(patch); err != nil {
		return "", "", fmt.Errorf("%s's patch cannot be read: %w", d.ID, err)
	}
	listed, err := h.ports.PatchFiles(ctx, patch)
	if err != nil {
		return "", "", fmt.Errorf("%s's patch cannot be read: %w", d.ID, err)
	}
	names := patchNames(listed)
	if err := h.ports.Apply(ctx, patch); err != nil {
		return "", "the change no longer applies to the tree (" + oneLine(err.Error()) + ")", nil
	}
	err = h.ports.Commit(ctx, vcsport.Commit{Message: d.Message, Quiet: true},
		vcsport.Talk{Env: append(message.WithoutFooters(os.Environ()), message.AmendEnv+"=0"), Stdout: w, Stderr: w})
	if err != nil {
		var failed *vcsport.Failed
		if !errors.As(err, &failed) {
			return "", "", errors.Join(err, h.putBack(ctx, names))
		}
		if err := h.putBack(ctx, names); err != nil {
			return "", "", fmt.Errorf("the commit of %s failed, and its files cannot be put back: %w", d.ID, err)
		}
		return "", fmt.Sprintf("its commit failed (git exited %d)", failed.Code), nil
	}
	return h.madeSince(ctx, before, d.Header()), "", nil
}

// madeSince is the commit HEAD is now, as a promote reports it: its short
// SHA and header, or what was run when HEAD did not move.
func (h Handler) madeSince(ctx context.Context, before, what string) string {
	now, err := h.ports.Show(ctx, "HEAD")
	if err != nil || now.SHA == before {
		return what + " (no commit)"
	}
	return "committed " + now.Short + " " + now.Subject
}
