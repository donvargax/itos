# The itos devcontainer

A container to run Claude Code in, unattended, on this repository: Claude Code with
`bypassPermissions`, itos and its git shim first on the PATH, the Go, pnpm and vp the hooks
need, and outbound traffic limited to an allowlist. The container limits what reaches the
host; the shim and the hooks still hold every commit to the repository's rules.

## Before the first start, on the host

The container mounts the clone's `.git/config` and `.git/hooks` read-only, so it cannot install
hooks or change the git config. Do these once per clone, on the host:

1. **Use a clone, not a git worktree.** A worktree's `.git` is a file naming the main
   repository's folder on the host, which the container does not mount, so git fails inside.
2. **Install the hooks:** `tools/bin/itos init` (it reports the hooks; `tools/bin/itos hook
install` writes them) and `vp config`, which points `core.hooksPath` at `.vite-hooks/_`.
3. **Start it:** `devcontainer up --workspace-folder .`, then
   `devcontainer exec --workspace-folder . zsh`. Inside, run `vp install` once, so the
   pre-commit hook finds the project's vite-plus in `node_modules`. Scripts are off inside
   (`PNPM_CONFIG_IGNORE_SCRIPTS`), so the `prepare` script's `vp config`, which would write
   the read-only git config, does not run there.

**Commit inside the container, never on the host, once the container has written the
workspace.** The hooks run code from the working tree: `.vite-hooks/pre-commit`,
`tools/bin/itos` (built from `cmd/` and `internal/`), the checks `itos.yaml` and the ledger
name. A commit or push on the host would run, with the host's rights, whatever the container
wrote there. The same holds for any host tool that runs code the tree configures (Claude Code
reads hooks from `.claude/settings.json`): read what the container changed before running one.

## What it is built from

[trailofbits/claude-code-devcontainer](https://github.com/trailofbits/claude-code-devcontainer)
at `feea113`, Apache-2.0 (`LICENSE`). Changed here: the Dockerfile adds Go (go.mod's toolchain),
pnpm and vite-plus, dnsmasq, itos from a pinned release with its git shim, and the allowlist
below as a root-owned script run at every start; it takes away the container user's passwordless
sudo and the `/workspace` folder, and moves the base image to a digest whose git is 2.55 (itos
runs its hooks from the git config, which needs git 2.54). `devcontainer.json` mounts the
workspace at its host path, so paths written on one side (`node_modules/.bin`, an absolute
`core.hooksPath`) hold on the other, runs the image's entrypoint (`overrideCommand: false`),
sets no container name, and sets `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` and
`PNPM_CONFIG_IGNORE_SCRIPTS`. `post_install.py` loses its `sudo chown` step. The base's `devc`
helper (`install.sh`), its `renovate.json` and its README are not carried: the devcontainer CLI
or an editor starts the container directly.

## Pinned versions

| What      | Where                                   | Checked against                                    |
| --------- | --------------------------------------- | -------------------------------------------------- |
| itos      | `ITOS_VERSION` in the Dockerfile        | `ITOS_CHECKSUMS_SHA256`, the hash of its checksums |
| Go        | `GO_VERSION`, go.mod's `toolchain` line | `GO_SHA256_AMD64`/`_ARM64`, from go.dev/dl         |
| pnpm      | `PNPM_VERSION`, package.json's          | npm's registry                                     |
| vite-plus | `VITE_PLUS_VERSION`, the global `vp`    | npm's registry                                     |

To move itos: set `ITOS_VERSION`, download that release's `checksums.txt`, set
`ITOS_CHECKSUMS_SHA256` to its `sha256sum`, and rebuild.

## The allowlist

`itos-firewall.sh`, baked into the image as `/usr/local/sbin/itos-firewall`, owned by root.
The entrypoint runs it at every start, before the container's command, since iptables rules
do not survive a restart. It closes outbound traffic first and opens it after, so a failure
leaves the container closed (see `docker logs`).

- dnsmasq, as its own user, is the only resolver. It answers only the names below (others
  are refused, so DNS is no way out) and adds every address it answers with to the allowed
  set, so a CDN address that rotates is allowed before the client that asked connects.
- GitHub's published ranges (`api.github.com/meta`: web, api, git) are in the set too, since
  GitHub's names answer with one rotating address.
- IPv6 is closed but for loopback.

| Name (and the names under it)        | What reaches it                                                                            |
| ------------------------------------ | ------------------------------------------------------------------------------------------ |
| `github.com`, GitHub's meta ranges   | git fetch and push, `gh`, `itos push` watching CI, Claude Code plugins                     |
| `api.anthropic.com`                  | Claude Code                                                                                |
| `proxy.golang.org`, `sum.golang.org` | `go build`, `go vet` and the unit tests in the hooks, fetching modules                     |
| `vuln.go.dev`                        | the pre-commit hook's `deps-check`, whose govulncheck runs when go.mod or go.sum is staged |
| `registry.npmjs.org`                 | `vp install` (pnpm), which installs what the pre-commit hook runs                          |
| `pypi.org`, `files.pythonhosted.org` | the base's Python tooling (`uv`); nothing in this repository's build                       |

Not on it, so failing inside: GitHub's release downloads (`*.githubusercontent.com`, which a
repository that pins another itos fetches), `gh run view --log` (Azure blob storage), Claude
Code's interactive login and its updates. Log in with a token instead
(`CLAUDE_CODE_OAUTH_TOKEN` on the host, from `claude setup-token`). To change the list, edit
`ALLOWED_NAMES` in `itos-firewall.sh` and rebuild.

## sudo

The base image gives the container user passwordless sudo for everything; with the
`NET_ADMIN` capability that would let the container drop the allowlist and remove the shim.
Here the one rule left is `sudo /usr/local/sbin/itos-firewall`, with no arguments, which
re-applies the same allowlist. `sudo -n true` and `iptables -F OUTPUT` are refused.

## What it does not protect against

- **The real git, by full path.** `/usr/local/bin/git` and `/usr/bin/git` are still there, and
  the shim has its own gaps (`--no-verify`, `-c core.hooksPath`; `p1-shim-refuses-hook-skipping`).
  CI re-checks every pushed commit.
- **Tokens inside.** A Claude or GitHub token given to the container is readable inside it.
- **Allowed hosts.** Anything the allowed names and GitHub's ranges serve is reachable,
  including other sites behind the same CDN addresses.
- **Container escape**, and VS Code's "Reopen in Container", which lets container code drive
  host editor commands (the base's README has the detail).
- **A host user id other than 1000.** The volumes' folders are made for uid 1000; the base
  fixed others with `sudo chown`, which is gone.
