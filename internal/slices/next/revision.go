package next

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/donvargax/itos/v8/internal/config"
	"github.com/donvargax/itos/v8/internal/kind"
	"github.com/donvargax/itos/v8/internal/out"
	trackerport "github.com/donvargax/itos/v8/internal/store/tracker/port"
	"github.com/donvargax/itos/v8/internal/value"
)

// RevisionOf is the item's revision as itos next gives it (Revision over
// the facts read as Handle reads them, the forge asked as next asks it),
// for a mutating command handed one back with --expect (slice 208, work
// take and work done): refused is the problem of a registry that is not
// there or an id no item has, as next refuses it, with no revision; what
// cannot be read is the error, and a forge that cannot be asked one of kind
// Missing, its problem's message.
func RevisionOf(ctx context.Context, cfg *config.Loaded, tracker trackerport.Tracker, forge Forge, id string) (revision string, refused *out.Problem, err error) {
	facts, refused, failed, err := read(ctx, cfg, tracker, forge, id)
	switch {
	case err != nil || refused != nil:
		return "", refused, err
	case failed != nil:
		return "", nil, kind.Wrap(kind.Missing, errors.New(failed.Message))
	}
	return Revision(facts), nil, nil
}

// Revision is "sha256:" and the hex digest of exactly the facts read, one
// line each in a fixed order, each field JSON so no field holds a line
// break: the item's entry as written, its kind as read (by its id and the
// config's ledger.id when the entry gives none, so a config change that
// moves the step moves the revision), each dependency's id and status,
// each open question's id and text, each tagged scenario's id, file and
// whether it is live, and each ledger check; under push.mode pr the ref the
// scenarios were read at and, when the forge was asked, what it and the
// journal said and each question naming the item with its status. A commit
// that touches none of them leaves it as it was; a change to any gives a
// new one.
func Revision(f Facts) string {
	var b strings.Builder
	b.WriteString("itos next revision v1\n")
	b.WriteString("item " + value.JSON(f.Item) + "\n")
	b.WriteString("kind " + value.JSON(f.Kind) + "\n")
	for _, d := range f.Dependencies {
		b.WriteString("dependency " + value.JSON(d.ID) + " " + value.JSON(d.Status) + "\n")
	}
	for _, q := range f.Questions {
		b.WriteString("question " + value.JSON(q.ID) + " " + value.JSON(q.Question) + "\n")
	}
	for _, s := range f.Scenarios {
		b.WriteString("scenario " + value.JSON(s.ID) + " " + value.JSON(s.File) + " " + value.JSON(s.Live) + "\n")
	}
	for _, c := range f.Checks {
		// A check's fields are exported texts, numbers and booleans, which
		// encoding/json always writes.
		text, _ := json.Marshal(c)
		b.WriteString("check " + string(text) + "\n")
	}
	if f.SpecRef != "" {
		b.WriteString("ref " + value.JSON(f.SpecRef) + "\n")
	}
	if o := f.Observed; o != nil {
		pull, _ := json.Marshal(o.Pull)
		b.WriteString("pull " + string(pull) + "\n")
		b.WriteString("verdict " + value.JSON(o.Verdict) + "\n")
		b.WriteString("checks " + value.JSON(o.Checks) + "\n")
		b.WriteString("threads_open " + strconv.Itoa(o.ThreadsOpen) + "\n")
		for _, r := range o.Records {
			record, _ := json.Marshal(r)
			b.WriteString("record " + string(record) + "\n")
		}
		for _, q := range f.Asked {
			b.WriteString("asked " + value.JSON(q.ID) + " " + value.JSON(q.Status()) + "\n")
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	return "sha256:" + hex.EncodeToString(sum[:])
}
