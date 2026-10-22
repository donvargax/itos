// Package draft is the draft command, a slice (ADR-0053): the coordinator's
// pending changes, kept while an agent holds the checkout and promoted once
// no work is going on (slice 96, features/draft.feature). Its CLI and Run
// are the command line, its Query and Handler the application. The list is
// drafts.yaml in the drafts folder of itos's folder under the git common
// dir (internal/draft, imported as domain), each change's patch beside it;
// add, edit, drop and promote hold the list's lock, the private store's
// Hold of it, from before they read it to after they write it (as the
// followup slice's Handle, bug 16).
//
// Command (command.go) declares its name, help and flag spec, by which
// internal/cli reads the line and hands Parse the arguments after draft.
// Parse judges them all; Handle reaches the world through Ports, which the
// slice owns and internal/cli binds: the VCS port, the private store's
// lock, git's patches (which the VCS port leaves to its adapter), the
// hooks, this itos's binary and the code of the commands that mint an
// item's id.
//
// A change is drafted against HEAD through a scratch index (takeChange):
// HEAD read into it, the paths added to it as the working tree has them
// (git add --all, new files and deletions too, pathspecs literal), and the
// difference written as a binary git patch (git diff --cached --binary
// --full-index --no-renames HEAD); then putBack puts those paths, and only
// those, back as HEAD has them, a file HEAD lacks taken out of the index
// and removed. promote first refuses while checkoutBusy finds the checkout
// unclean (a tracked change, a rebase or a merge in progress), never
// reading the registry: an item's status is no gate, since items stay
// doing for the coordinator's own work and for agents gone. Then, from the
// top, it applies each draft in order: a patch with git apply --index,
// which refuses the whole patch when any of it does not match the tree as
// it is (the change no longer applies), committed with its message through
// the hooks, as any commit, a failed commit put back by putBack on the
// files git apply --numstat lists; a command draft is run by this same
// itos binary, its arguments as kept, and commits itself. A draft promoted
// leaves the list (saved at once) before the next starts; the first that
// fails is draft-not-applied, exit 1, it and those after it kept.
//
// An edit draft (slice 99) never reads or writes the working tree, so it
// may be made while an agent holds the checkout: draft edit writes a copy
// of each path as HEAD has it (git cat-file blob, empty for a file HEAD
// lacks) under edits/<id>/ beside the list, and records HEAD as the
// draft's base; the person edits the copies there. promote's editPatch
// makes the draft's patch then, the copies against the base through a
// scratch index (git hash-object -w --no-filters, update-index --cacheinfo
// with the base's mode; a copy removed is --force-remove, an empty copy of
// a new file nothing), and applies it as any change draft's, so a file that
// moved since the base stops it.
//
// A drafted command that mints an item's id (slice 103, mint.go): work add
// --kind slice|task|bug, task add and work promote. draft add mints its id
// at once, as the command would mint it under its own global flags,
// through the id counter, prints it and keeps it in the draft
// (domain.Draft.Minted), so a later draft, a spec's tags and a brief can
// write it before the item exists. promote then runs the command in this
// process, not as a child itos, through RunLine, which internal/cli binds
// to the run Main handed it, with the id, which the command writes instead
// of minting another: no flag or argument hands an id over, so the
// commands still refuse one a person passes. inPlace puts back what the
// line's global flags change in this process. A draft dropped leaves its
// id a gap, as the counter never hands a number out twice.
package draft
