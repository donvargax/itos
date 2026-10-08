// Package shim is itos started as git (features/shim.feature): a link named
// git to the itos binary, in a folder before the real git on the PATH (itos
// git-shim install), makes every git command pass through itos first. In a
// repository itos manages (config.Managed, file checks alone), git commit
// runs as itos commit and git push as itos push, with git's arguments, none
// of them read as itos's global flags; every other command, every command in
// any other folder, and every git started under an itos run (ITOS_GIT set,
// git.EnvGit) runs the real git with its arguments, its streams and its exit
// code untouched. The pass-through costs a start, the PATH's lookup and a
// few file checks: no git runs before the real one, and neither the launcher
// nor the command line is reached.
//
// cmd/itos asks Named(os.Args[0]) first (base name git, or on windows
// git/git.exe in any case) and, when it is, Main before anything else. The
// real git is git.Inherited or git.Real (internal/git); none is exit 3.
// Otherwise parse reads git's options before the command, and for commit or
// push, with no GIT_DIR or GIT_WORK_TREE, config.Managed decides. Then enter
// moves to the -C folder, keeping the one to come back to, and tooOld asks
// launch.Handed the version the launcher would hand git-shim run to
// (ITOS_VERSION when it is a version, else the pin, read as the launcher
// reads it; never the newest release, and nothing when it is this binary's
// version). In a repository pinned to an itos older than the shim
// (cli.GitShimSince), that itos has no git-shim command to hand git commit
// or git push to, so the shim says so in one line on stderr, moves back and
// runs the real git with the arguments as they came; a folder it cannot come
// back to hands the command on. Otherwise configure appends each -c to
// GIT_CONFIG_COUNT, GIT_CONFIG_KEY_<n> and GIT_CONFIG_VALUE_<n> and sets
// ITOS_GIT, and the arguments become git-shim run -- <command> <args>…,
// which go through launch.Main and cli.Main as any run's: the -- keeps every
// git argument from the global flags, and git-shim run calls itos commit or
// itos push directly, so git commit check-paths stays a commit. Anything else
// is run: syscall.Exec of the real git, arguments and environment untouched,
// on unix, after writing the process's coverage counters to GOCOVERDIR when
// it is set, since an exec runs no exit hook to write them (T-124); elsewhere a child with the terminal's streams, an interrupt left
// to it and its exit code handed back.
//
// Of git's options before the command, -C <path> and -c <name>=<value> are
// honoured, since tools write git -C <dir> commit and git -c <key>=<value>
// commit (an editor's, say) for an ordinary commit: the shim moves to the
// folder -C names, joined as git joins them, before it looks for the config,
// and hands each -c to every git itos runs. --no-pager and -P change nothing
// itos prints. Any other option before the command (--git-dir, --work-tree,
// --bare, --namespace, --exec-path, …), or GIT_DIR or GIT_WORK_TREE in the
// environment, names a repository the file checks do not follow, so that
// command runs the real git as it is.
package shim
