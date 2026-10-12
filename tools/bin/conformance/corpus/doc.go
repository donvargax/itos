// Package corpus runs itos's conformance fixtures: the cases that say what the tool does, as
// data, run through its command line, so that the same files prove any implementation of it,
// whatever its language. tools/bin/conformance runs them from the command line, and
// tools/bin/previous-release runs the last release's with this package linked in, so the runner
// that judges an old corpus is always this tree's (T-144 ported it from run.ts).
//
// A fixture file is YAML: files, git and env at the top are every case's defaults, and cases is a
// list. A case is:
//
//	name     what it shows (unique in its file)
//	argv     the arguments after itos
//	git      a scratch repository, built first in the case's folder (git init -b main, fixed
//	         author and dates, so every SHA is fixed, and itos's two hooks declared in its git
//	         config as stand-ins, standInHooks): steps of commit: <message> (with files: to
//	         write first, and stage those only), stage: {files}, write: {files}, branch: <new>,
//	         checkout: <ref>, run: <sh command>; any step may say label: <name>, the commit HEAD
//	         is then
//	files    written after the repository: path → text, or null to delete it, or { text,
//	         executable: true }; a path ending in / is a folder. A case's files are laid over
//	         the file's own.
//	stdin, env (laid over the file's), cwd (a folder of the case's)
//	hide     commands the PATH must not reach (a folder holding one is replaced by a copy of
//	         links without it), as every itos-* program of the caller's PATH always is (below)
//	github   a fake GitHub API the case's itos is pointed at (below); a case's replaces the
//	         file's, and null leaves it out
//	exit     the expected exit code (required)
//	stdout, stderr          the exact text; stdout_has, stderr_has: text each must contain (one,
//	                        or a list); json: stdout's JSON
//	files_after             files as they must be after the run (null: absent)
//	git_config_after        entries of the case's git config as they must be after the run:
//	                        key → value (git config --get-all, a key set more than once its
//	                        values a line each), or null for a key unset; keys it does not name
//	                        are not read
//
// A fixture is read as YAML 1.2's core schema reads it, as the TypeScript runner's parser did: a
// number is a number, a timestamp a string. go.yaml.in/yaml/v3 reads it, and the one YAML 1.2
// form it refuses that a release's corpus holds, a question mark in a flow sequence's plain item
// (v8.0.0's ask.yaml), is read as YAML 1.2 reads it (quoteFlowQuestions).
//
// Additive (Options.Additive, the command line's --additive), which tools/bin/previous-release
// turns on for the last release's corpus (T-076): stdout and stderr are lines that must all
// appear in the output in the same order, lines added before, between or after them passing,
// and json is what the output's JSON must hold: every key it names there with the value it
// gives, at every depth, an array element by element at the same length, a key added anywhere
// passing (docs/decisions/0021-ci-cuts-a-release-from-a-green-push-judged-against-the-last-release.md).
// files_after is judged by a written file's data, its comment lines for people
// (docs/decisions/0040-a-written-file-is-contract-by-its-data-not-its-comment-lines.md): a .yaml
// or .yml file by what it parses to, comments dropped, held as json is, a key added passing; a
// .md file by its text less its HTML comments; any other file whole. The exit code, stdout_has,
// stderr_has and git_config_after (each entry data, and one it does not name never read, T-113)
// are judged as ever. Without it, as this tree's corpus runs, every output and every file is
// pinned exactly, comments and all.
//
// A case's github is a fake of GitHub's API for ci.watch, status and ci range's github providers
// (slice 85, when v5.0.0 removed the command providers the corpus faked CI with): the runner
// serves it on 127.0.0.1 for the case alone and gives itos GITHUB_API_URL, GITHUB_REPOSITORY
// (owner/name) and GITHUB_TOKEN (fake-token) as Actions does, before the case's env. It is
//
//	refuse   true: every request is answered 401
//	runs     the workflow runs it holds, newest first, each with workflow (ci.yml when left
//	         out), head_sha (a full SHA, or * for any commit, answered as the commit asked
//	         about), head_branch (main when left out), html_url ({sha} standing for the commit
//	         asked about), status, conclusion and jobs ([{name, status, conclusion}]); or looks,
//	         a list of those answer fields, the nth request that finds the run given the nth,
//	         the last one for every request after it, as a run going on would be seen
//	visibility  public, private or internal: the repository's, as its endpoint answers it,
//	         with private (GitHub's bool) beside it; left out, the endpoint is a 404
//
// It answers a workflow's runs (head_sha and branch narrowing them), a run's jobs and the
// repository, the token required; anything else is a 404.
//
// In every string, {{dir}} is the case's folder, {{PATH}} the runner's PATH with the caller's
// extensions hidden and a private native Git folder first, {{PATH_SEP}} its platform separator,
// {{git}} the native Git executable, {{sha.<label>}} and {{short.<label>}} a labelled commit (40
// and 7 characters), {{version}} the version the binary says it is (<command> version's last
// word, asked once before any case), so a build stamped with any version passes, and
// {{release}} that version's X.Y.Z, the version a config's requires can name (a build between
// releases says a pre-release, 2.3.1-dev.5.g1234abc, which no comparator reads).
//
// The command runs in a clean environment (cleanEnv): no GIT_*, ITOS_*, GITHUB_* or CI variable
// of the caller's, HOME an empty folder, no global or system git config, no background git
// maintenance (noMaintenance), and nowhere a release could be fetched from. Nothing is shared
// between cases, so they run in parallel.
//
// Native Git, the git the runner builds and inspects each case's repository with, is
// internal/git's Real, the first git on the PATH that is not an itos (a git shim's link), unless
// the caller names one (Options.Git, --git). The PATH is the caller's without any itos-<command>
// on it (T-080): itos runs and lists every extension it finds there, so one installed on the
// machine would change what a case sees. Each folder holding one is replaced, once a run, by a
// folder of links to everything else in it, so the other programs there stay reachable. A
// private folder with only native git goes first, so an old fixture wrapper's bare-git fallback
// cannot select the caller's shim; a case can still put its own shim first (PATH:
// "{{dir}}/bin{{PATH_SEP}}{{PATH}}").
//
// Each process a case starts leads a process group of its own, killed as soon as it exits, so a
// descendant holding its output open cannot hold the case; a case is bounded by a deadline
// (Options.CaseDeadline, a minute), past which every process it started is killed and the case
// fails, saying so. Nothing bounds the runner's setup: the TypeScript runner ran its own tests
// there under a 15-second deadline, which a busy machine (a load average of 44) passed, so the
// corpus never ran (p1-corpus-runner-load-deadline). Its tests are this package's Go tests now
// (TestRegressions), run by go test ./tools/... as any other.
package corpus
