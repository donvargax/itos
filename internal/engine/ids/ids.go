package ids

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/ledger"
	"github.com/donvargax/itos/v8/internal/nextid"
	"github.com/donvargax/itos/v8/internal/store/tracker/port"
	"github.com/donvargax/itos/v8/internal/work"
)

// Items are the ids of the registry's items.
func Items(registry work.Registry) []string {
	ids := make([]string, 0, len(registry.Items))
	for _, item := range registry.Items {
		if id, ok := item.At("id").(string); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// Registry is the work registry's item ids, which name a slice, a bug or a
// task before its scenarios or its ledger entry exist; none when the
// config's registry is not there.
func Registry(ctx context.Context, cfg *config.Loaded) ([]string, error) {
	if !cfg.Source.Has(ctx, cfg.Work.Registry) {
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

// Mint reserves an item number through t before the registry or ledger is
// written. Task IDs use ledger.NextIDAfter, so ledger.id remains the one
// authority for their prefix, padding and accepted width. A slice's or a
// bug's (slice 104) is <kind>-<n>, past the registry's and the feature
// files' tags of its kind (taggedIDs).
func Mint(ctx context.Context, t port.Tracker, cfg *config.Loaded, category string, registryIDs []string) (string, error) {
	var next string
	var ids []string
	switch category {
	case "task":
		var err error
		next, err = ledger.NextID(cfg, registryIDs)
		if err != nil {
			return "", err
		}
	case "slice", "bug":
		var err error
		ids, err = taggedIDs(cfg, category, registryIDs)
		if err != nil {
			return "", err
		}
		next = nextid.Next(category+"-", ids, 1, nil)
	default:
		return "", fmt.Errorf("cannot mint an item of kind %q", category)
	}
	n, err := trailingNumber(next)
	if err != nil {
		return "", fmt.Errorf("cannot read the number from next %s id %q: %w", category, next, err)
	}
	number, err := t.Reserve(ctx, category, n-1, 1)
	if err != nil {
		return "", err
	}
	if category == "task" {
		return ledger.NextIDAfter(cfg, registryIDs, number-1)
	}
	width := 1
	for _, id := range ids {
		if digits, ok := strings.CutPrefix(id, category+"-"); ok && digits != "" {
			allDigits := true
			for _, digit := range digits {
				allDigits = allDigits && digit >= '0' && digit <= '9'
			}
			if allDigits {
				width = max(width, len(digits))
			}
		}
	}
	return fmt.Sprintf("%s-%0*d", category, width, number), nil
}

// Claim claims count numbers of the area stem in a row through t's Reserve,
// its series the stem, the first past the higher of the counter and next's
// number less one (the highest the kind's files hold), and gives each as
// next is written: its tag prefix, the stem and the number, as wide as
// next's.
func Claim(ctx context.Context, t port.Tracker, stem, next string, count int) ([]string, error) {
	n, err := trailingNumber(next)
	if err != nil {
		return nil, fmt.Errorf("cannot read the number of the next %s id %q: %w", stem, next, err)
	}
	head := strings.TrimRight(next, "0123456789")
	width := len(next) - len(head)
	first, err := t.Reserve(ctx, stem, n-1, count)
	if err != nil {
		return nil, err
	}
	tags := make([]string, count)
	for i := range tags {
		tags[i] = fmt.Sprintf("%s%0*d", head, width, first+i)
	}
	return tags, nil
}

// taggedIDs are the ids given and the <kind>-<n> every @<kind>-<n> tag of
// the feature files names: slice or bug.
func taggedIDs(cfg *config.Loaded, kind string, ids []string) ([]string, error) {
	root := "features"
	if scenario, ok := cfg.Tests.Get("scenario"); ok && scenario.Root != nil && *scenario.Root != "" {
		root = *scenario.Root
	}
	found := append([]string(nil), ids...)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".feature" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "@") {
				continue
			}
			for _, word := range strings.Fields(line) {
				if tag, ok := strings.CutPrefix(word, "@"+kind+"-"); ok {
					found = append(found, kind+"-"+tag)
				}
			}
		}
		return nil
	})
	if os.IsNotExist(err) {
		return found, nil
	}
	return found, err
}

func trailingNumber(id string) (int, error) {
	i := len(id)
	for i > 0 && id[i-1] >= '0' && id[i-1] <= '9' {
		i--
	}
	if i == len(id) {
		return 0, fmt.Errorf("no trailing number")
	}
	return strconv.Atoi(id[i:])
}
