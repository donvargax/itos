# Container sandbox extension for itos

## Status and intent

Design draft for an experiment in a separate project, requested on 2026-10-08. The direction
is an itos extension, provisionally named `itos-container`, invoked as `itos container`.
The commands and configuration below are proposed interfaces, not commands installed today.
No implementation or new work item is authorized by this document.

The extension starts a coding agent and its tools inside a restricted container for one
repository. It owns the container lifecycle, mounts and network policy. Core itos keeps the
ledger, work routing, commit rules and CI proof; it gains no container-specific behavior.

The first experiment should answer a concrete question: can a person run OpenCode with
ChatGPT/Codex authentication, make a change, commit through the project's hooks and read its
CI failure, without mounting the person's home or giving the agent a Docker socket?

## What exists today

T-108 built `.devcontainer/` for unattended Claude Code in this itos checkout. Its useful
parts are a writable checkout, protected git configuration, a root-owned firewall applied
at startup, named volumes for tool state and pinned toolchains. Its installed Go, pnpm,
Vite+, Claude Code and itos versions, and its hostname allowlist, are repository-specific.

The existing network filter combines dnsmasq's name allowlist with resolved IP addresses
and GitHub's published network ranges. That is IP-based permission, not strict isolation
to named services: another site on an allowed CDN address can remain reachable. The script
warns and continues if IPv6 filtering is unavailable. Neither behavior is sufficient evidence
for the stronger guarantees proposed here.

The registry records manual probes for T-108, not automated container validation. It leaves
Docker's embedded DNS resolver, volume ownership for a UID other than 1000, arm64 runtime
behavior and downloading failed Actions logs as gaps. Extract the mechanisms and their
tests; do not copy the current image and describe those gaps as solved.

## Scope and non-goals

The pilot supports one checkout per session, a local Docker engine and a Linux workload.
Start with Linux amd64. Claim arm64 or Docker Desktop support only after their runtime
acceptance tests pass. Podman and remote Docker contexts can follow after the first project
has exercised the contract.

Included: explicit startup, command execution, stopping and inspection; host preparation
checks; restricted egress; persistent state scoped to a project; and a repeatable probe suite.
The workload can be OpenCode, Claude Code, a shell or another program supplied as argv.

Excluded: scheduling work items, creating worktrees, choosing agent models, forwarding all
host credentials, changing repository hooks, running arbitrary cleanup over Docker resources,
or automatically installing extensions through core itos. Editor-specific integration is
optional and must not be needed to use the terminal workflow.

## Threat model and trust boundary

Assume the agent can execute arbitrary code, dependencies can run hostile install scripts,
and repository text can contain prompt injection. The person, host-side extension, approved
images, network-control component and container engine are trusted. An engine administrator
can inspect the session and its secrets; containers do not defend against that administrator
or a kernel/container escape.

The agent is allowed to change or delete files inside its mounted checkout. This design does
not protect that checkout from the agent; backups, git history and human review still matter.
It must not make other host folders, devices, sockets or processes available to the workload.

The sandbox also does not make git hooks or the itos shim impossible to bypass. The current
container documentation records direct git executables and hook-skipping flags as gaps.
Core itos and CI remain responsible for judging commits; filesystem and network isolation
must hold even when the agent invokes a program outside that shim.

Mounting a repository into a container is not enough. The agent process and every tool that
executes code or reads files for it must run there too. A host-side MCP server, shell tool or
editor integration is a separate escape from that boundary unless explicitly constrained.
The host coordinator is outside this sandbox and retains its own permissions.

After a sandbox writes a checkout, run its hooks, builds, commits and pushes inside the
sandbox until a person reviews the changes. A host commit hook or host OpenCode plugin
can execute code the agent wrote. For git-based inspection, disable external diff and text
conversion (`git diff --no-ext-diff --no-textconv`). Running a host build, hook or dependency
install is a new trust decision.

## User workflow

This is the proposed first-project workflow; replace the image and network entries before use:

```sh
# Install a verified itos-container release on an absolute PATH directory.
itos container init
# Review itos-container.yaml and approve the image, mounts and network policy.
itos container doctor
itos container doctor --probe
itos container run -- opencode --standalone
```

`init` writes an example extension config only when none exists. It does not overwrite
`.devcontainer/`, install hooks, create provider accounts, start Docker resources or download
an image. The person prepares the project's hooks before the first restricted run.

`run` creates a session and attaches to its command. If no command is supplied, it opens the
image's configured shell. A command is an argv vector, never implicitly evaluated by a shell.
Normal exit removes the session containers and network, retaining workspace edits and
project-scoped tool state. If teardown fails, print the session ID and the exact retry command.

Other proposed commands:

| Command                                      | Contract                                                                               |
| -------------------------------------------- | -------------------------------------------------------------------------------------- |
| `itos container exec <session> -- <argv...>` | Enter a running workload with the same user, mounts and restrictions.                  |
| `itos container status`                      | List this project's sessions and effective policy, without credentials.                |
| `itos container stop <session>`              | Stop and remove that session's containers and network, preserving named state volumes. |
| `itos container doctor`                      | Check engine, config, images and host preparation without running repository code.     |
| `itos container doctor --probe`              | Run disposable isolation/network probes; create no coding-agent session.               |

Use an explicit session ID for mutating operations. An ambiguous name, a resource without
the extension's ownership labels or one belonging to another project is refused. The pilot
has no command to delete persistent authentication volumes; that needs a separate explicit
contract rather than being hidden in `stop`.

## Extension and configuration contract

The existing extension mechanism already supplies the integration boundary:

- Put an executable `itos-container` on the host PATH. Core itos dispatches `itos container`
  to it, passing arguments after `container` unread; built-in commands always win.
- `ITOS_ROOT` identifies the effective project root and `ITOS_CONFIG` names core's config.
  Do not assume the caller's original subdirectory is available: itos runs the extension
  from its effective root.
- `ITOS_BIN` and `ITOS_VERSION` identify the host itos. A host callback uses `ITOS_BIN`;
  never copy that absolute host executable path into the container environment.
- `ITOS_JSON=1` requests machine output. `init`, `doctor`, `status` and `stop` return one
  object with `schema: 1`, `ok` and typed problems; logs go to stderr. Interactive `run`
  and `exec` refuse JSON mode with a usage problem instead of mixing terminal output and JSON.
- Wrapper failures follow the documented extension convention: 1 policy refusal, 2 usage,
  3 missing environment. After a command starts, `run` and `exec` preserve its exit code and
  report cleanup failures on stderr. If it succeeded but cleanup failed, return nonzero.

Use a separate `itos-container.yaml`. Core's strict schema should not accept keys for a
feature it does not own. Resolve extension paths from `ITOS_ROOT`, reject unknown keys and
refuse an absent configuration instead of starting an unrestricted default container.

Proposed minimal config, with deliberately non-runnable image placeholders:

```yaml
version: 1
runtime: docker
image: "<reviewed-workload-image>@sha256:<digest>"
guard_image: "<reviewed-network-control-image>@sha256:<digest>"
network:
  allow:
    - host: github.com
      ports: [443]
      why: fetch and push this project's git repository
    - host: api.github.com
      ports: [443]
      why: watch CI through the GitHub API
state:
  profiles: [opencode] # Extension-defined container paths, never host home mounts.
```

The image carries the project's toolchain and hooks' prerequisites. The extension does not
infer that every project needs Go, pnpm or Vite+. Image digests and extension versions must
be explicit; image building and its downloads are a separate, person-approved operation.
Reuse a cached image matching the digest rather than fetching it on every start.
Provision a container-native itos at the project's verified pin and a git version that supports
its hook manager. Building an image or preparing hooks can execute repository-controlled
code, so neither belongs in an unannounced host-side `doctor` check.

Add model-provider, authentication, registry and release-download hosts for the actual
workflow only after observing and reviewing them. Record a reason for every permission.
Literal IPs, suffix wildcards and arbitrary Docker arguments are not part of the pilot config.
The effective default is deny, with no permissive fallback for missing hosts.

## Runtime design

Recommended architecture for the experiment: separate the workload from a trusted network
guard. The guard owns their network namespace and installs the policy before the workload
starts. The worker joins that namespace without gaining the guard's administrative rights.
This is a proposed architecture to validate, not an isolation guarantee proven by T-108.

The guard image is fixed, root-owned and has no workspace, authentication state or Docker
socket mounted. Only the guard receives the capabilities its network setup needs. It exposes
the restricted resolver to the worker and no administrative control API. A workload cannot
change the rules by asking the guard to execute a command.

The worker runs as the host user's UID/GID with all capabilities dropped and
`no-new-privileges`; it has no sudo, Docker socket, SSH-agent socket, host networking,
host PID namespace or broad host mount. A read-only image root and explicit writable state
mounts are the default. Supply temporary directories without mounting host `/tmp`.
The experiment must verify the guard/worker namespace and privilege separation on the
actual engine before it runs with valuable credentials.

Bind only the selected checkout, at the same absolute path on each side to preserve absolute
hook and tool paths. Protect `.git/config` and `.git/hooks` as read-only mounts, and use a
minimal container-owned git identity file instead of the host's complete `.gitconfig`.
Refuse a missing preparation file rather than silently creating it during startup.

The pilot uses a clone, not a linked worktree: worktrees can name a git common directory
outside the mount. Refuse that layout with an explanation until a tested least-privilege
mount design exists. Do not silently mount the main checkout or the containing directory.

State volumes belong to one canonical project identity, user and profile; two checkouts
with the same basename must not share credentials accidentally. Provision ownership before
the worker starts, without giving it a general-purpose `chown` or sudo escape. A fresh run
works for a UID other than 1000. Shared tool caches are optional and need a poisoning model;
separate per-project caches are the pilot default.

Snapshot the approved policy outside the writable checkout before starting the worker.
`exec` reuses that policy. A change the agent makes to `itos-container.yaml` cannot loosen an
existing session; a new policy requires a host-side review/approval step, not an agent-driven
restart. State transitions are preparing, ready, running, stopping, stopped or failed. Mark
ready only after mounts, identity and both address families' network controls are checked.

## Credentials and network access

Do not inherit the host environment wholesale. Provider login is explicit and provider
state persists in an isolated project profile. For OpenCode, authenticate inside the
container through its supported account flow; do not mount or copy the host's entire
OpenCode SQLite database. A ChatGPT/Codex subscription uses its OAuth connection, not an
API key. Test login and refresh with the selected provider before declaring it usable.

An API key or GitHub token supplied to the workload is readable by that workload. Use the
least scope needed, separate it from unrelated accounts, keep it out of YAML and logs, and
never promise to hide it from the coding agent. Credential forwarding from the host, if
added, must name each credential explicitly and use a supported transfer mechanism.

The guard must install deny rules before resolving permitted names or starting a command.
Permit DNS only through its restricted resolver; explicitly test attempts against Docker's
`127.0.0.11`, external resolvers, DNS over TCP and UDP, and other localhost addresses before
any broad loopback allowance. Reject metadata, host-gateway and private-network destinations
unless a later, reviewed feature expressly needs them. IPv6 must be blocked or equivalently
filtered; inability to do either is a startup failure, not a warning.

Match names exactly in the pilot. Resolve permitted names through the guard and update the
accepted addresses before replying; bound stale entries and re-resolution behavior. CNAMEs,
changing DNS answers and addresses that become private require explicit checks. The network
permission also limits destination ports, unlike a general IP allowlist.

An IP allowlist still cannot prove which HTTPS service a shared address serves, and an
allowed model provider or GitHub endpoint can itself receive data. State that residual risk
in `doctor` and the README. If the experiment needs hostname-level enforcement, add an
appropriately constrained egress proxy and tests for CONNECT, SNI/authority mismatch
and direct-IP bypass before claiming that stronger boundary. Do not broadly allow a CDN or
cloud-storage suffix merely to make a download work.

Exercise real redirects. Pinned itos downloads can leave `github.com`; failed Actions logs
can redirect to Azure Blob storage. Either review narrowly scoped destination permissions
or implement a host-side retrieval operation that validates the requested run/artifact and
returns only those bytes. A generic host HTTP-fetch or shell bridge is out of scope. The first
project must be able to inspect a red run without opening all of `blob.core.windows.net`.

## Failure and recovery

Configuration errors create no resources. A guard or resolver failure, missing IPv6 control,
identity mismatch or mount refusal prevents the worker from starting. Partially prepared
resources are cleaned by recorded session ownership, not a broad Docker prune.

At runtime, loss of the guard/resolver leaves egress denied and stops the workload rather
than relaunching it with the engine's default network. Reapply and verify network policy
on every restart before permitting workload execution. Forward terminal signals to the
workload and do not report success while a command or required cleanup is still running.

Persist host control records outside the workspace, with restrictive permissions. After
an interrupted host process, `status` reconciles those records with labelled engine resources;
`stop <session>` can finish cleanup without discarding repository changes or provider state.
Log policy changes, blocked destinations and resource errors without URLs containing tokens
or request bodies. Startup and teardown have bounded waits and name what timed out.

## Acceptance and experiment plan

Acceptance tests exercise a disposable workload under the effective restriction, not strings
in a Dockerfile. A passing build alone is insufficient.

| Probe                                                                              | Required result                                                  |
| ---------------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| A command writes a file and commits through this project's hooks                   | The change persists; gates run inside the container.             |
| The workload reads an unmounted host sentinel, Docker socket or host home          | Access is refused.                                               |
| Workload writes protected git config/hooks, firewall, capabilities or guard policy | Access is refused; the active policy is unchanged.               |
| Query an unrelated DNS name through normal DNS, Docker DNS or external DNS         | No unauthorized query reaches an external resolver.              |
| Connect to a non-allowed IP, IPv6 destination, host gateway or metadata address    | The connection is refused.                                       |
| Resolve an allowed name with changed answers, CNAMEs and a private answer          | Public allowed answers work; private/bypassing answers fail.     |
| Stop or crash the guard and restart a session                                      | No unfiltered execution window appears.                          |
| Start with missing controls or a malformed/loosened policy                         | No workload starts without the required approval.                |
| Run at UID 1000 and a different UID/GID                                            | Workspace, history and profile state remain usable without sudo. |
| Run OpenCode with Codex OAuth, including token refresh                             | It uses the selected account without a host database/home mount. |
| Fetch a pinned itos release and read a failed Actions log                          | The complete redirect chain works under reviewed permissions.    |
| Run two projects with equal folder basenames and stop one                          | State and cleanup stay isolated; the other project is untouched. |

First build a vertical slice in the separate extension project: strict config parsing,
`doctor`, guard startup, one worker command, recorded ownership and safe cleanup. Use a
disposable repository and no valuable credentials. Then run the denied-network, embedded-DNS,
IPv6 and capability probes before adding OpenCode authentication. Finish the first usable
slice with one real hook-governed commit and a readable CI failure in the consumer project.

Keep the existing `.devcontainer/` until the extension has passed those tests; the experiment
does not replace or remove it. When the extension is stable, offer devcontainer/editor
configuration as an adapter to the same policy rather than a second independent firewall.
Automate the image build and runtime probes in the extension project's CI; runtime testing
on each supported architecture remains necessary. No implementation schedule is set here.

## Open decisions

The user's proposed extension direction is the working assumption. The remaining choices
are recommendations for the experiment, not accepted ADRs:

- Use `itos-container` as the executable/command name, or choose `itos-sandbox` before the
  interface is released. Reserve the name through the extension project's normal process.
- Validate the separate guard architecture first. Decide whether its IP boundary is enough
  or a proxy with stronger hostname enforcement is required for real credentials.
- Decide how host-side policy approval is stored and how a person approves a changed policy.
  A workspace file an agent can edit must not count as its own approval.
- Choose the first workload image and whether it comes from the extension release or the
  consumer. Pin every security-relevant image by digest and verify its provenance.
- Define the provider login/refresh and narrowly scoped log-download paths through actual
  tests. Do not infer OpenCode support from the Claude Code image's token variables.
- Decide how to revoke/delete persistent profiles and whether worktree/remote-engine support
  is worth their additional host-mount and credential boundaries.

## References

- [Core goals and non-goals](../../PLAN.md), especially the boundary between policy and runners.
- [Existing extension contract](../extensions.md) and [extension scenarios](../../features/extensions.feature).
- [T-108 container documentation](../../.devcontainer/README.md),
  [mounts and capabilities](../../.devcontainer/devcontainer.json), and
  [firewall implementation](../../.devcontainer/itos-firewall.sh).
- [Task ledger](../../tasks/phase-1.yaml), T-108's recorded manual checks.
- [Work registry](../../tasks/work-items.yaml): `p1-devcontainer-actions-logs`,
  `p1-devcontainer-nightly-build` and `p1-devcontainer-untested-gaps`.

These references point into the itos source checkout. A copy in the new extension project
should preserve a source revision or permalinks rather than treating relative links as local files.
