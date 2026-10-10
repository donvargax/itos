package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v7/internal/config"
	"github.com/donvargax/itos/v7/internal/git"
	"github.com/donvargax/itos/v7/internal/idcounter"
	"github.com/donvargax/itos/v7/internal/ledger"
	"github.com/donvargax/itos/v7/internal/nextid"
	"github.com/donvargax/itos/v7/internal/work"
)

// reserved is the item id a draft reserved for the command itos draft
// promote runs in this process (runReserved), which the command writes
// instead of minting one; "" otherwise. Only promote sets it, for the one
// command it runs: no command line can, so a person passing an id is still
// refused.
var reserved string

// newItemID is the item id a command that mints one writes: the id a draft
// reserved for it, else one minted now.
func newItemID(cfg *config.Loaded, category string, registryIDs []string) (string, error) {
	if reserved != "" {
		return reserved, nil
	}
	return mintItemID(cfg, category, registryIDs)
}

// itemIDs are the ids of the registry's items.
func itemIDs(registry work.Registry) []string {
	ids := make([]string, 0, len(registry.Items))
	for _, item := range registry.Items {
		if id, ok := item.At("id").(string); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// mintItemID reserves an item number before the registry or ledger is
// written. Task IDs use ledger.NextIDAfter, so ledger.id remains the one
// authority for their prefix, padding and accepted width.
func mintItemID(cfg *config.Loaded, category string, registryIDs []string) (string, error) {
	var next string
	switch category {
	case "task":
		var err error
		next, err = ledger.NextID(cfg, registryIDs)
		if err != nil {
			return "", err
		}
	case "slice":
		ids, err := sliceIDs(cfg, registryIDs)
		if err != nil {
			return "", err
		}
		next = nextid.Next("slice-", ids, 1, nil)
	default:
		return "", fmt.Errorf("cannot mint an item of kind %q", category)
	}
	n, err := trailingNumber(next)
	if err != nil {
		return "", fmt.Errorf("cannot read the number from next %s id %q: %w", category, next, err)
	}
	number, err := reserveItemNumber(cfg, category, n-1)
	if err != nil {
		return "", err
	}
	if category == "task" {
		return ledger.NextIDAfter(cfg, registryIDs, number-1)
	}
	ids, err := sliceIDs(cfg, registryIDs)
	if err != nil {
		return "", err
	}
	width := 1
	for _, id := range ids {
		if digits, ok := strings.CutPrefix(id, "slice-"); ok && digits != "" {
			allDigits := true
			for _, digit := range digits {
				allDigits = allDigits && digit >= '0' && digit <= '9'
			}
			if allDigits {
				width = max(width, len(digits))
			}
		}
	}
	return fmt.Sprintf("slice-%0*d", width, number), nil
}

func reserveItemNumber(cfg *config.Loaded, category string, highest int) (int, error) {
	common, err := git.Output("rev-parse", "--git-common-dir")
	if err != nil {
		return 0, err
	}
	common, err = filepath.Abs(strings.TrimSpace(common))
	if err != nil {
		return 0, err
	}
	root, err := os.Getwd()
	if err != nil {
		return 0, err
	}
	remote := ""
	if !cfg.Stealth {
		remote, _, _ = git.Upstream(git.Branch())
		if remote == "" {
			remote = "origin"
		}
		if !git.Succeeds("remote", "get-url", remote) {
			remote = ""
		}
	}
	return idcounter.Mint(idcounter.Store{
		Root: root, CommonDir: common, Remote: remote, LocalOnly: cfg.Stealth,
	}, category, highest)
}

func sliceIDs(cfg *config.Loaded, ids []string) ([]string, error) {
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
				if tag, ok := strings.CutPrefix(word, "@slice-"); ok {
					found = append(found, "slice-"+tag)
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
