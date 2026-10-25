package upgrade

import (
	"context"
	"fmt"
	"os"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/agentrules"
	pinengine "github.com/donvargax/itos/v8/internal/engine/pin"
	"github.com/donvargax/itos/v8/internal/release"
	"github.com/donvargax/itos/v8/internal/version"
)

// Query is what upgrade asks: the version to move to, Want, "" for the
// newest.
type Query struct {
	Want string
}

// Ports are what upgrade reaches the world through, which internal/cli
// binds: Config, the config itos finds (the stealth one included) and its
// pin; Release, the release to move to, the version wanted or with "" the
// newest, and its checksums.txt, its errors of their kind
// (pinengine.Pinned); Fetch, the body at a release's address, its errors
// of their kind and a 404 one release.NotFound tells (release.Get);
// Stealth, whether the config is a stealth one; Rules, each stale block of
// the rules for agents rewritten in this itos's words
// (agentrules.Refreshed); Write, a file's new text, its mode kept.
type Ports interface {
	Config(ctx context.Context) (pinengine.Config, error)
	Release(ctx context.Context, want string) (string, []byte, error)
	Fetch(ctx context.Context, url string) ([]byte, error)
	Stealth(ctx context.Context, file string) bool
	Rules(ctx context.Context, stealth bool) []agentrules.Edit
	Write(ctx context.Context, file, text string) error
}

// Handler is upgrade's application, over its ports.
type Handler struct {
	ports Ports
}

// NewHandler is the handler over p.
func NewHandler(p Ports) Handler {
	return Handler{ports: p}
}

// The actions upgrade takes, as --json names them.
const (
	upgraded = "upgraded"
	already  = "already"
	refused  = "refused"
)

// Answer is what Handle did: the Action taken, the version left, From, and
// the one moved to, To; what each release between asks, Releases, oldest
// first; the files Edited; the config edits Applied; for a refusal, Why;
// or Unfetched, a release or what it asks that cannot be fetched, every
// file untouched.
type Answer struct {
	Action    string
	From, To  string
	Releases  []any
	Edited    []string
	Applied   []appliedStep
	Why       string
	Unfetched error

	// Uses and Problems are the workflow steps that launch the itos action
	// and the workflows that could not be read (slice 150): both empty for
	// a refusal and under a stealth config.
	Uses     []actionUse
	Problems []scanProblem
	Pinned   bool
}

// refuse is a refusal, saying why: nothing is upgraded.
func (a Answer) refuse(format string, args ...any) (Answer, error) {
	a.Action, a.Why = refused, fmt.Sprintf(format, args...)
	return a, nil
}

// Handle moves the project to the release q wants, from its pin's version
// or with none its install script's, every edit made in memory before a
// file is written. A move back, or a project with neither, is refused; one
// already on the release changes nothing, or is refused when its pin holds
// other checksums: the release changed after it was pinned.
func (h Handler) Handle(ctx context.Context, q Query) (Answer, error) {
	c, err := h.ports.Config(ctx)
	if err != nil {
		return Answer{}, err
	}
	script, hasScript, err := pinengine.ReadInstallScript(c.Dir)
	if err != nil {
		return Answer{}, err
	}
	a := Answer{To: q.Want, Releases: []any{}, Edited: []string{}, Applied: []appliedStep{},
		Uses: []actionUse{}, Problems: []scanProblem{}}
	hasPin := config.PinVersion.MatchString(c.Version)
	a.Pinned = hasPin
	switch {
	case hasPin:
		a.From = c.Version
	case hasScript:
		if a.From, err = pinengine.ScriptVersion(script); err != nil {
			return Answer{}, fmt.Errorf("cannot read %s: %w; every file is left as it was", pinengine.InstallScript, err)
		}
	default:
		return a.refuse("%s pins no itos, and there is no %s whose version to move: nothing to upgrade; "+
			"itos pin pins a release", c.File, pinengine.InstallScript)
	}
	if q.Want != "" && version.Compare(q.Want, a.From) < 0 {
		return a.refuse("itos %s is older than %s, the version this project is on: itos upgrade only moves "+
			"forward; itos pin %s moves it back", q.Want, a.From, q.Want)
	}
	v, sums, err := h.ports.Release(ctx, q.Want)
	if err != nil {
		a.Unfetched = err
		return a, nil
	}
	a.To = v
	sum := release.SHA256(sums)
	if version.Compare(v, a.From) < 0 {
		return a.refuse("the newest itos, %s, is older than %s, the version this project is on: nothing to "+
			"upgrade; itos pin %s moves it back", v, a.From, v)
	}
	if v == a.From {
		if hasPin && c.Replaced(v, sum) {
			return a.refuse("%s pins itos %s with the checksums %s, but its checksums.txt is %s now: the release "+
				"changed after it was pinned; every file is left as it was", c.File, v, c.Checksums, sum)
		}
		a.Action = already
		a.Uses, a.Problems = workflowsOf(c, h.ports.Stealth(ctx, c.File))
		return a, nil
	}
	if a.Releases, err = h.walkReleases(ctx, a.From, v); err != nil {
		a.Unfetched = err
		return a, nil
	}

	text, applied, err := applySteps(c.File, c.Text, a.Releases)
	if err != nil {
		return Answer{}, err
	}
	a.Applied = applied
	if hasPin {
		if text, err = c.Moved(text, v, sum); err != nil {
			return Answer{}, err
		}
	}
	text = moveSchemaLine(text, v)
	newScript := script
	if hasScript {
		if newScript, err = moveInstallScript(script, v, sums); err != nil {
			return Answer{}, fmt.Errorf("cannot edit %s: %w; every file is left as it was", pinengine.InstallScript, err)
		}
	}
	stealth := h.ports.Stealth(ctx, c.File)
	rules := h.ports.Rules(ctx, stealth)
	a.Uses, a.Problems = workflowsOf(c, stealth)
	if text != c.Text {
		if err := h.ports.Write(ctx, c.File, text); err != nil {
			return Answer{}, err
		}
		a.Edited = append(a.Edited, c.File)
	}
	if newScript != script {
		if err := h.ports.Write(ctx, pinengine.InstallScript, newScript); err != nil {
			return Answer{}, err
		}
		a.Edited = append(a.Edited, pinengine.InstallScript)
	}
	for _, e := range rules {
		if err := e.Write(); err != nil {
			return Answer{}, err
		}
		a.Edited = append(a.Edited, e.Path)
	}
	a.Action = upgraded
	return a, nil
}

// workflowsOf names the workflow steps that launch the itos action in the
// folder itos runs in, none under a stealth config: it is never committed,
// so no workflow reads its pin.
func workflowsOf(c pinengine.Config, stealth bool) ([]actionUse, []scanProblem) {
	if stealth {
		return []actionUse{}, []scanProblem{}
	}
	dir := c.Dir
	if dir == "" {
		dir = "."
	}
	return scanWorkflows(os.DirFS(dir))
}
