package ledger

import (
	"strconv"
	"strings"

	"github.com/donvargax/itos/v8/internal/out"
)

// CheckArg reads one --check or --timeout of a command that writes a
// task's checks, task add or work promote (cmd, as its usage errors name
// it): a --check's command, never blank, is the next check; a --timeout,
// once after a --check, is that check's seconds, a number above 0. timed is
// whether the last check has its timeout.
func CheckArg(cmd string, checks *[]NewCheck, timed *bool, name, v string) error {
	if name == "--check" {
		if strings.TrimSpace(v) == "" {
			return out.Usage("%s --check needs a command", cmd)
		}
		*checks, *timed = append(*checks, NewCheck{Run: v}), false
		return nil
	}
	seconds, err := strconv.ParseFloat(v, 64)
	if len(*checks) == 0 || *timed || err != nil || seconds <= 0 {
		return out.Usage("%s takes one --timeout <seconds> after a --check, a number above 0, for that check", cmd)
	}
	(*checks)[len(*checks)-1].Timeout, *timed = &seconds, true
	return nil
}
