// Package ask is the questions of itos decision (features/ask.feature): the
// questions waiting on the person a repository's work is for, each with an
// id, q-1, q-2 and so on, the text asked, the registry item it holds up when
// it names one, and its answer once given. They are public and committed,
// unlike itos followup's threads: one file, work.asks (asks.yaml beside the
// work registry by default, in the git folder under a stealth config),
// written whole by itos, which commits it alone as the registry's commands
// commit theirs (internal/cli/ask.go).
//
// An answered question is a decision, and itos decision record writes it as
// an architecture decision record (internal/adr), noting the record's number
// on the question as its decision, or none for an answer that concerned its
// item alone; one answered with no decision is recorded nowhere yet, and
// itos decision names it.
//
// The file is itos's own, so it is read with typed structs (Parse: unknown
// keys, an id not q-<n> or given twice, a second document and an empty file
// refused, exit 2) and written whole (Text), each text a double-quoted
// scalar written by encoding/json, whose escapes YAML reads the same, so
// what it holds reads back exactly and no formatter has a reason to change
// it. An id is one past the highest the file holds (NextID), so a question
// answered or removed by hand never gives its id again. File.About lists the
// questions naming an item, for itos work show.
package ask
