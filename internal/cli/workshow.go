package cli

// work show is the work slice's (internal/slices/work); what work done and
// work edit read as it does stays here.

import (
	"regexp"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/message"
	"github.com/donvargax/itos/v8/internal/tests"
)

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
