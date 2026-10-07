// Package config finds, reads and validates itos.yaml, the one policy file:
// the ledger's layout (ledger), the commit types, footers, path sets and
// scopes (commits), the named tests and their adapters (tests), CI's steps,
// costs, prose shortcut, nightly and range (ci), the work registry and the
// people (work), the hooks (hooks), the pin (pin) and where verification
// starts (commits.since). A project changes its policy here, not in the
// code, and every reader of the config goes through Load; none has a
// fallback of its own.
//
// # Finding the file
//
// Path, which is Locate(""), gives the file --config or ITOS_CONFIG names,
// else itos.yaml in the folder itos runs in, else the stealth config. --root
// changes that folder first, and so does a run from a subfolder (top.go):
// with no --config, no ITOS_CONFIG, no --root and no itos.yaml in the
// folder, Top asks git rev-parse --show-toplevel --git-common-dir once per
// folder and run (so the launcher and the command line ask once between
// them) and gives the top level when it is not the folder itself and holds
// an itos.yaml or the stealth config; inside the git folder and outside a
// repository it gives nothing. The command line then moves there before any
// command, as --root <top> would, and reads the paths the person typed from
// where they stood (cli's typed).
//
// The stealth mode (stealth.go) is one person's itos in a repository whose
// team does not use it: where nothing else is found, Locate gives <git
// common dir>/itos/itos.yaml (StealthFolder) when it exists, keeping git's
// answer relative (.git) or absolute (a linked worktree), so messages name
// .git/itos/… and every linked worktree finds the main one's. IsStealth
// tells the stealth config by where it is (os.SameFile with that path, after
// a check of the names that spares a project's itos.yaml any git), not by
// how it was found, so the absolute ITOS_CONFIG an extension is given reads
// the same files when it calls back. Load marks it Stealth and lays its own
// defaults over the file (stealthOnly: hooks.bin is GlobalBin, the global
// launcher, and there is no work.people, the person being the only one), and
// beside rewrites the paths of itos's own data (ledger.files, work.registry,
// work.asks, work.decisions, guide.orchestrating, each kind's smoke.file)
// into its folder before the typed config is decoded, so the two agree; a
// kind's root, the globs and the commands stay the project's. Nothing else
// knows the mode but internal/source (a path in the git folder is read from
// the file whatever the tree), the footer rules (a stealth ledger footer's
// IDs are read in the working tree, since no commit carries that ledger) and
// the commands that write notes or take the unpushed range.
//
// Managed (managed.go) is the same question asked by file checks alone, for
// the git shim and the guard, which must not start git: ITOS_CONFIG naming a
// file, else itos.yaml in the folder, else WorkTree's top (the first folder
// up from the physical one with a .git, a .git file's gitdir: and that
// gitdir's commondir giving the common dir, and none inside a git folder,
// told by HEAD, objects and refs) holding itos.yaml, or its common dir the
// stealth config. Its unit tests hold it to Top and Locate on each layout.
//
// # Loading
//
// Load holds the file to the schema (schema.go, the specs as data with their
// problems' wording), where an unknown key is an error naming the key it
// misspells, and every key accepted is one a tool reads: a key waiting on a
// feature not built yet is rejected until that feature reads it. A key or a
// value a major release removed is refused as removed, not as unknown
// (removedKeys and removedValues, rule config-removed), its message saying
// what to write instead, since config check prints the message alone;
// config get names such a key as removed too (RemovedKey). Then the
// cross-checks (cross.go) hold what the schema cannot say: names that must
// refer to something, patterns that must compile, commits.since a full SHA.
// The five pattern keys (ledger.id, ledger.group.pattern, tests.<kind>.id,
// ci.cost.static, ci.covers[].matches) are RE2, the binary's dialect, and
// config check refuses one RE2 cannot compile (config-regexp).
//
// # The one table of defaults
//
// The file is laid over defaults() (defaults.go), one ordered tree, merged
// under the file by layered (tests.<kind> under each kind the file has) and
// decoded into the typed Config the tools read. config check
// --print-defaults prints exactly that tree, so a default is applied
// exactly when it is printed. Some defaults depend on the file: DefaultsFor
// puts work.registry beside the config's ledger (work-items.yaml in
// ledger.files' folder, not itself a ledger file), and takes whether the
// config is the stealth one. GlobalBin (itos) is every config's default
// hooks.bin, the launcher on the PATH running the version the repository
// pins; the key stays, internal and unsupported, for a repository that must
// run its own build, as this one sets it. A key with no default (ledger.id,
// a kind's root) is a nil pointer, a nil list or an empty Ordered (a mapping
// whose order matters, as written). The file as written stays beside the
// loaded config for HasSection and Section, since a section the file leaves
// out holds only its defaults, and the tree it was decoded from for Get,
// which itos config get <key> reads a dotted key out of after KnownKey has
// held the key to the schema (an object's keys, any key of a map, nothing
// under a list or a scalar), so it prints what the tools use.
//
// Schema is the schema table as exported data with each key's words (about
// in schema.go), which tools/bin/config-schema converts to itos.schema.json,
// a release asset, with DefaultsFor(nil) laid on it as defaults.
//
// # Patterns over commands
//
// Every pattern that reads a command (ci.cost.static, ci.covers,
// ci.nightly_only and a kind's recognize templates) reads a command whose
// first word is hooks.bin as starting with itos, through Readings: each is
// tried on the command as written and as so read, so a pattern written for
// itos and one naming the path both match (MatchesStatic). Normal collapses
// a command's whitespace, as those patterns and check keys read it.
//
// # Where verification starts
//
// commits.since (since.go): verify, and with it the built-in moves rule,
// lists a range's commits less that commit and its ancestors (RangeArgs,
// ^<commits.since> put first, since --not turns round what follows), and a
// range command's {from} is that commit when the range's own start is empty
// or older (RangeStart). config check and verify fail with exit 2 when the
// repository does not have it (SinceIssues); in a shallow clone the same
// problem says the commit may lie beyond the clone's history, with git fetch
// --unshallow or fetch-depth: 0 as its fix. The commit-msg hook never reads
// it. A footer's own since (commits.footers.<name>.since) is the same idea
// for one rule: verify leaves that commit and its ancestors out of the
// footer's required_for (Before), so a footer a project requires later does
// not fail the history written before it, while the commit-msg hook and
// commit check-message, which judge the commit being made, always require
// it.
package config
