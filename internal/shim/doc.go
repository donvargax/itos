// Package shim is itos started as git (features/shim.feature): a link named
// git to the itos binary, in a folder before the real git on the PATH (itos
// git-shim install, the link install.go places and makes), makes every git command pass through itos first. In a
// repository itos manages (config.Managed, file checks alone), git commit
// runs as itos commit and git push as itos push, with git's arguments, none
// of them read as itos's global flags; every other command, every command in
// any other folder, and every git started under an itos run (ITOS_GIT set,
// EnvGit) runs the real git with its arguments, its streams and its exit
// code untouched. The pass-through costs a start, the PATH's lookup and a
// few file checks: no git runs before the real one, and neither the launcher
// nor the command line is reached.
//
// cmd/itos asks Named(os.Args[0]) first (base name git, or on windows
// git/git.exe in any case) and, when it is, Main before anything else. The
// real git is the git adapter's Inherited or Real (internal/vcs/adapter,
// on the platform cmd/itos gives it); none is exit 3.
// Otherwise parse reads git's options before the command, and for commit or
// push, with no GIT_DIR or GIT_WORK_TREE, config.Managed decides. Then enter
// moves to the -C folder, keeping the one to come back to, and tooOld asks
// launch.Handed, with the launch.CLI cmd/itos gives Main, the version the
// launcher would hand git-shim run to
// (ITOS_VERSION when it is a version, else the pin, read as the launcher
// reads it; never the newest release, and nothing when it is this binary's
// version). In a repository pinned to an itos older than the shim
// (GitShimSince), that itos has no git-shim command to hand git commit
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
// In a repository itos manages, git add and git stage that stage more than
// the files they name are the shim's own to refuse (slice 128, add.go), exit
// 2, saying to name the files with git add <path>, and no git runs: -A, --all,
// --no-ignore-removal, -u or --update, as a cluster's letter too (-Av) and as
// git abbreviates the long ones, or a path that is a folder, . among them,
// before -- or after it. sweeping reads git add's options as git reads them,
// a valued option's value (-U 3, --chmod +x) neither an option nor a path. A
// file named by its path, one that is not there, and every git add elsewhere
// run the real git. git commit's and git push's refusals (--no-verify, -n,
// -a, a -c that sets core.hooksPath) are itos commit's and itos push's, which
// read the -c the shim hands on in GIT_CONFIG_KEY_<n>.
//
// Where the link goes and what stands there (install.go) is what itos
// git-shim install and uninstall and itos init's offer of the shim share
// (internal/cli's gitshim.go and initshim.go). Place is the running itos
// (os.Executable, which the caller gives) and the link's path in a folder, by
// default the one holding that itos, made absolute, named git, or git.exe on
// the OS windows (Name). LinksItos is what a git there is: this itos, its
// links followed (the git adapter's IsSelf), or another itos: a symbolic link
// to a binary named itos, as an older install's, or what the git adapter's
// IsItos tells is one, a hard link to it or a copy. MakeLink makes the symbolic link,
// replacing the one there when told to, and on windows, where symbolic links
// need a privilege, a hard link where that fails. PathStanding places the
// link's folder and the real git's (the git adapter's Real) among the PATH's
// folders by os.SameFile, and StandingLine says where that leaves the shim.
// Reading the arguments, the messages of a refusal and the exit codes are
// internal/cli's.
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
