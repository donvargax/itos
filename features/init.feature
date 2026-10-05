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
  binary that was called). Then it installs the hooks, as hooks install
  does. With --stealth all of it goes under the git folder and the hooks into
  the git config, nothing the project tracks touched.

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
    And the file ".git/hooks/commit-msg" calls itos
    And the file ".git/hooks/pre-push" calls itos
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
    And the file ".git/hooks/pre-push" is removed
    When itos runs "init"
    Then itos exits with code 1
    And its output says "pre-push"
    And its output says "itos hooks install"
    And the file ".git/hooks/pre-push" does not exist

  @ID-INIT-09 @slice-48
  Scenario: With --stealth init writes everything under the git folder and declares the hooks in the git config
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init --stealth"
    Then itos exits with code 0
    And git status shows nothing to commit
    And the git config declares a "commit-msg" hook that runs itos
    When itos checks the config
    Then itos exits with code 0

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
    And the file ".git/hooks/commit-msg" calls itos

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
    When itos runs the command line "task add T-2 --group 1 --type chore --title 'Tidy the readme' --why 'It drifted.' --check 'true'"
    Then itos exits with code 0
    And the ledger file "tasks/phase-1.yaml" has the task "T-2" with the check "true"
    And the registry's item "T-2" is a task titled "Tidy the readme" with the status "todo"

  # The stealth ledger holds no task: it is [] until the first task add.
  @ID-INIT-25 @bug-12
  Scenario: Under a stealth config init wrote, task add writes the first task into the empty ledger
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init --stealth"
    When itos runs the command line "task add T-1 --group 1 --type chore --title 'Tidy the readme' --why 'It drifted.' --check 'true'"
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
  # is p3-init-agent-rules-stealth.
  @ID-INIT-26 @slice-59 @wip
  Scenario: With --agent-rules init writes the config's rules into a marked block of AGENTS.md, and CLAUDE.md imports it
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init --agent-rules"
    Then itos exits with code 0
    And the file "AGENTS.md" has the line "<!-- itos:begin -->"
    And the file "AGENTS.md" has the line "<!-- itos:end -->"
    And the file "AGENTS.md" says "Task:" between the markers
    And the file "CLAUDE.md" has the line "@AGENTS.md"

  @ID-INIT-27 @slice-59 @wip
  Scenario: Away from a terminal and with no --agent-rules, init writes no rules and says how to
    Given a repository that does not use itos, its one commit "docs: start"
    When itos runs "init"
    Then itos exits with code 0
    And the file "AGENTS.md" does not exist
    And its output says "--agent-rules"

  @ID-INIT-28 @slice-59 @wip
  Scenario: An AGENTS.md of the project's own keeps its text, and gains the block at its end
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "AGENTS.md" holding "Be kind to the build."
    When itos runs "init --agent-rules"
    Then itos exits with code 0
    And the file "AGENTS.md" has the line "Be kind to the build."
    And the file "AGENTS.md" has the line "<!-- itos:begin -->" after the line "Be kind to the build."

  @ID-INIT-29 @slice-59 @wip
  Scenario: Run again after the config changed, init reports the rules the config no longer matches and changes nothing
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init --agent-rules"
    And the config's commits.types gains "deps"
    When itos runs "init"
    Then its output says "itos init --agent-rules"
    And the file "AGENTS.md" does not say "deps" between the markers

  @ID-INIT-30 @slice-59 @wip
  Scenario: Run again with --agent-rules, init rewrites the block alone, the text outside the markers kept
    Given a repository that does not use itos, its one commit "docs: start"
    And the committed file "AGENTS.md" holding "Be kind to the build."
    And itos has already run "init --agent-rules"
    And the config's commits.types gains "deps"
    When itos runs "init --agent-rules"
    Then the file "AGENTS.md" says "deps" between the markers
    And the file "AGENTS.md" has the line "Be kind to the build."

  @ID-INIT-31 @bug-12
  Scenario: In a repository init set up, a docs commit touching code is refused, so docs cannot skip the Task footer
    Given a repository that does not use itos, its one commit "docs: start"
    And itos has already run "init"
    And the files init wrote are committed
    And a change to "src/app.js" is staged
    When the commit-msg hook checks the message "docs: describe the app"
    Then itos exits with code 1
    And its output says "docs commits may not touch src/app.js"
