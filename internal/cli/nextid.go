package cli

// tests next-id and task next-id (slice 60, features/next-id.feature): the
// next free ID for a spec writer, who grepped features/ and the ledger for
// it before. They read the working tree and judge nothing. tests next-id of
// an area (tests.Area) claims what it prints through the id counter (slice
// 105), as the commands that create the registry's items mint theirs, so two
// machines writing specs in one area never take the same number; task
// next-id, and tests next-id of any other stem, only read.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/idcounter"
	"github.com/donvargax/itos/v7/internal/ledger"
	"github.com/donvargax/itos/v7/internal/out"
	"github.com/donvargax/itos/v7/internal/source"
	"github.com/donvargax/itos/v7/internal/tests"
	"github.com/donvargax/itos/v7/internal/work"
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

// testsNextID is `tests next-id <kind> <stem> [--count <n>]`: the kind's
// next free tag of the stem (tests.NextTag), counting the registry's ids
// too. Of an area it claims the tag, or n tags in a row, through the id
// counter (claimTags), each printed; of any other stem it only reads, so
// --count there is a usage error, as is a count that is no number from 1 up
// and a kind the config does not have.
func testsNextID(args []string, o Out) (int, error) {
	words := positional(args, "--count")
	if len(words) != 2 {
		return 0, usage("tests next-id needs <kind> <stem>")
	}
	name, stem := words[0], words[1]
	text, counted := flagValue(args, "--count")
	count, err := strconv.Atoi(text)
	if !counted {
		count, err = 1, nil
	}
	if err != nil || count < 1 {
		return 0, usage("tests next-id --count takes how many ids to claim, a number from 1 up, not %q", text)
	}
	cfg, err := config.Load(config.Path())
	if err != nil {
		return 0, err
	}
	k, ok := cfg.Tests.Get(name)
	if !ok {
		return 0, usage("tests next-id: the config has no tests kind %s (it has %s)", name, kindNames(cfg))
	}
	area := tests.Area(k, stem)
	if counted && !area {
		return 0, usage("tests next-id --count claims an area's ids, and %s is no area of tests.%s's id: its next id is only read", stem, name)
	}
	others, err := registryIDs(cfg)
	if err != nil {
		return 0, err
	}
	next, err := tests.NextTag(cfg, name, stem, others)
	if err != nil {
		return 0, err
	}
	ids := []string{next}
	if area {
		if ids, err = claimTags(cfg, stem, next, count); err != nil {
			return 0, err
		}
	}
	if o.JSON {
		return 0, out.Emit(o.Stdout,
			out.Field{Key: "kind", Value: name},
			out.Field{Key: "stem", Value: stem},
			out.Field{Key: "id", Value: ids[0]},
			out.Field{Key: "ids", Value: ids},
			out.Field{Key: "claimed", Value: area})
	}
	fmt.Fprintln(o.Stdout, strings.Join(ids, "\n"))
	return 0, nil
}

// claimTags claims count numbers of the area stem in a row through the id
// counter, its category the stem, the first past the higher of the counter
// and next's number less one (the highest the kind's files hold), and gives
// each as next is written: its tag prefix, the stem and the number, as wide
// as next's.
func claimTags(cfg *config.Loaded, stem, next string, count int) ([]string, error) {
	n, err := trailingNumber(next)
	if err != nil {
		return nil, fmt.Errorf("cannot read the number of the next %s id %q: %w", stem, next, err)
	}
	head := strings.TrimRight(next, "0123456789")
	width := len(next) - len(head)
	store, err := counterStore(cfg)
	if err != nil {
		return nil, err
	}
	first, err := idcounter.Claim(store, stem, n-1, count)
	if err != nil {
		return nil, err
	}
	tags := make([]string, count)
	for i := range tags {
		tags[i] = fmt.Sprintf("%s%0*d", head, width, first+i)
	}
	return tags, nil
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
