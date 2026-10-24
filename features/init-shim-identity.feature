@phase-1
Feature: A pinned init recognizes the launcher shim already effective on PATH
  # #53: cached runtime identity differs from the stable launcher identity.
  # With no --git-shim-dir, consider only the first executable Git on PATH
  # and recognize it with git.IsItos (internal/git/bin.go), reached through
  # a new IsItos method on initcmd's Ports, bound in internal/cli/init.go;
  # shim.LinksItos (internal/shim/install.go) stayed unchanged here; bug-128
  # later made it read a hard link or copy of another itos too. A recognized
  # first Git returns "kept" at once, before the existing-link check in
  # internal/slices/initcmd/shim.go, and its text line says "The git shim is
  # there already: <path> is itos." naming no target, so it never names the
  # cached runtime. git-shim install and internal/launch are out of scope.
  # Keep an effective itos shim for plain init and explicit --git-shim;
  # never replace that launcher with the cached runtime or install another
  # cache link. Reporting must identify the actual entry and never invent
  # a link target for a hard link/copy or claim it targets the runtime.
  # Preserve the git_shim JSON shape, kept action, link and PATH standing.
  # No candidate process is run merely to identify it. Do not scan past an
  # earlier real Git and describe a later shim as effective. --no-git-shim
  # and explicit --git-shim-dir retain their precedence and refusal rules.
  # The rest of init still runs under the selected pin (bug-104).
  # New/missing-shim installation and cache-path advice are separately
  # p1-pinned-shim-install-target. No guard or default-policy change here.
  # All launcher/cache/release fixtures are local; no live update service.

  Background:
    Given a repository with a valid pin and completed itos setup
    And init executes the itos under test from that pin's cache
    And a distinct global itos launcher exists in a stable PATH folder
    And the cache folder is not on PATH
    And a real Git executable follows the stable folder on PATH

  @ID-SHIMINIT-01 @bug-113
  Scenario Outline: A cached runtime reports the active global launcher shim as already kept
    Given the first Git on PATH links to the global launcher rather than the running runtime
    When itos runs "<command>"
    Then itos exits with code 0
    And its shim report identifies that first Git path as kept and effective before real Git
    And JSON, when requested, retains the existing git_shim fields and truthful on_path and before_git values
    And text, when requested, does not call the effective shim missing or advise another cache link
    And text, when requested, does not name the cached runtime's path
    And neither the global shim nor its target changed
    And no Git shim was created in the runtime cache

    Examples:
      | command                               |
      | init --plugin no --no-agent-rules      |
      | init --plugin no --no-agent-rules --json |

  @ID-SHIMINIT-02 @bug-113
  Scenario: Explicit installation keeps an already effective launcher rather than replacing it with the pin
    Given the first Git on PATH links to the global launcher rather than the running runtime
    When itos runs "init --plugin no --no-agent-rules --git-shim --json"
    Then itos exits with code 0
    And git_shim action is "kept" and link names the original stable Git path
    And the original target and executable identity are unchanged
    And the pinned runtime's cache gained no Git link
    And the rest of init was handled by the selected pinned runtime, not handed back to the launcher

  # On Windows without symlink privilege the harness falls back to a hard
  # link (features/shim_test.go), so the symbolic-link rows then test a hard
  # link: accepted, the hard-link row being recognized the same way. Running
  # the candidate cannot be observed for a Go binary, so no step claims it;
  # @ID-SHIMINIT-04's wrapper can leave a marker and keeps that check.
  @ID-SHIMINIT-03 @bug-113
  Scenario Outline: Existing cross-binary identity forms are recognized without executing a probe
    Given the first Git on PATH is <identity> of the distinct global itos
    When itos runs "init --plugin no --no-agent-rules --json"
    Then itos exits with code 0 with that path kept as the effective shim
    And the candidate's content, link representation and target are unchanged
    When itos runs "init --plugin no --no-agent-rules"
    Then its text says that first Git path is itos and does not name the cached runtime's path

    Examples:
      | identity                              |
      | an absolute symbolic link             |
      | a relative symbolic link              |
      | a valid symbolic-link chain           |
      | a hard link to an itos Go binary       |
      | a native copy retaining itos build information |

  # @ID-SHIMINIT-04 and -05 pass before the fix: guards that the fix keeps
  # today's behaviour; only -01 to -03 must fail first.
  @ID-SHIMINIT-04 @bug-113 @guard
  Scenario: A later shim and an unrecognized Git wrapper are not reported as the active itos shim
    Given real Git comes before the launcher shim on PATH
    And the cache has no Git link
    When itos runs "init --plugin no --no-agent-rules --json"
    Then its report does not claim that later launcher shim is effective
    And it has not scanned past the first executable Git to keep that later path
    When the first entry is instead an unrecognized executable Git wrapper
    Then that wrapper is not treated as itos merely from its Git filename
    And init neither executes it for identification nor replaces it

  @ID-SHIMINIT-05 @bug-113 @guard
  Scenario: Explicit decline and directory choice keep their existing precedence
    Given the first Git on PATH is an effective global itos shim
    When itos runs "init --plugin no --no-agent-rules --no-git-shim --json"
    Then itos exits with code 0 with git_shim action "declined" and a null link
    And the effective shim is unchanged
    Given an explicitly requested shim directory contains an ordinary Git executable
    When init is run with --git-shim and --git-shim-dir naming that directory
    Then itos exits with code 1 using the existing refusal to replace ordinary Git
    And the requested executable and the global shim are unchanged
    And it has not substituted the already active PATH shim for the explicitly requested location
