package cli

import (
	"context"
	"strings"
	"time"

	"github.com/donvargax/itos/v8/internal/ask"
	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/slices/decision"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
)

// runDecision is `decision`, the decision slice (internal/slices/decision)
// over its ports bound to the real code: loadConfig, tracker and the clock.
func runDecision(args []string, o Out) (int, error) {
	c, err := decision.Parse(args, o.JSON, o.Quiet)
	if err != nil {
		return 0, err
	}
	return c.Run(context.Background(), o.Stdout, o.Stderr, decision.NewHandler(decisionPorts{o}))
}

// decisionPorts are the decision slice's ports over the real code, the
// tracker on o's OS.
type decisionPorts struct{ o Out }

// Config is loadConfig.
func (decisionPorts) Config(context.Context) (*config.Loaded, error) { return loadConfig() }

// Tracker is tracker.
func (p decisionPorts) Tracker(_ context.Context, cfg *config.Loaded) trackerport.Tracker {
	return tracker(cfg, p.o.OS)
}

// Now is time.Now.
func (decisionPorts) Now(context.Context) time.Time { return time.Now() }

// askEntry is a question as --json gives it (ask.Entry).
type askEntry = ask.Entry

// entryOf is ask.EntryOf.
func entryOf(q ask.Question) askEntry { return ask.EntryOf(q) }

// loadAsks is ask.Load of the config's work.asks.
func loadAsks(cfg *config.Loaded, t trackerport.Tracker) (ask.File, string, bool, error) {
	return ask.Load(context.Background(), t, cfg.Work.Asks)
}

// oneLine is a text on one line, its whitespace each a space, as a list and
// a commit's body give it.
func oneLine(text string) string { return strings.Join(strings.Fields(text), " ") }
