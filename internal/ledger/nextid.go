package ledger

import (
	"errors"
	"regexp"
	"strconv"

	"github.com/donvargax/itos/v5/internal/config"
	"github.com/donvargax/itos/v5/internal/nextid"
)

// trailing splits an ID into what comes before its number and the number.
var trailing = regexp.MustCompile(`^(.*\D)(\d+)$`)

// NextID is `task next-id` (slice 60): the ledger's next task ID, one past
// the highest of its tasks' IDs and of others (the work registry's ids,
// since work promote names a task there before the ledger holds it) that
// match ledger.id, written with the ledger's padding and as wide as
// ledger.id asks. The prefix is ledger.id's literal one (T- for T-\d+), or,
// for a pattern that starts otherwise, the highest ID's own.
func NextID(cfg *config.Loaded, others []string) (string, error) {
	tasks, err := Tasks(cfg)
	if err != nil {
		return "", err
	}
	pattern := IDPattern(cfg)
	var ids []string
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	for _, id := range others {
		if pattern.MatchString(id) {
			ids = append(ids, id)
		}
	}
	prefix := ""
	if cfg.Ledger.ID != nil {
		if re, err := regexp.Compile(*cfg.Ledger.ID); err == nil {
			prefix, _ = re.LiteralPrefix()
		}
	}
	if prefix == "" {
		highest := -1
		for _, id := range ids {
			if m := trailing.FindStringSubmatch(id); m != nil {
				if n, err := strconv.Atoi(m[2]); err == nil && n > highest {
					prefix, highest = m[1], n
				}
			}
		}
	}
	if prefix == "" {
		return "", errors.New("ledger.id has no fixed start and the ledger no numbered task: there is no series to continue")
	}
	return nextid.Next(prefix, ids, 1, pattern.MatchString), nil
}
