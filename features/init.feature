@phase-3
Feature: itos init, a repository made ready for itos
  Adopting itos meant writing an itos.yaml by hand, a ledger and a registry
  beside it, a pin from a release's checksums.txt, then installing the
  hooks. itos init does it in one command, in the repository it runs in, or
  in a folder that is not one yet, after git init. Where there is no config
  it writes a starter one (the user's call, 2026-10-03): the Conventional
  Commits types, a Task footer required of every type but feat and fix, a
  ledger and a work registry under tasks/, and, when features/ holds feature
  files, a scenario kind whose Scenarios footer feat and fix need, with a
  smoke set naming one live scenario of each file; no path scopes, which are
  each project's own. commits.since names HEAD, so history written before
  itos is never judged. hooks.bin is itos, the global launcher, and the pin
  the newest release, as itos pin writes it; where the release server cannot
  be reached it pins nothing and says how to (a config with no pin runs the
  binary that was called). Then it declares the hooks in the git config, as
  hook install does. With --stealth all of it goes under the git folder,
  nothing the project tracks touched.

  Where a config already is, init writes nothing: it reports what is missing
  (the config's problems, a hook not installed) and exits 1 when anything
  is, 0 when nothing is, so run again it doubles as a check. init is the
  launcher's own command, as itos pin is: where there is no config there is
  no pin to hand the run to. Slice 49 offers the Claude Code plugin, and
  slice 50 the git shim and what a rerun notes beside what is missing (below).

  # The repository has a commit and no itos file at all; the scenarios
  # without a release server cannot reach one, so init pins nothing there.
  @ID-INIT-01 @slice-48
  Scenario: In an existing repository init writes a config the checks accept, starting at HEAD, and installs the hooks
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init"
    Then itos exits with code 0
    And the config's commits.since is HEAD's full SHA
    And the git config declares a "commit-msg" hook that runs itos
    And the git config declares a "pre-push" hook that runs itos
    When itos checks the config
    Then itos exits with code 0

  @ID-INIT-02 @slice-48
  Scenario: The starter config asks a Task footer of a chore, and no footer of a feat without feature files
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init"
    And a change to "README.md" is staged
    When the commit-msg hook checks the message "chore: tidy the readme"
    Then itos exits with code 1
    And its output says "Task"
    When the commit-msg hook checks the message "feat: add a page"
    Then itos exits with code 0

  @ID-INIT-03 @slice-48
  Scenario: With feature files the starter config adds the scenarios, their footer and a smoke set the smoke rule accepts
    Given a repository that does not use itos, its one commit "docs: start"
    And the feature file "features/pages.feature" with the scenario "@ID-PAGE-01"
    When itos runs "init"
    Then itos exits with code 0
    When itos runs "tests smoke check scenario"
    Then itos exits with code 0
    Given a change to "README.md" is staged
    When the commit-msg hook checks the message "feat: add a page"
    Then itos exits with code 1
    And its output says "Scenarios"

  @ID-INIT-04 @slice-48
  Scenario: In a folder that is not a repository init runs git init first, and starts nowhere
    Given a folder that is not a git repository
    When itos runs "init"
    Then itos exits with code 0
    And the folder is a git repository
    And the config has no commits.since
    When itos checks the config
    Then itos exits with code 0

  @ID-INIT-05 @slice-48
  Scenario: With a release server init pins its newest release
    Given a release server offering the versions "9.1.0" and "9.2.0"
    And a repository that does not use itos, its one commit "docs: start"
    When itos runs "init"
    Then itos exits with code 0
    And the config's pin is the version "9.2.0" of the release server, with its checksums
    And no version of the release server ran

  @ID-INIT-06 @slice-48
  Scenario: Where the release server cannot be reached init pins nothing and says how to
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init"
    Then itos exits with code 0
    And the config has no pin
    And its output says "itos pin"

  @ID-INIT-07 @slice-48
  Scenario: Run again, init changes nothing and finds nothing missing
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init"
    When itos runs "init"
    Then itos exits with code 0
    And no file changed since the last run

  @ID-INIT-08 @slice-48
  Scenario: Run again where a hook is missing, init reports it and installs nothing
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init"
    And itos's "pre-push" hook is taken out of the git config
    When itos runs "init"
    Then itos exits with code 1
    And its output says "pre-push"
    And its output says "itos hook install"
    And the git config declares no "pre-push" hook

  @ID-INIT-09 @slice-48
  Scenario: With --stealth init writes everything under the git folder and declares the hooks in the git config
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init --stealth"
    Then itos exits with code 0
    And git status shows nothing to commit
    And the git config declares a "commit-msg" hook that runs itos
    When itos checks the config
    Then itos exits with code 0

  # Slice 108, issue #33: a template supplies reviewed policy, not its work
  # history. --policy names an ordinary config to materialize into a fresh
  # project; it is neither --config nor configuration inheritance. Plain init
  # and its existing-config diagnostic behavior remain unchanged. The source
  # is read from the caller's directory, even when init moves to the git top.
  # Configuration paths in that policy mean the target project's root (or the
  # stealth data folder for itos-owned data), never the source's directory.
  #
  # Preflight the complete operation before creating git metadata, files or
  # hooks: validate the policy, its fresh data layout and every write target.
  # Refuse an existing effective config, any file matching the target ledger
  # pattern, or the target registry or smoke file, including empty files. No merge, --force, overwrite
  # or automatic retry/adoption recovery belongs in this slice. Destinations
  # must remain inside the target data root, not traverse symlinks out of it.
  # --config/ITOS_CONFIG cannot select a different target with --policy.
  #
  # Retain the source's applicable schema, commit/test/CI/proof policy, requires,
  # hook settings, data-path settings and well-formed pin. With no source pin,
  # use init's usual latest-release attempt and offline fallback. Never run the
  # pin's binary, a CI step, code proof or source task check during adoption.
  # Reset commits.since to the target HEAD, absent in an unborn repository;
  # remove every source footer since. Never import source ledger/registry,
  # questions, decisions, smoke selections, proof results or mutation caches.
  # In project mode create only the adoption task T-1; in stealth mode the
  # ledger stays empty, as for plain init. Create an unowned group 1 with no
  # work items, using configured paths and groups_key. If the source omits the
  # ledger section, supply the starter's tasks/phase-{group}.yaml layout alone,
  # not its generic commit/test/CI policy. A policy whose task ID, commit types
  # or ledger group layout cannot represent that bootstrap is refused before
  # writing; never weaken it or guess a string that satisfies an arbitrary RE2.
  # For a configured built-in Gherkin kind, derive a fresh smoke set from its
  # actual target tests without changing the source's test policy. A command
  # adapter's smoke set is derived the same way from its list, run once in the
  # target before anything is written (slice 110, the user's call 2026-10-10:
  # the project's own configured command, which tests smoke check would run
  # anyway); a list that fails, or whose output the protocol refuses, refuses
  # init with its error and writes nothing.
  # Preserve current explicit plugin/shim/agent-rules offers; those setup
  # integrations are not policy-quality commands. Report normal initialized
  # JSON and concrete written paths. Preflight refusals leave everything alone;
  # an I/O/setup failure reports failure and only this call's partial work,
  # never claims initialization succeeded or removes preexisting user files.
  @ID-INIT-41 @slice-108
  Scenario: Explicit policy initialization retains quality policy and creates fresh work data at the selected paths
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "template/policy.yaml" holding the lines:
      """
      version: 1
      ledger:
        files: work/phase-{group}.yaml
        id: 'T-\d+'
      commits:
        header_lint: { use: builtin }
        footers:
          Task:
            source: ledger
            required_for: [chore]
            validate_for: all
            read_at: commit
        scopes:
          docs: { only: ['**/*.md', 'work/**'] }
      work:
        registry: work/items.yaml
        groups_key: groups
      ci:
        steps:
          - run: "printf 'ci' > ci-ran"
      proof:
        code:
          paths: ['**/*.go']
          check: "printf 'proof {base}' > proof-ran"
      """
    When itos runs "init --policy template/policy.yaml --plugin no --no-git-shim --no-agent-rules --json"
    Then itos exits with code 0
    And the output's JSON field "action" is "initialized"
    And the config value "ci.steps" equals the YAML:
      """
      - run: "printf 'ci' > ci-ran"
      """
    And the config value "proof.code" equals the YAML:
      """
      paths: ['**/*.go']
      check: "printf 'proof {base}' > proof-ran"
      """
    And the config value "commits.footers.Task.required_for" equals the YAML:
      """
      [chore]
      """
    And the config value "commits.scopes.docs.only" equals the YAML:
      """
      ['**/*.md', 'work/**']
      """
    And the ledger file "work/phase-1.yaml" contains only the adoption task "T-1"
    And the registry file "work/items.yaml" has no items and its group "1" under "groups" is unowned
    And the file "template/policy.yaml" has the same contents as its committed version
    And the file "ci-ran" does not exist
    And the file "proof-ran" does not exist
    And the file "tasks/phase-1.yaml" does not exist
    And the git config declares a "commit-msg" hook that runs itos
    And the git config declares a "pre-push" hook that runs itos
    When itos checks the config
    Then itos exits with code 0

  @ID-INIT-42 @slice-108
  Scenario: Policy initialization resets history boundaries without importing the source's work history
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "template/policy.yaml" holding the lines:
      """
      version: 1
      commits:
        since: ffffffffffffffffffffffffffffffffffffffff
        footers:
          Task:
            source: ledger
            required_for: [chore]
            since: ffffffffffffffffffffffffffffffffffffffff
      """
    And the committed file "template/tasks/phase-9.yaml" holding the lines:
      """
      - id: T-999
        type: chore
        title: Template history
        done_when:
          - run: "printf 'task' > template-task-ran"
      """
    And the committed file "template/tasks/work-items.yaml" holding the lines:
      """
      phases: { 9: template-owner }
      items:
        - id: T-999
          title: Template history
          phase: 9
          owner: template-owner
          status: done
          kind: task
          depends_on: []
      """
    And the committed file "template/tasks/asks.yaml" holding "questions: [{id: q-999, question: old, status: open}]"
    And the committed file "template/.metrics/proof.json" holding "template proof"
    When itos runs "init --policy template/policy.yaml --plugin no --no-git-shim --no-agent-rules"
    Then itos exits with code 0
    And the config's commits.since is HEAD's full SHA
    And the config has no value at "commits.footers.Task.since"
    And the ledger file "tasks/phase-1.yaml" contains only the adoption task "T-1"
    And the registry file "tasks/work-items.yaml" has no items and its group "1" under "phases" is unowned
    And the file "tasks/phase-9.yaml" does not exist
    And the file "tasks/asks.yaml" does not exist
    And the file ".metrics/proof.json" does not exist
    And the file "template-task-ran" does not exist
    And the file "template/policy.yaml" has the same contents as its committed version

  @ID-INIT-43 @slice-108
  Scenario: Policy initialization preserves a supplied pin instead of moving it to the newest release
    Given a release server offering the versions "9.1.0" and "9.2.0"
    And a repository that does not use itos, its one commit "docs: start"
    And the committed file "policy.yaml" holding the lines:
      """
      version: 1
      pin:
        version: 9.1.0
        checksums: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
      """
    When itos runs "init --policy policy.yaml --plugin no --no-git-shim --no-agent-rules"
    Then itos exits with code 0
    And the config value "pin.version" equals the YAML:
      """
      9.1.0
      """
    And the config value "pin.checksums" equals the YAML:
      """
      aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
      """
    And no version of the release server ran
    And the file "policy.yaml" has the same contents as its committed version

  @ID-INIT-44 @slice-108
  Scenario: Policy initialization with no supplied pin uses init's usual newest-release pinning
    Given a release server offering the versions "9.1.0" and "9.2.0"
    And a repository that does not use itos, its one commit "docs: start"
    And the committed file "policy.yaml" holding "version: 1"
    When itos runs "init --policy policy.yaml --plugin no --no-git-shim --no-agent-rules"
    Then itos exits with code 0
    And the config's pin is the version "9.2.0" of the release server, with its checksums
    And no version of the release server ran

  @ID-INIT-45 @slice-108
  Scenario: Explicit policy initialization refuses an existing project configuration without changing it
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init"
    And the files init wrote are committed
    And the committed file "policy.yaml" holding "version: 1"
    When itos runs "init --policy policy.yaml --plugin no --no-git-shim --no-agent-rules"
    Then itos exits with code 1
    And its output says "itos.yaml"
    And no file changed since the last run

  @ID-INIT-46 @slice-108
  Scenario: Policy initialization refuses a preexisting registry before creating any project files or hooks
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "policy.yaml" holding "version: 1"
    And the committed file "tasks/work-items.yaml" holding "someone else's registry"
    When itos runs "init --policy policy.yaml --plugin no --no-git-shim --no-agent-rules"
    Then itos exits with code 1
    And its output says "tasks/work-items.yaml"
    And no file changed since the last run
    And the file "itos.yaml" does not exist
    And the file "tasks/phase-1.yaml" does not exist
    And the git config declares no "commit-msg" hook
    And the git config declares no "pre-push" hook

  @ID-INIT-47 @slice-108
  Scenario: Policy initialization refuses a preexisting ledger rather than retaining or importing its tasks
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "policy.yaml" holding "version: 1"
    And the committed file "tasks/phase-9.yaml" holding "someone else's tasks"
    When itos runs "init --policy policy.yaml --plugin no --no-git-shim --no-agent-rules"
    Then itos exits with code 1
    And its output says "tasks/phase-9.yaml"
    And no file changed since the last run
    And the file "itos.yaml" does not exist
    And the file "tasks/phase-1.yaml" does not exist
    And the file "tasks/work-items.yaml" does not exist
    And the git config declares no "commit-msg" hook

  @ID-INIT-48 @slice-108
  Scenario: An invalid source policy is refused before even initializing git in an unmanaged folder
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "policy.yaml" holding the lines:
      """
      version: 1
      imaginary_policy: true
      """
    And a folder that is not a git repository
    When itos runs "init --policy policy.yaml --plugin no --no-git-shim --no-agent-rules"
    Then itos exits with code 2
    And its output says "imaginary_policy"
    And the folder is still not a git repository
    And no file changed since the last run

  @ID-INIT-49 @slice-108
  Scenario: Policy initialization under stealth keeps fresh configured work data in the git folder
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "policy.yaml" holding the lines:
      """
      version: 1
      ledger:
        files: work/phase-{group}.yaml
      work:
        registry: work/items.yaml
      """
    When itos runs "init --policy policy.yaml --stealth --plugin no --no-git-shim --no-agent-rules"
    Then itos exits with code 0
    And the config's commits.since is HEAD's full SHA
    And the ledger file ".git/itos/work/phase-1.yaml" contains no tasks
    And the registry file ".git/itos/work/items.yaml" has no items and its group "1" under "phases" is unowned
    And the file "itos.yaml" does not exist
    And the file "work/items.yaml" does not exist
    And the file "policy.yaml" has the same contents as its committed version
    And git status shows nothing to commit
    And the git config declares a "commit-msg" hook that runs itos
    When itos checks the config
    Then itos exits with code 0

  @ID-INIT-50 @slice-108
  Scenario: Policy initialization derives Gherkin smoke selections from the target tests rather than source data
    Given a repository that does not use itos, its one commit "docs: start"
    And the feature file "specs/pages.feature" with the scenario "@ID-PAGE-01"
    And the committed file "template/policy.yaml" holding the lines:
      """
      version: 1
      tests:
        scenario:
          root: specs
          id: 'ID-[A-Z]+-\d+'
          smoke: { file: work/smoke.yaml }
      """
    And the committed file "template/work/smoke.yaml" holding "template selections must not be copied"
    When itos runs "init --policy template/policy.yaml --plugin no --no-git-shim --no-agent-rules"
    Then itos exits with code 0
    And the file "work/smoke.yaml" names "@ID-PAGE-01"
    And the file "features/smoke.yaml" does not exist
    When itos runs "tests smoke check scenario"
    Then itos exits with code 0

  @ID-INIT-51 @slice-108
  Scenario: Policy initialization refuses a generated data path outside the target root before writing
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "policy.yaml" holding the lines:
      """
      version: 1
      ledger:
        files: ../outside/phase-{group}.yaml
      """
    When itos runs "init --policy policy.yaml --plugin no --no-git-shim --no-agent-rules"
    Then itos exits with code 2
    And its output says "ledger.files"
    And no file changed since the last run
    And the file "../outside/phase-1.yaml" does not exist
    And the git config declares no "commit-msg" hook

  @ID-INIT-52 @slice-108
  Scenario: Policy initialization refuses a task ID policy that cannot represent the adoption task without weakening it
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "policy.yaml" holding the lines:
      """
      version: 1
      ledger:
        files: tasks/phase-{group}.yaml
        id: 'TASK-[0-9]+'
      """
    When itos runs "init --policy policy.yaml --plugin no --no-git-shim --no-agent-rules"
    Then itos exits with code 2
    And its output says "ledger.id"
    And no file changed since the last run
    And the file "itos.yaml" does not exist
    And the git config declares no "commit-msg" hook

  @ID-INIT-53 @slice-110 @wip
  Scenario: Policy initialization derives a command adapter's smoke set by running its list once
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "list.sh" holding the lines:
      """
      printf '%s\n' '{"protocol":1,"tests":[{"id":"U-1","file":"a_test.go","live":true},{"id":"U-2","file":"b_test.go","live":true}],"files":["a_test.go","b_test.go"]}'
      """
    And the committed file "template/policy.yaml" holding the lines:
      """
      version: 1
      tests:
        unit:
          adapter: { command: "sh list.sh" }
          id: 'U-\d+'
          smoke: { file: tests/smoke.yaml }
      """
    When itos runs "init --policy template/policy.yaml --plugin no --no-git-shim --no-agent-rules"
    Then itos exits with code 0
    And the file "tests/smoke.yaml" names "U-1"
    And the file "tests/smoke.yaml" names "U-2"
    When itos runs "tests smoke check unit"
    Then itos exits with code 0

  @ID-INIT-54 @slice-110 @wip
  Scenario: Policy initialization refuses before writing when a command adapter's list fails
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "list.sh" holding the lines:
      """
      echo 'no test binary yet' >&2
      exit 3
      """
    And the committed file "policy.yaml" holding the lines:
      """
      version: 1
      tests:
        unit:
          adapter: { command: "sh list.sh" }
          id: 'U-\d+'
          smoke: { file: tests/smoke.yaml }
      """
    When itos runs "init --policy policy.yaml --plugin no --no-git-shim --no-agent-rules"
    Then itos exits with code 1
    And its output says "no test binary yet"
    And no file changed since the last run
    And the file "itos.yaml" does not exist
    And the git config declares no "commit-msg" hook

  # Slice 49: the Claude Code plugin
  # (docs/decisions/0031-itos-init-offers-the-plugin-and-the-git-shim-opt-in-everywhere.md) is offered,
  # through Claude Code's own CLI: claude plugin list --json to see whether
  # itos@itos is installed, claude plugin marketplace add donvargax/itos and
  # claude plugin install itos@itos, both with --scope, to install it.
  # The offer is opt-in everywhere (the user's call, 2026-10-03): --plugin
  # <scope> answers it, project, user, local or no, and a bare --plugin takes
  # the mode's default scope, local under --stealth (.claude/settings.local.json,
  # this project and this person only), project otherwise. On a terminal with
  # no --plugin init asks, its default answer that same scope; anywhere else
  # (an agent, CI) it installs nothing and says how to. Under --stealth project
  # is refused, since it writes the committed .claude/settings.json, and the
  # local settings file is listed in .git/info/exclude when git would
  # otherwise show it. A plugin not installed is reported, never counted as
  # missing: it is an offer. Given --plugin, init installs it even where a
  # config already is, the flag being the ask.
  @ID-INIT-10 @slice-49
  Scenario: With --plugin init installs the itos plugin for Claude Code at that scope
    Given a repository that does not use itos, its one commit "docs: start"
    And a claude on the PATH that records its arguments
    When itos runs "init --plugin project"
    Then itos exits with code 0
    And claude was given "plugin marketplace add donvargax/itos --scope project"
    And claude was given "plugin install itos@itos --scope project"

  @ID-INIT-11 @slice-49
  Scenario: Away from a terminal and with no --plugin, init installs nothing and says how to
    Given a repository that does not use itos, its one commit "docs: start"
    And a claude on the PATH that records its arguments
    When itos runs "init"
    Then itos exits with code 0
    And claude was not given "plugin install"
    And its output says "--plugin"

  @ID-INIT-12 @slice-49
  Scenario: A plugin already installed is not installed again
    Given a repository that does not use itos, its one commit "docs: start"
    And a claude on the PATH that lists the plugin "itos@itos" as installed
    When itos runs "init --plugin user"
    Then itos exits with code 0
    And claude was not given "plugin install"
    And its output says "itos@itos"

  @ID-INIT-13 @slice-49
  Scenario: Under --stealth the plugin is not installed for the project, whose settings are committed
    Given a repository that does not use itos, its one commit "docs: start"
    And a claude on the PATH that records its arguments
    When itos runs "init --stealth --plugin project"
    Then itos exits with code 2
    And its output says "--plugin local"
    And claude was not given "plugin install"

  @ID-INIT-14 @slice-49
  Scenario: Without Claude Code on the PATH init says so and goes on
    Given a repository that does not use itos, its one commit "docs: start"
    And no claude on the PATH
    When itos runs "init --plugin user"
    Then itos exits with code 0
    And its output says "Claude Code"
    And the git config declares a "commit-msg" hook that runs itos

  @ID-INIT-15 @slice-49
  Scenario: Run again where the plugin is not installed, init reports it and still finds nothing missing
    Given a repository that does not use itos, its one commit "docs: start"
    And a claude on the PATH that records its arguments
    And itos has already run "init"
    When itos runs "init"
    Then itos exits with code 0
    And its output says "itos init --plugin"

  @ID-INIT-16 @slice-49
  Scenario: Under --stealth a bare --plugin installs it for this project alone, leaving git status clean
    Given a repository that does not use itos, its one commit "docs: start"
    And a claude on the PATH that records its arguments, writing .claude/settings.local.json as claude does
    When itos runs "init --stealth --plugin"
    Then itos exits with code 0
    And claude was given "plugin install itos@itos --scope local"
    And git status shows nothing to commit

  @ID-INIT-17 @slice-49
  Scenario: Under --stealth, away from a terminal and with no --plugin, init installs nothing and says how to
    Given a repository that does not use itos, its one commit "docs: start"
    And a claude on the PATH that records its arguments
    When itos runs "init --stealth"
    Then itos exits with code 0
    And claude was not given "plugin install"
    And its output says "--plugin"

  # Slice 50: the git shim (itos help git-shim) is offered as the plugin is:
  # --git-shim installs it as git-shim install does (--git-shim-dir is its
  # --dir), --no-git-shim declines, a terminal is asked, anywhere else
  # nothing is installed and the report says how. The report also says, never
  # counting them as missing, a pin behind the newest release (naming itos
  # pin) and a people file the config names but the repository lacks.
  @ID-INIT-18 @slice-50
  Scenario: With --git-shim init links itos as git in the folder given
    Given a repository that does not use itos, its one commit "docs: start"
    And a "shims" folder
    When itos runs "init --git-shim --git-shim-dir shims"
    Then itos exits with code 0
    And "shims/git" runs itos

  @ID-INIT-19 @slice-50
  Scenario: Away from a terminal and with no --git-shim, init links nothing and says how to
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init"
    Then itos exits with code 0
    And its output says "--git-shim"

  @ID-INIT-20 @slice-50
  Scenario: Run again where the pin is behind the newest release, init says so and names itos pin
    Given a release server offering the versions "9.1.0" and "9.2.0"
    And a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init"
    And itos has already run "pin 9.1.0"
    When itos runs "init"
    Then itos exits with code 0
    And its output says "9.2.0"
    And its output says "itos pin"

  # The starter names no work section, so the people file is the default,
  # CONTRIBUTORS.md, which init does not write; config check only warns.
  @ID-INIT-21 @slice-50
  Scenario: Run again where the people file the config names is missing, init says which file and goes on
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init"
    When itos runs "init"
    Then itos exits with code 0
    And its output says "CONTRIBUTORS.md"

  # p3-init-untagged-features, folded into slice 50 (the user's call,
  # 2026-10-03): most Cucumber projects tag no scenario with an ID, and a
  # Scenarios footer required of feat and fix would then refuse every feat,
  # with nothing to name. Where no scenario carries an ID tag, init writes the
  # scenario kind without requiring the footer, and says how to tag a
  # scenario so that it can be required.
  @ID-INIT-22 @slice-50
  Scenario: With feature files whose scenarios carry no ID tag, a feat needs no Scenarios footer, and init says how to tag them
    Given a repository that does not use itos, its one commit "docs: start"
    And the feature file "features/pages.feature" with a scenario that has no tag
    When itos runs "init"
    Then itos exits with code 0
    And its output says "@ID-"
    When itos runs "tests smoke check scenario"
    Then itos exits with code 0
    Given a change to "README.md" is staged
    When the commit-msg hook checks the message "feat: add a page"
    Then itos exits with code 0

  # Bug 12: init writes the registry as phases: {} and items: [], and its
  # stealth ledger as [], and work add and task add could append only to a
  # block list that already had an item, so the first item or task of a
  # freshly adopted repository was refused, and a registry listing no phase
  # refused any item anyway. Init now lists its ledger's one group, phase 1,
  # owned by nobody, and both commands write the first item into an empty
  # list. Adding a further phase is p1-work-add-phase.
  # Its second half (the user's call, 2026-10-04, q-1): the starter required
  # a Task footer of docs commits, and every registry command commits docs
  # with none, so the hooks init installed refused them. The starter now
  # leaves docs out of required_for, as this repository does, and gives docs
  # the one path scope the starter has, Markdown, docs/ and itos's own data
  # beside the ledger, so a change labelled docs to skip the footer is still
  # refused (@ID-INIT-31); every other type stays unscoped.
  @ID-INIT-23 @bug-12
  Scenario: In a repository init set up, work add writes the first item into the empty registry, in phase 1
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init"
    And the files init wrote are committed
    When itos runs the command line "work add p1-thing --title 'Do the thing' --why 'Because it is missing.'"
    Then itos exits with code 0
    And the registry's item "p1-thing" is an idea titled "Do the thing" with the status "todo"
    And the last commit's header is "docs: add p1-thing"
    When itos checks the work registry
    Then itos exits with code 0

  @ID-INIT-24 @bug-12
  Scenario: In a repository init set up, task add writes the first item into the empty registry
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init"
    And the files init wrote are committed
    When itos runs the command line "task add --group 1 --type chore --title 'Tidy the readme' --why 'It drifted.' --check 'true'"
    Then itos exits with code 0
    And the ledger file "tasks/phase-1.yaml" has the task "T-2" with the check "true"
    And the registry's item "T-2" is a task titled "Tidy the readme" with the status "todo"

  # The stealth ledger holds no task: it is [] until the first task add.
  @ID-INIT-25 @bug-12
  Scenario: Under a stealth config init wrote, task add writes the first task into the empty ledger
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init --stealth"
    When itos runs the command line "task add --group 1 --type chore --title 'Tidy the readme' --why 'It drifted.' --check 'true'"
    Then itos exits with code 0
    And the ledger file "tasks/phase-1.yaml" has the task "T-1" with the check "true"
    And the registry's item "T-1" is a task titled "Tidy the readme" with the status "todo"

  # Slice 59, the first half of p3-init-agent-rules (the user's calls,
  # 2026-10-03;
  # docs/decisions/0032-itos-init-generates-the-config-s-rules-into-a-marked-block-of-agents-md.md):
  # init readies agents too. What the
  # config decides (the commit types, the footer each needs, the paths each
  # may touch, what each gate runs) is generated from the config into a
  # marked block of AGENTS.md, between <!-- itos:begin --> and
  # <!-- itos:end -->; how to work with itos is itos's own guides, itos go and
  # itos guide work (decision 33), not the block; and CLAUDE.md gets a line
  # importing it, @AGENTS.md, so Claude Code reads it. The block is written
  # formatter-stable, one line per paragraph and no code span across lines,
  # so a project's formatter leaves it alone. It is offered as the plugin is
  # (slice 49): --agent-rules writes it, --no-agent-rules declines, a terminal
  # is asked, and anywhere else nothing is written and the report says how.
  # itos touches only what is inside the markers: an AGENTS.md without them
  # gains the block at its end, its text kept; a CLAUDE.md that lacks the
  # import gains it as its last line, a missing one is created holding it.
  # Run again, init reports a block the config no longer matches, naming
  # itos init --agent-rules, which rewrites only inside the markers; like the
  # plugin, it is reported and never counted as missing. The stealth half
  # (CLAUDE.local.md and Codex's AGENTS.override.md under .git/info/exclude)
  # is slice 80, below.
  @ID-INIT-26 @slice-59
  Scenario: With --agent-rules init writes the config's rules into a marked block of AGENTS.md, and CLAUDE.md imports it
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init --agent-rules"
    Then itos exits with code 0
    And the file "AGENTS.md" has the line "<!-- itos:begin -->"
    And the file "AGENTS.md" has the line "<!-- itos:end -->"
    And the file "AGENTS.md" says "Task:" between the markers
    And the file "CLAUDE.md" has the line "@AGENTS.md"

  @ID-INIT-27 @slice-59
  Scenario: Away from a terminal and with no --agent-rules, init writes no rules and says how to
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init"
    Then itos exits with code 0
    And the file "AGENTS.md" does not exist
    And its output says "--agent-rules"

  @ID-INIT-28 @slice-59
  Scenario: An AGENTS.md of the project's own keeps its text, and gains the block at its end
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "AGENTS.md" holding "Be kind to the build."
    When itos runs "init --agent-rules"
    Then itos exits with code 0
    And the file "AGENTS.md" has the line "Be kind to the build."
    And the file "AGENTS.md" has the line "<!-- itos:begin -->" after the line "Be kind to the build."

  @ID-INIT-29 @slice-59
  Scenario: Run again after the config changed, init reports the rules the config no longer matches and changes nothing
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init --agent-rules"
    And the config's commits.types gains "deps"
    When itos runs "init"
    Then its output says "itos init --agent-rules"
    And the file "AGENTS.md" does not say "deps" between the markers

  @ID-INIT-30 @slice-59
  Scenario: Run again with --agent-rules, init rewrites the block alone, the text outside the markers kept
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "AGENTS.md" holding "Be kind to the build."
    And itos has already run "init --agent-rules"
    And the config's commits.types gains "deps"
    When itos runs "init --agent-rules"
    Then the file "AGENTS.md" says "deps" between the markers
    And the file "AGENTS.md" has the line "Be kind to the build."

  # Slice 80, the stealth half of slice 59 (the user's calls of 2026-10-03,
  # in slice-59's why; asked for at once on 2026-10-05). Under a stealth
  # config nothing the project tracks changes, so --agent-rules writes where
  # git never looks: the rules block goes to <git common dir>/itos/AGENTS.md,
  # beside the stealth config and shared by every worktree. Claude Code gets
  # a CLAUDE.local.md at the worktree's top whose marked block imports that
  # file, by a path that resolves from that worktree, and @AGENTS.md too when
  # the project has one, since a CLAUDE.local.md stops Claude Code falling
  # back to AGENTS.md. Codex reads one file per folder, AGENTS.override.md
  # winning over AGENTS.md, and has no imports, so itos creates an
  # AGENTS.override.md holding a marked copy of the project's AGENTS.md,
  # between <!-- itos:agents-md:begin --> and <!-- itos:agents-md:end -->,
  # then the rules block; with no AGENTS.md, the rules block alone. Never an
  # AGENTS.md: an untracked one would make git refuse the pull the day the
  # project adds its own. Each file itos writes at the worktree's top is
  # listed in .git/info/exclude. A CLAUDE.local.md or AGENTS.override.md
  # already there is the person's: it gains only the block, never the copy,
  # its own text kept. The offer is slice 59's: --agent-rules writes,
  # --no-agent-rules declines, a terminal is asked, anywhere else nothing is
  # written and the report says how. Run again under the stealth config,
  # init reports a copy that no longer matches AGENTS.md, or rules that no
  # longer match the config, naming itos init --agent-rules, which rewrites
  # only between the markers; that is never counted as missing. v4.1.0
  # refused --stealth --agent-rules (exit 2, "not written yet"); accepting it
  # is additive (T-095 taught the release check so).
  @ID-INIT-32 @slice-80
  Scenario: With --stealth --agent-rules init writes the rules in the git folder and the files agents read, git status left clean
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init --stealth --agent-rules"
    Then itos exits with code 0
    And git status shows nothing to commit
    And the file ".git/itos/AGENTS.md" says "Task:" between the markers
    And the file "CLAUDE.local.md" names ".git/itos/AGENTS.md"
    And the file "AGENTS.override.md" says "Task:" between the markers
    And the file "AGENTS.md" does not exist
    And the file "CLAUDE.md" does not exist

  @ID-INIT-33 @slice-80
  Scenario: Under --stealth the project's AGENTS.md is imported by CLAUDE.local.md and copied, marked, into AGENTS.override.md
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "AGENTS.md" holding "Be kind to the build."
    When itos runs "init --stealth --agent-rules"
    Then itos exits with code 0
    And git status shows nothing to commit
    And the file "AGENTS.md" does not name "itos:begin"
    And the file "CLAUDE.local.md" has the line "@AGENTS.md"
    And the file "AGENTS.override.md" has the line "<!-- itos:agents-md:begin -->"
    And the file "AGENTS.override.md" has the line "<!-- itos:begin -->" after the line "Be kind to the build."

  @ID-INIT-34 @slice-80
  Scenario: Under --stealth a CLAUDE.local.md and an AGENTS.override.md of the person's keep their text and gain the blocks alone
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "AGENTS.md" holding "Be kind to the build."
    And the untracked file "CLAUDE.local.md" holding "My own notes."
    And the untracked file "AGENTS.override.md" holding "My own override."
    When itos runs "init --stealth --agent-rules"
    Then itos exits with code 0
    And the file "CLAUDE.local.md" has the line "My own notes."
    And the file "CLAUDE.local.md" has the line "@AGENTS.md"
    And the file "AGENTS.override.md" has the line "My own override."
    And the file "AGENTS.override.md" says "Task:" between the markers
    And the file "AGENTS.override.md" does not name "Be kind to the build."

  @ID-INIT-35 @slice-80
  Scenario: Under a stealth config, run again after the project's AGENTS.md changed, init reports the stale copy and changes nothing
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "AGENTS.md" holding "Be kind to the build."
    And itos has already run "init --stealth --agent-rules"
    And the committed file "AGENTS.md" holding "Be kinder to the build."
    When itos runs "init"
    Then itos exits with code 0
    And its output says "itos init --agent-rules"
    And the file "AGENTS.override.md" does not name "Be kinder to the build."

  @ID-INIT-36 @slice-80
  Scenario: Under a stealth config, run again with --agent-rules, init refreshes the copy and the rules, git status still clean
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "AGENTS.md" holding "Be kind to the build."
    And itos has already run "init --stealth --agent-rules"
    And the committed file "AGENTS.md" holding "Be kinder to the build."
    And the config's commits.types gains "deps"
    When itos runs "init --agent-rules"
    Then itos exits with code 0
    And the file "AGENTS.override.md" has the line "Be kinder to the build."
    And the file "AGENTS.override.md" says "deps" between the markers
    And the file ".git/itos/AGENTS.md" says "deps" between the markers
    And git status shows nothing to commit

  @ID-INIT-37 @slice-80
  Scenario: Under --stealth, away from a terminal and with no --agent-rules, init writes no rules and says how to
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init --stealth"
    Then itos exits with code 0
    And the file "CLAUDE.local.md" does not exist
    And the file "AGENTS.override.md" does not exist
    And its output says "--agent-rules"

  # Slice 95 (the user's call, 2026-10-06). Every session reads the block,
  # whatever its role, and nothing in it said which guide is whose, so a
  # session in a repository that adopted itos found neither itos go nor the
  # repository's own notes. The block gains one paragraph: the session the
  # person talks to coordinates and runs itos go, which ends with the
  # repository's own notes, the file guide.orchestrating resolves to, named
  # from the repository's top; a session handed an item runs itos guide work.
  # It points at the guides and holds none of their text (decision 33): how
  # to work with itos stays in the guides, versioned with the binary. Under
  # --stealth the notes resolve under the git folder, as the config's other
  # paths do, and the block names them there.
  @ID-INIT-38 @slice-95 @wip
  Scenario: The rules for agents send the coordinator to itos go and an implementing session to itos guide work
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init --agent-rules"
    Then itos exits with code 0
    And the file "AGENTS.md" says "itos go" between the markers
    And the file "AGENTS.md" says "itos guide work" between the markers
    And the file "AGENTS.md" says "docs/ORCHESTRATING.md" between the markers

  @ID-INIT-39 @slice-95 @wip
  Scenario: Under --stealth the rules for agents name the notes itos go appends where the stealth config puts them
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init --stealth --agent-rules"
    Then itos exits with code 0
    And the file ".git/itos/AGENTS.md" says ".git/itos/docs/ORCHESTRATING.md" between the markers

  @ID-INIT-31 @bug-12
  Scenario: In a repository init set up, a docs commit touching code is refused, so docs cannot skip the Task footer
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init"
    And the files init wrote are committed
    And a change to "src/app.js" is staged
    When the commit-msg hook checks the message "docs: describe the app"
    Then itos exits with code 1
    And its output says "docs commits may not touch src/app.js"

  # Bug 43 (the code review of 07b0e6d, 2026-10-05): init --stealth in a
  # repository whose itos.yaml is at its top went ahead, and with
  # --agent-rules wrote the stealth rules files: a copy of an AGENTS.md
  # already holding the block, gone stale unreported, naming a stealth config
  # that does not exist. A stealth config is for a clone that cannot change
  # the project, so where the project has its own config, --stealth is a
  # usage error (exit 2) naming itos.yaml, and nothing is written.
  @ID-INIT-40 @bug-43
  Scenario: init --stealth where the project has its own itos.yaml is a usage error, and writes nothing
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init"
    And the files init wrote are committed
    When itos runs "init --stealth --agent-rules"
    Then itos exits with code 2
    And its output says "itos.yaml"
    And the file "CLAUDE.local.md" does not exist
    And the file "AGENTS.override.md" does not exist
    And git status shows nothing to commit
