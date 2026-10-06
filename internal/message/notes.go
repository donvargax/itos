package message

// The stealth mode's footers (slice 32, features/stealth.feature). A footer
// in a commit's message shows to everyone who reads the history, so under a
// stealth config a commit's footers live in a git note on it, in
// refs/notes/itos, which git push does not send unless asked. itos commit
// writes the note from its flags and hands the same lines to the commit-msg
// hook in FootersEnv, since git passes its environment to its hooks; every
// reader of a made commit's footers (verify, ci plan's named tasks, the
// footer list) reads its note instead of its message, and the footer rules
// refuse a footer typed into the message. A project's config keeps its
// footers in the message, as it always has.

import (
	"fmt"
	"strings"

	"github.com/donvargax/itos/v6/internal/config"
	"github.com/donvargax/itos/v6/internal/git"
)

// NotesRef is where the stealth mode keeps a commit's footers.
const NotesRef = "refs/notes/itos"

// FootersEnv is the variable itos commit hands the commit-msg hook the
// footers in, a line each, under a stealth config.
const FootersEnv = "ITOS_FOOTERS"

// showNotes are git log's options that print only the itos notes as %N.
var showNotes = []string{"--no-notes", "--notes=" + NotesRef}

// Note is a commit's itos note, "" when it has none.
func Note(sha string) (string, error) {
	return git.Read(append(append([]string{"log", "-1"}, showNotes...), "--format=%N", sha)...)
}

// footersFormat is how a range log prints each commit's footers for a
// config: its note under a stealth config, else its message.
func footersFormat(cfg *config.Loaded) []string {
	if cfg.Stealth {
		return append(append([]string{}, showNotes...), "--format=%N")
	}
	return []string{"--format=%B"}
}

// typedFooter is why a footer of the config written in the message is
// refused under a stealth config, "" when the message has none of it.
func typedFooter(cfg *config.Loaded, key, message string) string {
	f, _ := cfg.Commits.Footers.Get(key)
	if len(Texts(message, key)) == 0 {
		return ""
	}
	why := fmt.Sprintf("the %s: footer is in the message, where everyone would see it", key)
	if flag := footerFlag(cfg, key, f); flag != "" {
		return why + "; in stealth mode itos commit " + flag + " writes it in the commit's note"
	}
	return why + "; in stealth mode a commit's footers live in its note"
}

// footerFlag is the flag of itos commit that writes a footer, "" when none
// does: --task the first of the ledger, --item the first of the work
// registry, --scenarios the first of a kind of named tests.
func footerFlag(cfg *config.Loaded, key string, f config.Footer) string {
	if k, ok := LedgerFooter(cfg); ok && k == key {
		return "--task"
	}
	if k, ok := RegistryFooter(cfg); ok && k == key {
		return "--item"
	}
	if k, ok := TestsFooter(cfg); ok && k == key {
		return "--scenarios"
	}
	return ""
}

// stealthNeed is the end of a missing footer's sentence under a stealth
// config: the itos commit flag that writes it in the note, when one does.
func stealthNeed(cfg *config.Loaded, key string, f config.Footer) string {
	if !cfg.Stealth {
		return ""
	}
	if flag := footerFlag(cfg, key, f); flag != "" {
		return ", which in stealth mode itos commit " + flag + " writes in the commit's note"
	}
	return ""
}

// stealthFix is a footer rule's fix under a stealth config, by what its
// sentence says; "" when the project's fix applies.
func stealthFix(cfg *config.Loaded, key string, f config.Footer, message string) string {
	flag := footerFlag(cfg, key, f)
	if flag == "" {
		return ""
	}
	what := "<id>"
	if flag == "--scenarios" {
		what = "<ids>"
	}
	switch {
	case strings.Contains(message, " is in the message"):
		return "take the footer out of the message and commit with itos commit " + flag + " " + what
	case needs.MatchString(message):
		return "commit with itos commit " + flag + " " + what + ", which writes the footer in the commit's note"
	}
	return ""
}
