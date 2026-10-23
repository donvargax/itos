# The in-process tier

ADR-0059 decides it; this is the order. The feature scenarios stay one
specification with one set of steps and two drivers, as itos-cc runs them:
locally in process over per-run fakes, from the pre-push hook; in CI the
built binary with real git and the real filesystem on Linux, macOS and
Windows. Until the last step, CI owns task checks and the push proof
(ADR-0057), and agents run neither by hand.

## Where it starts (2026-10-23)

Every scenario runs the binary (`features/steps_test.go`, the step that
starts itos). The spike (branch `spike-in-process`, 5c6e8afb) put a second
driver behind `ITOS_FEATURE_DRIVER=in-process` that calls `internal/cli.Main`
and captures streams and the exit code: `config.feature` passed 54 of 54 and
`guard.feature` 11 of 12, the twelfth needing the launcher. It ran serially,
saving and restoring process state around each run, and took as long as the
binary: scenario setup and real git dominate.

What blocks parallel runs: `cli.applyGlobals` sets `ITOS_CONFIG` and
`NO_COLOR` with `os.Setenv` and moves the process with `os.Chdir`; `getwd` is
a package variable; about fifteen production sites read `os.Stdin` directly
(guard, init, tests, the hooks, commit, the work and decision writes, the ci
and check runs, gitrun).

## The steps

Each is one task or slice; none changes what a consumer sees until step 6.

1. **The driver contract.** A run value (args, folder, environment, stdin)
   and result (stdout, stderr, exit); `@binary-only` with a reason for what
   needs the process boundary (launcher, shim, signals, pin), as itos-cc tags
   it; a run that selects nothing or skips a scenario is never a pass. The
   in-process driver lands serial, behind its switch, with the binary driver
   unchanged as the control (T-285, done). The push proof already counts
   the lines scenarios reach through the binary driver (T-121's `-cover`
   build, read by itos-cc since T-120), so in-process coverage waits for
   step 5; `cmd/itos`, the launcher and the shim stay judged by binary runs.
2. **No process state in the CLI.** `cli.Main`'s path takes the folder,
   environment and stdin from the run value: no `os.Chdir`, `os.Setenv`,
   `getwd` variable or `os.Stdin` read. A test runs two scenarios at once
   and shows neither sees the other's. Then the driver runs in parallel.
   The run value reaches the code as explicit fields: `cli.Main` builds it
   and hands it to each command's ports and adapters when it constructs
   them, never through a `context.Context` value (q-101); `os.*` stays in
   `cmd/itos`, the launcher and the shim. Four tasks, in order: the working
   folder, the environment, stdin, then the two-at-once test and the
   parallel driver.
3. **Fakes per run.** The test binary binds the VCS, tree, tracker and
   private-store fakes (their `porttest` packages, held by the contract
   suites) per scenario, with a test that every port has a binding. Pilot
   on config, then one git-backed area. This is where the speed comes from.
4. **Area by area.** Each feature area moves when its OS and process
   dependencies are behind ports, timed as it moves; what stays a binary
   run is tagged with its reason. The spike guesses 500 to 600 of the 923
   scenarios can move: a guess, measured per area.
5. **This repository's pre-push tier.** The pre-push hook runs, for the
   push's range: the in-process scenarios of the areas it reaches, the unit
   tests, changed-line coverage over both, and one mutant of the changed
   functions at CI's seed; under 2 minutes warm, failing past 5 with the
   timing. A gate change (`build`, `ci`), no release.
6. **The product: task checks at pre-push, opt-in.** A `done_when` check
   may declare its local command beside `run:` (an optional ledger key); a
   config key makes the pre-push hook run the checks of the tasks a push
   names, each failure naming task, command, exit and output, and push and
   ci watch carry that detail. A check with no local command blocks the
   local run, never passes. Feats, minor releases; this repository turns the
   key on, and ADR-0057's CI-only ownership ends here.
7. **v9.** Pre-push becomes the default place for task checks instead of the
   commit-msg hook, which changes `commit-tasks.feature`; it joins the v9
   bundle (q-89).
