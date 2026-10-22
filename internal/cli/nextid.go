package cli

// tests next-id and task next-id (slice 60, features/next-id.feature): the
// next free ID for a spec writer, who grepped features/ and the ledger for
// it before. They read the working tree and judge nothing. tests next-id of
// an area (tests.Area) claims what it prints through the id counter (slice
// 105), as the commands that create the registry's items mint theirs, so two
// machines writing specs in one area never take the same number; task
// next-id, and tests next-id of any other stem, only read. testsNextID is
// tests.go's.

import (
	"context"
	"fmt"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/engine/ids"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/out"
)

// registryIDs are the work registry's item ids (ids.Registry).
func registryIDs(cfg *config.Loaded) ([]string, error) {
	return ids.Registry(context.Background(), cfg)
}

// taskNextID is `task next-id`: the ledger's next task ID (ledger.NextID),
// counting the registry's ids that ledger.id matches too.
func taskNextID(args []string, o Out) (int, error) {
	if words := positional(args); len(words) > 0 {
		return 0, usage("task next-id takes no arguments")
	}
	cfg, err := loadConfig()
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
