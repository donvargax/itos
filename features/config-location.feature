@phase-1
Feature: A runtime project config belongs to the working tree
  # Issue #31, q-79. The boundary is the current Git worktree top when
  # inside one, otherwise the effective execution directory after --root
  # or ITOS_ROOT. Finding the boundary does not chdir or rebase config data.
  # A normal project config must resolve within that boundary. Compare
  # cleaned absolute paths by path components, resolving symlinks (including
  # parent links), with the platform's filesystem semantics; a string prefix
  # such as project-other is not containment. Do not follow config data paths
  # to infer the boundary or permit another repository's config.
  # The existing recognized private stealth config remains an exception,
  # including an external common Git directory; arbitrary files in .git or
  # folders named itos do not gain the exception. No new external data mode.
  # Before parsing an unsupported external runtime file, invoking an adapter,
  # reading a ledger or launching/fetching a pinned runtime, return config
  # usage exit 2 and config-outside-project with the requested path, working
  # boundary and a fix: use a config in this project, --root to operate on
  # its owning project, or supported init --stealth for private setup.
  # JSON uses the existing schema/ok/problems envelope, no success payload.
  # In-bound missing/unreadable configs retain their existing diagnostics.
  # --policy is an input document, not the effective runtime config, and may
  # be external. Parsing in-memory policy/default/schema text stays usable.
  # Help/version/defaults that do not need a config keep their existing
  # fallback behavior. Do not create files, change refs or substitute a local
  # config silently. State the supported boundary in help/docs; do not claim
  # branch-specific plans or data beside arbitrary config files now work.

  @wip @slice-160 @ID-CONFIG-61
  Scenario: An external clone's config is refused before misleading data-path errors
    Given separate work and data clones with a config and metadata only in data
    And ITOS_CONFIG names the data clone's absolute config path
    When itos runs "config check --json" in work
    Then itos exits with code 2
    And its problem rule is "config-outside-project"
    And its diagnostic identifies the requested config and work boundary
    And its fix explains the supported project-root and stealth alternatives
    And no missing-ledger diagnostic or metadata write occurs

  @wip @slice-160 @ID-CONFIG-62
  Scenario: Command-line environment and root selection enforce the same runtime boundary
    Given an external project config whose data would otherwise resolve successfully in work
    When config-dependent read and write commands use it through --config or ITOS_CONFIG
    Then each exits with code 2 before adapters checks metadata writes or commits
    When --root or ITOS_ROOT selects the config's owning project instead
    Then the config uses that project's existing data-resolution semantics
    And the caller's former working project remains untouched

  @wip @slice-160 @ID-CONFIG-63
  Scenario: In-tree alternative configs keep their root-relative data model
    Given a working tree with its runtime config in a nested config directory
    And the config's data paths resolve from the effective working project as currently supported
    When itos loads that config through a relative or absolute --config path
    Then no outside-project refusal occurs
    And data is not silently rebased beside the config
    And automatic top-level config discovery keeps its current behavior

  @wip @slice-160 @ID-CONFIG-64
  Scenario: Symlinks and sibling prefixes do not conceal an external config
    Given an in-tree config symlink whose resolved file is in a sibling project
    And a sibling directory whose name begins with the working project's full name
    When each external config is selected for a config-dependent command
    Then itos exits with code 2 and "config-outside-project"
    And neither config's data or pinned runtime is used
    When a selected symlink resolves to an actual config within the working tree
    Then location validation permits it and existing parsing rules apply

  @wip @slice-160 @ID-CONFIG-65
  Scenario: The recognized private stealth config remains supported
    Given a repository whose common Git directory is outside its working tree
    And its existing recognized private itos config and data are in that Git directory
    When itos uses automatic stealth discovery or explicitly selects that same private config
    Then existing stealth commands and private path rebasing remain supported
    And no tracked project file is changed
    When an unrelated external config merely uses an itos directory name
    Then it is refused with code 2 and "config-outside-project"

  @wip @slice-160 @ID-CONFIG-66
  Scenario: Unsupported location is detected before pinned execution
    Given an external runtime config with a pin and observable provider or extension commands
    When a config-dependent itos invocation selects it
    Then itos exits with code 2 and "config-outside-project"
    And no pinned binary launch release fetch or configured command occurred
    And no local config was selected as a fallback

  @wip @slice-160 @ID-CONFIG-67
  Scenario: External policy import remains distinct from runtime config selection
    Given a fresh project and a valid external policy input
    When itos runs init with that --policy input and no external runtime config
    Then policy validation and local initialization follow their existing contract
    And the runtime config written belongs to the project or its supported stealth location
    When an external runtime --config is supplied as well
    Then the existing policy/runtime-option refusal or outside-project refusal occurs before writes

  @wip @slice-160 @ID-CONFIG-68
  Scenario: Missing in-tree config and config-free commands retain their existing outcomes
    Given missing and unreadable in-tree runtime config fixtures
    When a config-dependent command loads each
    Then it reports the existing missing or unreadable config outcome
    Given no Git repository and an effective execution directory
    When an explicit runtime config resolves outside that directory
    Then a config-dependent command refuses it with code 2 and "config-outside-project"
    And ordinary help version and print-defaults fallback behavior is unchanged
