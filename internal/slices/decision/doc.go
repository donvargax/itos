// Package decision is the decision command (slice 62, features/ask.feature),
// a slice (ADR-0053) with no domain of its own: the questions waiting on the
// person a repository's work is for, in work.asks beside the registry
// (internal/ask), and the architecture decision records their answers
// become (internal/adr). Its CLI and Run are the command line, its Query and
// Handler the application, over the Ports it owns, which internal/cli binds:
// the config, the tracker store and the clock.
//
// Command (command.go) declares its name, help and flag spec; every error
// Parse gives is a usage error, before anything is read. add and answer
// write the questions through the record engine (internal/engine/record),
// the file alone, "docs: ask q-<n>" and "docs: answer q-<n>", refused while
// the file has changes no commit holds (asks-file-uncommitted); add --item
// reads the registry, unjudged, for the id. Under a stealth config the file
// is in the git folder beside the stealth registry, written under the
// registry's lock (the tracker's Hold), which every stealth writer holds
// (bug 16), given back after Run has printed, and nothing is committed. show
// and the list read, never write; the list ends naming the answered
// questions recorded nowhere, each as itos decision record would record it.
//
// record (slice 69) writes an answered question as the next architecture
// decision record, in MADR 4's format since slice 71, in the folder
// work.decisions names: Considered Options from --option, which repeated
// takes out of the arguments before the args engine reads the rest; a
// supersede sets the old record's status and the new one names it in More
// Information, and is refused, nothing written, for a record already
// superseded (ask-superseded). The question gains decision: <n>, and the questions, the new
// record, the one it supersedes and the folder's index are committed
// together, "docs: record q-<n> as decision <m>", each written naming its
// own rule for changes no commit holds (decision-file-uncommitted,
// decision-index-uncommitted); the folders made for the record are removed
// again when the write fails (madeDirs, unmake). --none notes no record and
// commits the questions alone, "docs: mark q-<n> as recorded nowhere".
// Under a stealth config the records are in the git folder too, decisions/
// beside the stealth config (work.decisions resolved there), and nothing is
// committed.
package decision
