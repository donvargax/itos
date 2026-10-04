package cli

// tests next-id and task next-id (slice 60, features/next-id.feature): the
// next free ID for a spec writer, who grepped features/ and the ledger for
// it before. Help only: they read the working tree, write nothing, judge
// nothing and add no rule.

import (
	"fmt"
	"strings"

	"github.com/donvargax/itos/v2/internal/config"
	"github.com/donvargax/itos/v2/internal/ledger"
	"github.com/donvargax/itos/v2/internal/out"
	"github.com/donvargax/itos/v2/internal/source"
	"github.com/donvargax/itos/v2/internal/tests"
	"github.com/donvargax/itos/v2/internal/work"
)

// registryIDs are the work registry's item ids, which name a slice, a bug
// or a task before its scenarios or its ledger entry exist; none when the
// config's registry is not there.
func registryIDs(cfg *config.Loaded) ([]string, error) {
	if !source.Has(cfg.Work.Registry) {
		return nil, nil
	}
	registry, err := work.Load(cfg, cfg.Work.Registry)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, item := range registry.Items {
		if id, ok := item.At("id").(string); ok {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// testsNextID is `tests next-id <kind> <stem>`: the kind's next free tag of
// the stem (tests.NextTag), counting the registry's ids too. A kind the
// config does not have is a usage error naming it.
func testsNextID(args []string, o Out) (int, error) {
	words := positional(args)
	if len(words) != 2 {
		return 0, usage("tests next-id needs <kind> <stem>")
	}
	name, stem := words[0], words[1]
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	if _, ok := cfg.Tests.Get(name); !ok {
		return 0, usage("tests next-id: the config has no tests kind %s (it has %s)", name, kindNames(cfg))
	}
	others, err := registryIDs(cfg)
	if err != nil {
		return 0, err
	}
	next, err := tests.NextTag(cfg, name, stem, others)
	if err != nil {
		return 0, err
	}
	if o.JSON {
		return 0, out.Emit(o.Stdout,
			out.Field{Key: "kind", Value: name},
			out.Field{Key: "stem", Value: stem},
			out.Field{Key: "id", Value: next})
	}
	fmt.Fprintln(o.Stdout, next)
	return 0, nil
}

// kindNames are the config's tests kinds, as a usage error lists them.
func kindNames(cfg *config.Loaded) string {
	if len(cfg.Tests.Keys) == 0 {
		return "none"
	}
	return strings.Join(cfg.Tests.Keys, ", ")
}

// taskNextID is `task next-id`: the ledger's next task ID (ledger.NextID),
// counting the registry's ids that ledger.id matches too.
func taskNextID(args []string, o Out) (int, error) {
	if words := positional(args); len(words) > 0 {
		return 0, usage("task next-id takes no arguments")
	}
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	others, err := registryIDs(cfg)
	if err != nil {
		return 0, err
	}
	next, err := ledger.NextID(cfg, others)
	if err != nil {
		return 0, err
	}
	if o.JSON {
		return 0, out.Emit(o.Stdout, out.Field{Key: "id", Value: next})
	}
	fmt.Fprintln(o.Stdout, next)
	return 0, nil
}
