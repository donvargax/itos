package next

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/donvargax/itos/v8/internal/value"
)

// Revision is "sha256:" and the hex digest of exactly the facts read, one
// line each in a fixed order, each field JSON so no field holds a line
// break: the item's entry as written, each dependency's id and status,
// each open question's id and text, each tagged scenario's id, file and
// whether it is live, and each ledger check. A commit that touches none of
// them leaves it as it was; a change to any gives a new one.
func Revision(f Facts) string {
	var b strings.Builder
	b.WriteString("itos next revision v1\n")
	b.WriteString("item " + value.JSON(f.Item) + "\n")
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
	sum := sha256.Sum256([]byte(b.String()))
	return "sha256:" + hex.EncodeToString(sum[:])
}
