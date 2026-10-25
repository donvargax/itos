// Package upgrade is the upgrade command, a slice (ADR-0053) with no domain
// of its own, moving the pin with pin's engine, internal/engine/pin
// (imported as pinengine): its CLI and Run are the command line, its Query
// and Handler the application (slice 75, features/upgrade-command.feature).
//
// Command (command.go) declares its name and help, by which internal/cli
// hands Parse the arguments after upgrade, with the global --json; Parse
// reads the one <version>, the newest by default or with latest, as itos
// pin reads it. Handle moves the project to it from the version it is on:
// the pin's (pinengine's ReadConfig, through Ports), else the version= line
// of tools/bin/install-itos, the install script a release's notes write
// (v2.0.0's Upgrading); with neither, or a version older than that, it
// refuses (exit 1) and names itos pin. It asks for the release's
// checksums.txt as pin does (Ports.Release, pinengine.Pinned), then walks
// back from the new release through each upgrading.json's previous (T-091)
// until the old version, so no API lists the releases, fetching each
// release's itos.schema.json too (Ports.Fetch). A 404 (release.NotFound),
// a release cut before upgrading.json existed, ends the walk naming its
// notes, as does an upgrading.json of a schema it does not read; any other
// failure is a line on stderr, exit by its kind, every file untouched.
//
// It edits only what a release fixes exactly, every edit made in memory
// before a file is written: the pin (Config.Moved, Config.Replaced), the
// config's first line when it is a yaml-language-server schema line naming
// a release's itos.schema.json (only its version changes), the script's
// version= value and each platform=<os-arch> sum=<hash> hash, looked up in
// the new checksums.txt by release.Listed, and the config's keys each
// crossed release's schema declares renamed (a property's
// x-itos-renamed-from) or removed (its top-level x-itos-removed), release by
// release, oldest first, by value.Doc's Rename and Remove (applySteps), so
// a key renamed twice ends under its newest name (slice 113). A layout the
// editor cannot change byte for byte exits 2 naming the key. Never anything
// read from a commit's text: the breaking changes and the Upgrading footers
// are what is left by hand, which Run prints last, or under --json returns
// beside the releases as upgrading.json holds them and the edits applied.
// It commits nothing.
//
// It also names, and moves none of, the steps that launch the itos action
// (slice 150, scanWorkflows, report in Run): each uses: under
// jobs.<job>.steps of the .github/workflows/*.yml and *.yaml files directly
// in the folder itos runs in, read as YAML, whose value is
// donvargax/itos@<ref>, by its file, the line of its uses: key, its ref and
// its with.version as written. The action installs a launcher, which the
// ref or the version input chooses, and the repository's pin chooses the itos
// that runs (action.yml, ADR-0024), so a line left as it is runs the pinned
// release; with no pin the release it installs runs, which the report says,
// both unless the workflow sets ITOS_VERSION or loads another config, which
// the scan does not read. A workflow that value.Parse refuses (a syntax
// error, a key written twice, several documents) or that cannot be read is a
// scan problem naming it and why, the others still scanned. A stealth config, never committed, is read by no
// workflow, so it names none. --json carries both as action_uses and
// action_scan_problems, [] when empty and on a refusal.
//
// The itos doing the upgrade also rewrites a present block of its rules for
// agents in its own words, as itos init --agent-rules does (Ports.Rules,
// internal/engine/agentrules' Refreshed): only between the markers, only in
// a file that has a block, a stealth config's copies too, each file it
// rewrites listed among the edits (slice 126,
// features/upgrade-handoff.feature). Like pin it is the launcher's own
// command, never the pinned version's, which may predate it; but when the
// version it moves to is newer than this itos, the launcher hands the
// upgrade to that version (UpgradeTarget in internal/cli, internal/launch),
// naming it on the command line (NamingVersion), so the steps applied, the
// block's words among them, are always the newest's.
package upgrade
