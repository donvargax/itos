// Package providers is what itos asks of the world outside the repository,
// each a provider chosen in itos.yaml: where a push's range starts
// (ci.range), the CI run of a commit and the last nightly (ci.watch), who a
// session works for (work.identity) and who may own work (work.people). A
// provider is a function made from the config and the environment, never an
// error at the call: none makes none, github a GitHub value (repository,
// token, workflow, API address). A provider over another forge's API
// (GitLab, Forgejo) would be another such value and another case in each
// constructor; the schema accepts none yet, so a config naming one is a
// config error, not a command that runs half-built.
//
// # GitHub's API
//
// The API's address is GITHUB_API_URL (APIEnv) when the environment sets it,
// as Actions does and GitHub Enterprise needs, for every look at GitHub,
// ci.range's, ci.watch's and itos status's; else the variable GitHubAPI, so
// a unit test points it at an httptest server and the scenarios point the
// binary at one through the environment, as no test reaches the network.
// Requests go through net/http with Timeout (a minute).
//
// No provider reads a list of a branch's runs: GitHub served that list stale
// (2026-10-05) and a range reached back past commits already proved. Every
// look asks for one commit's runs (head_sha=) instead.
//
// # Where a range starts
//
// RangeProvider(cfg, env) makes a Range, a function of the head giving the
// start or "". github's NearestGreen walks the head's first parents from its
// parent (git rev-list --first-parent, FirstParentsAsked of them, 100) and
// asks the API for each commit's runs (GreenRunOf), starting at the first
// with a successful run of that commit, on whatever branch it ran. Any
// failure of the walk reads as no green run, and runs everything. RangeStart
// keeps a pull request's base without asking the provider, else holds the
// provider's start to the head with git merge-base --is-ancestor.
// LastGreenProvider gives itos status the same walk from a branch's head as
// fetched (LastGreenFrom: one walk, greenFirstParent, the head itself
// included there and its parent first for a range), saying what went wrong
// where the range reads it as no start; its token and repository are found
// as the watch's are, so it reads outside CI.
//
// # Watching a run
//
// WatchProvider(cfg, WatchSetup) makes a Watcher from ci.watch: Look, one
// look at a commit's run, github's RunOf, which lists the workflow's runs for
// head_sha, takes the newest by created_at, and reads its jobs, a Run (url,
// status, conclusion, jobs, and the ID, head, branch and creation --json
// leaves out); and Runs, RunsOn, the workflow's last 20 runs on a branch,
// newest first, among which a watch finds the run that superseded a
// cancelled one (bug 41). Its token is ci.range.github.token_env's variable else
// GhToken (gh auth token), its repository repository_env's else
// GitHubRepository, the remote's URL read as GitHub's (https, ssh or
// scp-like); with no token it fails before any request, naming both ways to
// give one. RunOf answers found false while GitHub has no run, and a
// kind.Temporary error for no network, a 5xx or a 429; any other refusal is
// an error. NightlyProvider reads ci.watch.github.nightly_workflow's newest
// run on a branch (NewestRun) as RunOf reads a commit's; a provider naming
// no nightly gives no look. The loop that waits is internal/cli's watchRun.
//
// # Who a session works for, and the people
//
// IdentityProvider makes an Identity that answers a handle or why it has
// none, never an error: none with its hint, and github running gh api user
// --jq .login (GhLogin), gh's stderr dropped and gh missing told apart from
// gh failing. gh gets GhTimeout, 5 seconds, a constant and no setting, its
// stdin the null device; past it the context kills gh and, on unix, its
// process group (killTree, gh_unix.go), WaitDelay stops waiting on its
// output, and the answer is a problem naming --as. work.Whoami asks it only
// among several people.
//
// People reads work.people: an All Contributors table in Markdown
// (AllContributorsMd, CONTRIBUTORS.md here), an .all-contributorsrc
// (AllContributorsRc) or a YAML list (YAMLLogins). A people file that is missing or cannot be read is
// a warning, never a failure: the registry's check then checks no owner.
package providers
