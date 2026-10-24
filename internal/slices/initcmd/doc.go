// Package initcmd is the init command (init cannot name a package), a slice
// (ADR-0053) with no domain of its own: its CLI and Run are the command
// line, its Query and Handler the application (slice 48,
// features/init.feature;
// docs/decisions/0030-itos-init-readies-a-repository-and-doubles-as-a-doctor.md):
// a repository made ready for itos in one command, and run again, a report
// of what is missing.
//
// Command (command.go) declares its name, help and flag spec, by which
// internal/cli hands Parse the arguments after init, with the folder itos
// was started in, which --git-shim-dir (made absolute by Ports' Abs) and
// --policy are read from as the person typed them. Parse refuses, before any folder is
// looked at, what init does not take, --git-shim-dir with --no-git-shim and
// --stealth with --plugin project.
//
// Handle moves to the repository's top (toTop: the ports of the top, Ports'
// At, the process's own folder never changed), after a git init where no
// repository is found (gitInit, the VCS's Init), before anything asks
// config.Path; in a git folder it is a usage error. Every path init reads
// or writes relative is read from the folder it runs in, the VCS's Folder. With a config there it
// writes nothing and is a doctor (report): configcheck.Findings' problems
// and hookProblems' (a git that runs no hook its config declares, else each
// of itos's two entries the git config does not declare, hooks.Declared over
// Ports' hooks.Git), exit 1 when there is any; then its notes, never counted
// as missing, under their own heading and in the checked object's notes:
// Findings' warnings and pinBehind, a pin older than the newest release,
// naming itos upgrade (slice 115), left out when the release server does
// not answer. Where the config found is not the stealth one, --stealth is a
// usage error before anything is written (bug 43): a stealth config is for
// a clone that cannot change the project, not one with its own itos.yaml;
// under a stealth config --plugin project is one too. ITOS_CONFIG naming a
// file that does not exist is a usage error.
//
// Else write asks Ports.Release for the newest release, renders the starter
// (starter.go: starter.config, a template of commented YAML, not
// value.YAML, so it reads as a person's file), writes it, the ledger and the
// registry, never over a file that is there, and, when features/ holds a
// .feature file, lists the scenario kind's tests through the adapter
// (tests.ListTests) to name each file's first live one in the smoke set, so
// the smoke rule and init cannot disagree; where no scenario carries an ID
// tag it writes starter.untagged, whose Scenarios footer is required of no
// type, and says how to tag one. Then Ports.InstallHooks, as hook install
// runs, its exit code init's (under --json its object nested as hooks).
// --stealth puts the config at <git common dir>/itos/itos.yaml and its
// files beside it; the hooks go into the git config either way.
//
// init --policy <file> (policy.go, policyInit; slice 108, q-49) takes the
// place of the starter and of the doctor alike. policyPlanned reads the
// file from where the person stood (typed), judges it with
// config.FromText, and makes it the project's own (policyConfig, the
// policy's text edited in place by value.Doc: commits.since HEAD or none,
// each footer's since dropped, the starter's ledger layout where it has
// none, the newest release pinned where it pins none); then, before
// anything is written, git init included (toTop moves to the top without
// it, gitInit runs it after), policyData plans the fresh ledger, registry
// and smoke sets at the config's paths and refuses an adoption task T-1 the
// ledger cannot hold, policyPaths a file outside the project or the stealth
// folder once its existing part's symbolic links are followed (resolved),
// both exit 2; policyThere refuses any config, ledger file or registry
// already there, and policySmokeSets a command adapter's list that fails or
// that the protocol refuses (slice 110: the list runs once, through
// tests.ListTests, as tests list runs it), both exit 1 (policyRefused). A
// smoke file already there is the project's own (slice 112, issue #39):
// policySmokeCheck judges it as tests smoke check does (tests.LoadSmoke,
// tests.SmokeIssues), and it is kept, listed as kept, when the rule holds,
// else refused with the rule's problems, exit 1; a set is derived only where
// there is none. writeNew never writes over a file, and a failure once
// writing began names what this run did (partial). The hooks and the
// offers follow as for the starter, and policyNext names what to commit.
//
// Both ways, init then makes three offers, in this order, through
// makeOffers
// (docs/decisions/0031-itos-init-offers-the-plugin-and-the-git-shim-opt-in-everywhere.md,
// docs/decisions/0032-itos-init-generates-the-config-s-rules-into-a-marked-block-of-agents-md.md).
// The first run without a flag asks a terminal (question, only where Ports'
// Terminal says stdin and stdout are both a character device other than the
// null device, never under --json, the offers sharing one buffered reader of
// Ports' Stdin so none takes another's answer) or says how; run again where
// a config is, none asks: each says how, or installs for its flag, and the
// checked object always holds notes and git_shim. Their outcomes are
// --json's plugin, git_shim and agent_rules, their highest exit code init's
// when hook install's is 0, and the plugin installed at the project's scope
// and the rules' files written are named in what to commit.
//
//   - The plugin (plugin.go, pluginOffer.run): --plugin no ends it, a
//     claude the PATH lacks (Ports' LookPath) is said only for --plugin,
//     claude plugin list --json (Ports' Command) read as a list whose
//     itos@itos entry, enabled where init runs, means installed; the scope
//     is the flag's (a bare one pluginDefault's) or the terminal's answer.
//     It runs marketplace add, whose failure alone is not one, then install,
//     whose failure is printed with claude's output and exits 1; a local
//     install under --stealth lists .claude/settings.local.json in
//     info/exclude while git shows it untracked (excludeIfShown). --stealth
//     --plugin project, or --plugin project where the config found is the
//     stealth one, is a usage error before anything runs.
//   - The git shim (shim.go, shimOffer.run): --no-git-shim ends it;
//     Ports' ShimPlace gives the link git-shim install would make (its
//     --git-shim-dir, made absolute where it was typed), unless, with no
//     folder named, the first git on the PATH is an itos (Ports' IsItos,
//     the git adapter's cross-binary identity check); that entry is kept
//     without claiming a target, before the checks of the proposed link. A
//     link to this itos is kept, a git that is not itos refused for
//     --git-shim (exit 1) and otherwise only said, another itos
//     replaced; then MakeLink and StandingLine.
//   - The rules for agents (rules.go, rulesOffer.run): the block, its
//     markers and their staleness are internal/engine/agentrules'.
//     writeRules replaces the block agentrules.Split found, or appends one,
//     and gives CLAUDE.md an @AGENTS.md line unless it has one or is
//     AGENTS.md itself; both files keep the line endings they had. A rerun
//     with no flag compares the block with agentrules.Block and says only
//     that it is stale. Under a stealth config runStealth
//     (rulesstealth.go) writes the three files agentrules.Stealth plans,
//     never AGENTS.md or CLAUDE.md, refusing a file at the top that the
//     project tracks.
//
// It is the launcher's own command, as pin is (internal/launch), where the
// config pins nothing: where there is no config there is no pin to hand the
// run to, and the newest release must not run in its place. Where the config
// pins a version, that version runs it, as any other command (bug 104).
package initcmd
