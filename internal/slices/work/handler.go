package work

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"slices"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/ciwatch"
	"github.com/donvargax/itos/v8/internal/engine/evidence"
	"github.com/donvargax/itos/v8/internal/engine/judge"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/out"
	"github.com/donvargax/itos/v8/internal/providers"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	treeport "github.com/donvargax/itos/v8/internal/store/tree/port"
	"github.com/donvargax/itos/v8/internal/tests"
	"github.com/donvargax/itos/v8/internal/value"
	vcsport "github.com/donvargax/itos/v8/internal/vcs/port"
	domain "github.com/donvargax/itos/v8/internal/work"
)

// Ports are what work reaches the world through, which internal/cli binds:
// Config, the config every command finds; Worktree, the working tree, where
// a file work check names is looked for before the config is read;
// Identity, work.identity's provider, asked who the session works for;
// Tracker, the tracker store the questions and the registry are in, which
// a write holds, reads and commits through; ItemID, the id work add
// gives an item of a numbered kind: the one a draft reserved for the
// command, else one minted past registryIDs; and for work done,
// TaskChecks, a task's static checks run in written order through the
// evidence engine on HEAD's tree (Checked), the first failure's output on
// stderr, and Watcher, ci.watch's provider for cfg, for a commit pushed to
// remote. The repository is the config's VCS.
type Ports interface {
	Config(ctx context.Context) (*config.Loaded, error)
	Worktree(ctx context.Context) treeport.Source
	Identity(ctx context.Context, cfg *config.Loaded) providers.Identity
	Tracker(ctx context.Context, cfg *config.Loaded) trackerport.Tracker
	ItemID(ctx context.Context, cfg *config.Loaded, kind string, registryIDs []string) (string, error)
	TaskChecks(ctx context.Context, cfg *config.Loaded, task ledger.Task, stderr io.Writer) (Checked, error)
	Watcher(ctx context.Context, cfg *config.Loaded, remote string) (providers.Watcher, bool, error)
}

// Checked is what running a task's static checks found: Failure, the first
// that failed (nil when none did), and the Evidence of each opted-in check
// looked up, in run order.
type Checked struct {
	Failure  *judge.Failure
	Evidence []evidence.Entry
}

// Handler is work's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// Out is what Handle is told of how a change is reported: Quiet, which
// silences the commit's words, and Stderr, where git's words and the
// hooks' go, and the line of a lock that cannot be given back; work done
// writes the watch of its CI run as ciwatch.Out says, to Stdout, or Stderr
// under JSON, and its notes and the task's failing checks to Stderr.
type Out struct {
	Quiet, JSON    bool
	Stdout, Stderr io.Writer
}

// Answer is what Handle found or did. work check: the File judged and the
// Found problems. The proposal: Found, the problems of a registry that is
// not sound; else the Notes said on stderr of who the session is, Missing
// when --as is not among the people, and the Proposal. work list: Found, a
// registry that is not there; else the File read and the Items listed.
// work show: Refused, a registry that is not there or an id no item has;
// else the Shown item. A write: Release, nil until the registry's lock is
// held, then for the caller to defer; Refused, the problems that refused
// it, nothing written, Code its exit and Each the ids refused of several;
// else the Change of one id, made for ID (work add's minted one), the
// Several of more, what work promote Promoted or what work done Done, and
// the Commit, "" under a stealth config or when nothing changed. work done
// refused by its CI run is CI, the watch, Code its exit; Evidence is that of
// the task checks it looked up, which its --json gives.
type Answer struct {
	File     string
	Found    []out.Problem
	Notes    []string
	Missing  bool
	Proposal domain.Proposal
	Items    []*value.Map
	Refused  []out.Problem
	Shown    *Shown

	Release  func()
	Code     int
	Each     []string
	ID       string
	Change   *domain.Change
	Several  *domain.Several
	Promoted *Promoted
	Done     *Closed
	CI       *ciwatch.Watched
	Evidence []evidence.Entry
	Commit   string
}

// Shown is work show's item: the Item, whether it is a Task of the ledger,
// its Scenarios, the Questions naming it and its Commits; with --patch,
// under --json each commit's Patches, its diff, else Shows, git show's
// text of them all, or ShowsErr, why it could not be read, which comes
// after the commits are listed.
type Shown struct {
	Item      domain.Shown
	Task      bool
	Scenarios []tests.Test
	Questions []ask.Question
	Commits   []message.Logged
	Patches   []string
	Shows     string
	ShowsErr  error
}

// Handle answers q, a write reporting as o says.
func (h Handler) Handle(ctx context.Context, q Query, o Out) (Answer, error) {
	switch q.Sub {
	case "check":
		return h.check(ctx, q.File, q.Named)
	case "list":
		return h.list(ctx, q.All, q.Tag)
	case "show":
		return h.show(ctx, q)
	case "take":
		return h.take(ctx, q.ID, q.As, o)
	case "add":
		return h.add(ctx, q.New, o)
	case "promote":
		return h.promote(ctx, q.Promotion, o)
	case "done":
		return h.done(ctx, q.ID, o)
	case "edit", "queue", "drop", "defer", "resume", "give-back":
		if len(q.IDs) > 1 {
			return h.each(ctx, q, o)
		}
		return h.one(ctx, q, o)
	}
	return h.proposal(ctx, q.As)
}

// check is `work check [<file>]` (work.ts's workCheck): the registry's
// problems, the config's work.registry or the file named. A named file that
// is not there is a problem found before the config is read, as the
// TypeScript reads it only to load the registry.
func (h Handler) check(ctx context.Context, path string, named bool) (Answer, error) {
	a, _, err := h.judge(ctx, path, named)
	return a, err
}

// judge is check, with the config when it was read.
func (h Handler) judge(ctx context.Context, path string, named bool) (Answer, *config.Loaded, error) {
	if named && !h.ports.Worktree(ctx).Has(ctx, path) {
		return Answer{File: path, Found: []out.Problem{domain.Missing(path, true)}}, nil, nil
	}
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, nil, err
	}
	file := path
	if !named {
		file = cfg.Work.Registry
	}
	found, err := domain.ProblemsAt(cfg, file, named)
	if err != nil {
		return Answer{}, nil, err
	}
	return Answer{File: file, Found: found}, cfg, nil
}

// proposal is `work [--as <handle>]` (work.ts's work): what the person can
// start, once the registry is sound. An identity provider that cannot say
// who the session is, or names someone the people do not list, is not an
// error: the session is nobody, or owns nothing yet, and a note says so;
// with --as the first is Missing. The provider is asked only when the
// people list several (bug 46): one person is the session, and with no one
// in the people (a people file missing, unreadable or listing nobody) the
// session is nobody and nothing is said of them. Under a stealth config
// with no --as nobody is asked (slice 38): the person is the only one, so
// the session owns every item, whatever owner it names; --as still
// proposes that handle's, as in a project.
func (h Handler) proposal(ctx context.Context, as string) (Answer, error) {
	a, cfg, err := h.judge(ctx, "", false)
	if err != nil || len(a.Found) > 0 {
		return a, err
	}
	registry, err := domain.Load(cfg, cfg.Work.Registry)
	if err != nil {
		return Answer{}, err
	}
	if cfg.Stealth && as == "" {
		return Answer{Proposal: domain.ProposeEvery(registry)}, nil
	}
	listedIn := cfg.Work.People.File
	who := domain.Whoami(registry, listedIn, as, h.ports.Identity(ctx, cfg))
	var notes []string
	switch {
	case who.Problem != "":
		notes = append(notes, who.Problem)
		if as != "" {
			return Answer{Notes: notes, Missing: true}, nil
		}
	case who.Handle != "" && !who.Listed:
		notes = append(notes, fmt.Sprintf("%s is not in %s: nothing is theirs yet", who.Handle, listedIn))
	}
	return Answer{Notes: notes, Proposal: domain.Propose(registry, who.Handle)}, nil
}

// list is `work list` (slice 43), the open items of the registry (every one
// neither done nor dropped, slice 79 and bug 25) in its order, whatever
// their kind or owner; with all (slice 77) every item, done and dropped
// ones too; with a tag (slice 97) only those carrying it. It judges
// nothing, as task list does not, so a registry with problems still lists;
// a registry that is not there is the one problem it finds.
func (h Handler) list(ctx context.Context, all bool, tag string) (Answer, error) {
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	file := cfg.Work.Registry
	if !cfg.Source.Has(ctx, file) {
		return Answer{Found: []out.Problem{domain.Missing(file, false)}}, nil
	}
	registry, err := domain.Load(cfg, file)
	if err != nil {
		return Answer{}, err
	}
	listed := registry.Items
	if !all {
		listed = domain.Open(listed)
	}
	if tag != "" {
		listed = domain.Tagged(listed, tag)
	}
	return Answer{File: file, Items: listed}, nil
}

// show is `work show <id> [--patch]`: the item, its scenarios, the
// questions naming it and its commits. A registry that is not there, or no
// item with the id, is refused; the registry is not judged otherwise, as
// work list does not judge it.
func (h Handler) show(ctx context.Context, q Query) (Answer, error) {
	id := q.ID
	cfg, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	file := cfg.Work.Registry
	if !cfg.Source.Has(ctx, file) {
		return Answer{Refused: []out.Problem{domain.Missing(file, false)}}, nil
	}
	registry, err := domain.Load(cfg, file)
	if err != nil {
		return Answer{}, err
	}
	shown, problem := domain.Show(registry, id)
	if problem != nil {
		return Answer{Refused: []out.Problem{*problem}}, nil
	}
	task := domain.SpecKind(shown.Item, taskIDs(cfg)) == "task"
	if task {
		shown.LedgerWhy = ledgerWhy(cfg, id)
	}
	hasHead := hasCommit(ctx, cfg.VCS, "HEAD")
	at := "HEAD"
	if !hasHead {
		at = "worktree"
	}
	scenarios, err := tests.Tagged(cfg, "@"+id, at)
	if err != nil {
		return Answer{}, err
	}
	var commits []message.Logged
	if hasHead {
		history, err := message.History(cfg, "HEAD")
		if err != nil {
			return Answer{}, err
		}
		commits = belonging(cfg, id, scenarios, history)
	}
	asks, _, _, err := ask.Load(ctx, h.ports.Tracker(ctx, cfg), cfg.Work.Asks)
	if err != nil {
		return Answer{}, err
	}
	s := &Shown{Item: shown, Task: task, Scenarios: scenarios, Questions: asks.About(id), Commits: commits}
	if !q.Patch || len(commits) == 0 {
		return Answer{Shown: s}, nil
	}
	if q.JSON {
		for _, c := range commits {
			diff, err := cfg.VCS.Patch(ctx, c.SHA)
			if err != nil {
				return Answer{}, err
			}
			s.Patches = append(s.Patches, diff)
		}
		return Answer{Shown: s}, nil
	}
	shas := make([]string, len(commits))
	for i, c := range commits {
		shas[i] = c.SHA
	}
	// The itos notes alone, under a stealth config, where a commit's links
	// are.
	s.Shows, s.ShowsErr = cfg.VCS.Shown(ctx, shas, cfg.Stealth)
	return Answer{Shown: s}, nil
}

// belonging are the commits of the history that belong to the item, oldest
// first: a registry commit of itos's naming it in its header, or one whose
// footers link to it or to one of its scenarios.
func belonging(cfg *config.Loaded, id string, scenarios []tests.Test, history []message.Logged) []message.Logged {
	links := linksTo(cfg, id, scenarios)
	var found []message.Logged
	for _, c := range history {
		named, ok := domain.RegistryHeader(c.Header)
		if (ok && named == id) || slices.ContainsFunc(c.Links, links) {
			found = append(found, c)
		}
	}
	return found
}

// linksTo is whether a footer's link names the item: its id, in a footer of
// IDs (Task:, Item:), or one of its scenarios (Scenarios:).
func linksTo(cfg *config.Loaded, id string, scenarios []tests.Test) func(message.Link) bool {
	tagged := map[string]bool{}
	for _, t := range scenarios {
		tagged[t.ID] = true
	}
	return func(l message.Link) bool {
		if l.Tests == "" {
			return l.ID == id
		}
		k, _ := cfg.Tests.Get(l.Tests)
		return tagged[l.ID] || tagged[k.TagPrefix+l.ID]
	}
}

// taskIDs is ledger.id as a pattern, nil when the config has none: an item
// of no kind is a task by its id only where the ledger says what a task's id
// is (work.SpecKind).
func taskIDs(cfg *config.Loaded) *regexp.Regexp {
	if cfg.Ledger.ID == nil {
		return nil
	}
	return ledger.IDPattern(cfg)
}

// ledgerWhy is the why of the ledger's task with the id (slice 76: a task's
// reasons live in its ledger entry), "" when the ledger has no such task, or
// none to read: work show reads, never judges, so a ledger that cannot be
// read is config check's to report.
func ledgerWhy(cfg *config.Loaded, id string) string {
	tasks, err := ledger.Tasks(cfg)
	if err != nil {
		return ""
	}
	for _, t := range tasks {
		if t.ID == id {
			return t.Why
		}
	}
	return ""
}

// hasCommit is whether the repository has the commit rev names.
func hasCommit(ctx context.Context, v vcsport.VCS, rev string) bool {
	_, err := v.Resolve(ctx, rev)
	return err == nil
}

// itos-cc mutation: 54 killed, 0 survived, 0 uncovered
// end itos-cc mutation
