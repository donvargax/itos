package brief

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/agentrules"
	guidetext "github.com/donvargax/itos/v8/internal/guide"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/tests"
	"github.com/donvargax/itos/v8/internal/value"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	domain "github.com/donvargax/itos/v8/internal/work"
)

// Ports are what the brief command reaches the world through, which
// internal/cli binds: the repository; Config, the config and its data as
// the commit's tree has them, so that an uncommitted change to the policy
// changes no brief and its Source is the tree the reads are made from
// (ADR-0053's --data-at reads a commit's data by the working tree's
// config; the brief is a function of one commit); TreeEntries, a
// commit's entries at paths, each read and named from the folder itos
// runs in, as the config's paths are, a path the commit lacks among them
// not at all; and LastTouching, the last commit on a
// revision's first-parent history touching one of the paths, "" when none
// does.
type Ports interface {
	vcsport.VCS
	Config(ctx context.Context, commit string) (*config.Loaded, error)
	TreeEntries(ctx context.Context, commit string, paths []string) (map[string]TreeEntry, error)
	LastTouching(ctx context.Context, rev string, paths []string) (string, error)
}

// TreeEntry is one entry of a commit's tree: its Kind, blob or tree, its
// Mode as git lists it (a symbolic link is a blob of mode 120000), and its
// Object, the blob's or the tree's id.
type TreeEntry struct{ Kind, Mode, Object string }

// Handler is the brief command's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Read is one file the agent reads: its Path from the top, its Blob at
// HEAD, the Reason it is read and whether it is Required, true for
// AGENTS.md and the spec's files, false for a ref.
type Read struct {
	Path     string `json:"path"`
	Blob     string `json:"blob"`
	Reason   string `json:"reason"`
	Required bool   `json:"required"`
}

// Omitted is a file or a ref left out of the reads, and the Reason.
type Omitted struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Brief is a brief as Handle gives it: the Item and Role briefed, the
// Request digest, the Base commit it was computed at, the Spec commit (""
// when nothing is tagged yet), the Reads and the Omitted, and the Text
// printed.
type Brief struct {
	Item, Role, Request, Base, Spec string
	Reads                           []Read
	Omitted                         []Omitted
	Text                            string
}

// Answer is what Handle found: the problems a brief is Refused for, or the
// Brief.
type Answer struct {
	Refused []out.Problem
	Brief   *Brief
}

// head is the revision everything is read from when --at names none.
const head = "HEAD"

// Handle answers q: the role's template filled from the item at the commit
// q.At names, HEAD's when it names none, or why it is refused. An error is
// a config or a repository that cannot be read, each with its kind.
func (h Handler) Handle(ctx context.Context, q Query) (Answer, error) {
	if !slices.Contains(guidetext.Roles, q.Role) {
		return refused(out.Problem{Rule: "brief-unknown-role", Message: "no brief for the role " + q.Role,
			Fix: "name one of the roles: drafter, implementer or reviewer"}), nil
	}
	problems, base, err := h.base(ctx, q.At)
	if problems != nil || err != nil {
		return Answer{Refused: problems}, err
	}
	at, err := h.ports.Config(ctx, base)
	if err != nil {
		return Answer{}, err
	}
	src := at.Source
	file := at.Work.Registry
	if !src.Has(ctx, file) {
		return refused(domain.Missing(file, false)), nil
	}
	registry, err := domain.Load(at, file)
	if err != nil {
		return Answer{}, err
	}
	shown, problem := domain.Show(registry, q.ID)
	if problem != nil {
		return refused(*problem), nil
	}
	f := facts{ID: q.ID, Role: q.Role, Kind: itemKind(shown.Item, taskIDs(at)), Depends: dependencies(registry, shown.Item)}
	if f.Kind == "idea" {
		return refused(out.Problem{Rule: "brief-idea", Message: q.ID + " is an idea, which has no spec and no kind to brief for",
			Fix: fmt.Sprintf("make it work first: itos work promote %s --kind slice, or --kind task --type <type> --check <command>", q.ID)}), nil
	}
	f.Title = value.String(shown.Item.At("title"))
	if f.Title == "" || absent(shown.Item.At("title")) {
		f.Title = q.ID
	}
	l, err := h.ports.Locate(ctx, "")
	if err != nil {
		return Answer{}, kind.Else(kind.Missing, err)
	}
	f.Top = filepath.ToSlash(l.Top)
	if err := h.spec(ctx, at, base, &f); err != nil {
		return Answer{}, err
	}
	if len(f.specFiles) == 0 && q.Role != guidetext.Drafter {
		return refused(noSpec(q.ID, f.Kind)), nil
	}
	b := &Brief{Item: q.ID, Role: q.Role, Base: base, Reads: []Read{}, Omitted: []Omitted{}}
	if len(f.specFiles) > 0 {
		if b.Spec, err = h.ports.LastTouching(ctx, base, f.specFiles); err != nil {
			return Answer{}, err
		}
	}
	if err := h.reads(ctx, b, &f, refsOf(shown.Item)); err != nil {
		return Answer{}, err
	}
	f.Lines = lines(ctx, at, src)
	b.Text = fill(q.Role, f)
	b.Request = request(b)
	return Answer{Brief: b}, nil
}

func refused(p out.Problem) Answer { return Answer{Refused: []out.Problem{p}} }

// base is the full name of the commit the brief is computed at: the one at
// names, HEAD's when it names none. A revision git cannot resolve is
// refused, brief-unknown-revision; a repository with no commit, when at
// names none, is an error, a missing environment.
func (h Handler) base(ctx context.Context, at string) ([]out.Problem, string, error) {
	if at == "" {
		sha, err := h.ports.Resolve(ctx, head)
		if err != nil {
			return nil, "", kind.Wrap(kind.Missing, errors.New("itos brief reads HEAD's tree, and the repository has no commit"))
		}
		return nil, sha, nil
	}
	sha, err := h.ports.Resolve(ctx, at)
	if err != nil {
		return []out.Problem{{Rule: "brief-unknown-revision", Message: "git cannot resolve " + at + " to a commit of the clone",
			Fix: "name a commit the clone has with --at: a SHA, a branch, a tag, or origin/<branch> once fetched"}}, "", nil
	}
	return nil, sha, nil
}

// noSpec is the refusal of an item nothing specifies yet, for a role that
// works from a spec: the drafter's brief is the fix.
func noSpec(id, k string) out.Problem {
	what := "no scenario is tagged @" + id + " yet"
	if k == "task" {
		what = id + " has no ledger entry yet"
	}
	return out.Problem{Rule: "brief-no-spec", Message: what + ", so " + id + " has no spec to brief from",
		Fix: "itos brief " + id + " --role drafter briefs the drafter, who writes it"}
}

// absent is a registry value an item does not give.
func absent(v any) bool { return v == nil || v == value.Undefined }

// bugID is a bug's id, bug-<n>.
var bugID = regexp.MustCompile(`^bug-\d+$`)

// taskIDs is ledger.id as a pattern, nil when the config has none, where
// an item of no kind is never a task by its id (work.SpecKind), as work
// show reads it.
func taskIDs(at *config.Loaded) *regexp.Regexp {
	if at.Ledger.ID == nil {
		return nil
	}
	return ledger.IDPattern(at)
}

// itemKind is the item's kind as the brief names it: its kind when it gives
// one, else slice or task as the registry's header reads an id, bug for
// bug-<n>, and idea for any other.
func itemKind(item *value.Map, taskID *regexp.Regexp) string {
	if k := item.At("kind"); !absent(k) {
		return value.String(k)
	}
	if k := domain.SpecKind(item, taskID); k != "" {
		return k
	}
	if bugID.MatchString(value.String(item.At("id"))) {
		return "bug"
	}
	return "idea"
}

// dependency is an item the briefed one depends on, with its status in the
// registry, "unknown" for an id no item has.
type dependency struct{ ID, Status string }

// dependencies are the item's depends_on, each with its status.
func dependencies(r domain.Registry, item *value.Map) []dependency {
	deps, _ := item.At("depends_on").([]any)
	var found []dependency
	for _, dep := range deps {
		d := dependency{ID: value.String(dep), Status: "unknown"}
		for _, other := range r.Items {
			if value.String(other.At("id")) == d.ID {
				if status := other.At("status"); !absent(status) {
					d.Status = value.String(status)
				}
				break
			}
		}
		found = append(found, d)
	}
	return found
}

// refsOf are the item's refs as written, "" and non-texts left out.
func refsOf(item *value.Map) []string {
	listed, _ := item.At("refs").([]any)
	var refs []string
	for _, ref := range listed {
		if r := value.String(ref); r != "" && !absent(ref) {
			refs = append(refs, r)
		}
	}
	return refs
}

// spec reads the item's spec at the commit base into f: a task's ledger
// entry, its checks and its file, or the scenarios tagged for the item and
// their files, each once, in the order found.
func (h Handler) spec(_ context.Context, at *config.Loaded, base string, f *facts) error {
	if f.Kind == "task" {
		// Files first: Tasks reads them too, so a folder that cannot be
		// listed fails here and a file that is not YAML there.
		files, err := ledger.Files(at)
		if err != nil {
			return err
		}
		tasks, err := ledger.Tasks(at)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(tasks, func(t ledger.Task) bool { return t.ID == f.ID })
		if i < 0 {
			return nil
		}
		// The task's file is the one of its group, read as Tasks reads a
		// file's group: a number under ledger.group.numeric (phase-01's
		// is 1), else the text.
		for _, file := range files {
			var group any = file.Group
			if at.Ledger.Group.Numeric {
				group = value.ToNumber(file.Group)
			}
			if value.String(group) == tasks[i].GroupText() {
				f.specFiles = []string{file.Path}
				break
			}
		}
		f.task = &tasks[i]
		return nil
	}
	scenarios, err := tests.TaggedFiles(at, "@"+f.ID, base)
	if err != nil {
		return err
	}
	f.Scenarios = scenarios
	for _, s := range scenarios {
		if !slices.Contains(f.specFiles, s.Path) {
			f.specFiles = append(f.specFiles, s.Path)
		}
	}
	return nil
}

// reads fills b's Reads and Omitted, and f's paths read, from the entries
// of b's Base: AGENTS.md, the spec's files, then each ref, a file as it is, a
// folder by its doc.go; what is not there, or is a symbolic link (git's
// mode 120000: the agent would read whatever the link points at in its
// checkout, not the blob the request records), is omitted with its reason.
// A ref is looked up by its clean path (./docs/plan.md is docs/plan.md,
// internal/foo/ is internal/foo) and named as written; one inTop refuses is
// not looked up. A folder is asked for with its doc.go, which makes git
// list the folder's entries rather than the folder itself, so a folder is
// known by an entry under it.
func (h Handler) reads(ctx context.Context, b *Brief, f *facts, refs []string) error {
	paths := append([]string{agentrules.AgentsFile}, f.specFiles...)
	for _, ref := range refs {
		if p := path.Clean(ref); inTop(p) {
			paths = append(paths, p, path.Join(p, "doc.go"))
		}
	}
	entries, err := h.ports.TreeEntries(ctx, b.Base, paths)
	if err != nil {
		return err
	}
	folder := func(p string) bool {
		for name := range entries {
			if strings.HasPrefix(name, p+"/") {
				return true
			}
		}
		return false
	}
	read := func(p, reason string, required bool) bool {
		if slices.ContainsFunc(b.Reads, func(r Read) bool { return r.Path == p }) {
			return true
		}
		e, ok := entries[p]
		if !ok || e.Kind != "blob" || e.Mode == symlinkMode {
			return false
		}
		b.Reads = append(b.Reads, Read{Path: p, Blob: e.Object, Reason: reason, Required: required})
		f.Reads = append(f.Reads, p)
		return true
	}
	// omit leaves written out: a symbolic link at p says so, else why.
	omit := func(written, p, why string) {
		if e, ok := entries[p]; ok && e.Mode == symlinkMode {
			why = symlinked
		}
		b.Omitted = append(b.Omitted, Omitted{Path: written, Reason: why})
	}
	if !read(agentrules.AgentsFile, "the repository's agent rules", true) {
		omit(agentrules.AgentsFile, agentrules.AgentsFile, notInTree)
	}
	for _, p := range f.specFiles {
		reason := "the item's scenarios"
		if f.task != nil {
			reason = "the task's ledger entry"
		}
		if !read(p, reason, true) {
			omit(p, p, notInTree)
		}
	}
	for _, ref := range refs {
		p := path.Clean(ref)
		reason := "the item's ref " + ref
		switch {
		case !inTop(p):
			omit(ref, p, notInTree)
		case read(p, reason, false):
		case folder(p):
			if doc := path.Join(p, "doc.go"); !read(doc, reason, false) {
				omit(ref, doc, "no doc.go at HEAD")
			}
		default:
			omit(ref, p, notInTree)
		}
	}
	f.Omitted = b.Omitted
	return nil
}

// inTop is whether a ref's clean path p is one git ls-tree is asked for: a
// path under the folder itos runs in, not that folder, nor above it, nor
// absolute, a drive's or one with a backslash (on Windows, git reads
// either as a path outside the repository and refuses the whole listing).
// Any other is not in the tree, on every OS alike. The adapter runs git
// with literal pathspecs, so a leading colon is a name, not magic.
func inTop(p string) bool {
	if p == "." || p == ".." || strings.HasPrefix(p, "../") || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) {
		return false
	}
	return len(p) < 2 || p[1] != ':'
}

// notInTree is why a path HEAD lacks is omitted.
const notInTree = "not in the tree at HEAD"

// symlinkMode is git's mode of a symbolic link, and symlinked why one is
// omitted.
const (
	symlinkMode = "120000"
	symlinked   = "a symbolic link at HEAD"
)

// briefHeading heads the section of the repository's notes that follows
// the filled template.
const briefHeading = "## What a brief adds here"

// heading is a line that ends the section: a level-one or level-two
// heading.
var heading = regexp.MustCompile(`^#{1,2}(\s|$)`)

// lines is the body of the "## What a brief adds here" section of the
// file guide.orchestrating names, read at HEAD, "" when the file or the
// section is not there, else ending in one newline.
func lines(ctx context.Context, at *config.Loaded, src treeport.Source) string {
	p, _ := at.Get("guide.orchestrating").(string)
	if p == "" || !src.Has(ctx, p) {
		return ""
	}
	text, err := src.Read(ctx, p)
	if err != nil {
		return ""
	}
	var body []string
	in := false
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimRight(line, " \t")
		if heading.MatchString(trimmed) {
			if in {
				break
			}
			in = trimmed == briefHeading
			continue
		}
		if in {
			body = append(body, trimmed)
		}
	}
	for len(body) > 0 && body[0] == "" {
		body = body[1:]
	}
	for len(body) > 0 && body[len(body)-1] == "" {
		body = body[:len(body)-1]
	}
	if len(body) == 0 {
		return ""
	}
	return strings.Join(body, "\n") + "\n"
}

// request is the brief's digest: sha256 over LF-terminated lines, the
// item, the role, the spec commit or "-", then each read's path and blob.
func request(b *Brief) string {
	spec := b.Spec
	if spec == "" {
		spec = "-"
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\n%s\n%s\n", b.Item, b.Role, spec)
	for _, r := range b.Reads {
		fmt.Fprintf(h, "%s %s\n", r.Path, r.Blob)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
