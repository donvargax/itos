# Claude Code Devcontainer
# Based on Microsoft devcontainer image for better devcontainer integration
#
# trailofbits/claude-code-devcontainer at feea113 (Apache-2.0, LICENSE beside this file), changed
# for itos (T-108): dnsmasq-base; Go, pnpm and vite-plus for this repository's hooks; itos and
# its git shim first on the PATH; the egress allowlist baked in and run at every start; no
# passwordless sudo; no /workspace (the workspace is mounted at its host path); the base image
# moved to a digest whose git is 2.55, since itos runs its hooks from the git config (2.54+).
FROM ghcr.io/astral-sh/uv:0.10@sha256:10902f58a1606787602f303954cea099626a4adb02acbac4c69920fe9d278f82 AS uv
FROM mcr.microsoft.com/devcontainers/base:ubuntu24.04@sha256:d7c468679f45a52ad3673d06656b5bf16e488990b17216a1fa295d9e1f89d724

ARG TZ
ENV TZ="$TZ"

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# Install additional system packages (base image already includes git, curl, sudo, etc.)
RUN apt-get update && apt-get install -y --no-install-recommends \
  # Sandboxing support for Claude Code
  bubblewrap \
  socat \
  # Modern CLI tools
  fd-find \
  ripgrep \
  tmux \
  zsh \
  # Build tools
  build-essential \
  # Utilities
  jq \
  nano \
  unzip \
  vim \
  # Network tools (for security testing)
  dnsutils \
  ipset \
  iptables \
  iproute2 \
  # The allowlist's resolver (itos-firewall)
  dnsmasq-base \
  && apt-get clean && rm -rf /var/lib/apt/lists/*

# Install git-delta
# renovate: datasource=github-releases depName=dandavison/delta
ARG GIT_DELTA_VERSION=0.18.2
RUN ARCH=$(dpkg --print-architecture) && \
  curl -fsSL "https://github.com/dandavison/delta/releases/download/${GIT_DELTA_VERSION}/git-delta_${GIT_DELTA_VERSION}_${ARCH}.deb" -o /tmp/git-delta.deb && \
  dpkg -i /tmp/git-delta.deb && \
  rm /tmp/git-delta.deb

# Install uv (Python package manager) via multi-stage copy
COPY --from=uv /uv /usr/local/bin/uv

# Install fzf from GitHub releases (newer than apt, includes built-in shell integration)
# renovate: datasource=github-releases depName=junegunn/fzf
ARG FZF_VERSION=0.70.0
RUN ARCH=$(dpkg --print-architecture) && \
  case "${ARCH}" in \
    amd64) FZF_ARCH="linux_amd64" ;; \
    arm64) FZF_ARCH="linux_arm64" ;; \
    *) echo "Unsupported architecture: ${ARCH}" && exit 1 ;; \
  esac && \
  curl -fsSL "https://github.com/junegunn/fzf/releases/download/v${FZF_VERSION}/fzf-${FZF_VERSION}-${FZF_ARCH}.tar.gz" | tar -xz -C /usr/local/bin

# Go: the toolchain go.mod's toolchain line names, so the hooks (tools/bin/itos, go vet, the
# unit tests) build and run inside. Each archive checked against go.dev's published hash.
ARG GO_VERSION=1.27.1
ARG GO_SHA256_AMD64=63d339f0da5ab53635a56f2490a7984dfe12dfcff22ad749f63edaf590168445
ARG GO_SHA256_ARM64=3450b45a3f9ee8568792736a5c5e70a1f2e9b36c35a8f74958c03e51d7d92bec
RUN ARCH=$(dpkg --print-architecture) && \
  case "${ARCH}" in \
    amd64) GO_SHA256="${GO_SHA256_AMD64}" ;; \
    arm64) GO_SHA256="${GO_SHA256_ARM64}" ;; \
    *) echo "Unsupported architecture: ${ARCH}" && exit 1 ;; \
  esac && \
  curl -fsSL "https://dl.google.com/go/go${GO_VERSION}.linux-${ARCH}.tar.gz" -o /tmp/go.tar.gz && \
  echo "${GO_SHA256}  /tmp/go.tar.gz" | sha256sum -c - && \
  tar -C /usr/local -xzf /tmp/go.tar.gz && \
  rm /tmp/go.tar.gz

# itos: the released binary, its checksums.txt checked against a pinned hash and the
# archive against its line there; then the git shim, a link named git beside it.
# /usr/local/bin already holds the image's own git (built from source), so itos
# goes in a folder of its own that ENV PATH puts first (end of this file), and not under
# /opt, which the RUN below chowns to vscode.
ARG ITOS_VERSION=6.3.0
ARG ITOS_CHECKSUMS_SHA256=41efeb642689de0cfd3ec7dbc57d2223423aa174aac0fb5db85eb17bc53faa04
RUN ARCH=$(dpkg --print-architecture) && \
  tmp=$(mktemp -d) && cd "$tmp" && \
  base="https://github.com/donvargax/itos/releases/download/v${ITOS_VERSION}" && \
  curl -fsSLO "${base}/checksums.txt" && \
  echo "${ITOS_CHECKSUMS_SHA256}  checksums.txt" | sha256sum -c - && \
  curl -fsSLO "${base}/itos-${ITOS_VERSION}-linux-${ARCH}.tar.gz" && \
  sha256sum --ignore-missing -c checksums.txt && \
  mkdir -p /usr/local/lib/itos/bin && \
  tar --no-same-owner -xzf "itos-${ITOS_VERSION}-linux-${ARCH}.tar.gz" -C /usr/local/lib/itos/bin itos && \
  chmod 0755 /usr/local/lib/itos/bin/itos && \
  cd / && rm -rf "$tmp" && \
  PATH="/usr/local/lib/itos/bin:$PATH" /usr/local/lib/itos/bin/itos git-shim install

# The egress allowlist, root's alone: the script is owned by root and outside the workspace, so
# the container cannot rewrite what runs as root. The base image gives vscode passwordless sudo
# for everything, which with NET_ADMIN could undo the allowlist and the shim; that rule goes,
# and the one left runs this script, with no arguments, and nothing else. dnsmasq runs as
# itos-dns, the one user allowed to reach the upstream resolver.
COPY --chown=root:root --chmod=0755 itos-firewall.sh /usr/local/sbin/itos-firewall
RUN useradd --system --user-group --no-create-home --shell /usr/sbin/nologin itos-dns && \
  rm -f /etc/sudoers.d/vscode && \
  (gpasswd -d vscode sudo 2>/dev/null || true) && \
  echo 'vscode ALL=(root) NOPASSWD: /usr/local/sbin/itos-firewall ""' > /etc/sudoers.d/itos-firewall && \
  chmod 0440 /etc/sudoers.d/itos-firewall && \
  visudo -c && \
  ! grep -rEn 'NOPASSWD: *ALL' /etc/sudoers /etc/sudoers.d

# Create directories and set ownership (combined for fewer layers)
RUN mkdir -p /commandhistory /home/vscode/.claude /home/vscode/.config/gh /opt && \
  touch /commandhistory/.bash_history && \
  touch /commandhistory/.zsh_history && \
  chown -R vscode:vscode /commandhistory /home/vscode/.claude /home/vscode/.config /opt

# Set environment variables
ENV DEVCONTAINER=true
ENV SHELL=/bin/zsh
ENV EDITOR=nano
ENV VISUAL=nano

# Switch to non-root user for remaining setup
USER vscode
WORKDIR /home/vscode

# Set PATH early so claude and other user-installed binaries are available
ENV PATH="/home/vscode/.local/bin:$PATH"

# Install Claude Code natively with marketplace plugins
RUN curl -fsSL https://claude.ai/install.sh | bash && \
  claude plugin marketplace add anthropics/skills && \
  claude plugin marketplace add trailofbits/skills && \
  claude plugin marketplace add trailofbits/skills-curated

# Install Python 3.13 via uv (fast binary download, not source compilation)
RUN uv python install 3.13 --default

# Install ast-grep (AST-based code search)
RUN uv tool install ast-grep-cli

# Install fnm (Fast Node Manager) and Node
ARG NODE_VERSION=24
ENV FNM_DIR="/home/vscode/.fnm"
RUN curl -fsSL https://fnm.vercel.app/install | bash -s -- --install-dir "$FNM_DIR" --skip-shell && \
  export PATH="$FNM_DIR:$PATH" && \
  eval "$(fnm env)" && \
  fnm install ${NODE_VERSION} && \
  fnm default ${NODE_VERSION}

# fnm's shell hook is zsh-only, so without this node/npm are missing from bash/sh
ENV PATH="$FNM_DIR/aliases/default/bin:$PATH"

# pnpm, the version package.json's packageManager names, and vp, the Vite+ CLI the hooks and
# the review checklist call (it runs the project's own vite-plus from node_modules).
ARG PNPM_VERSION=12.4.1
ARG VITE_PLUS_VERSION=1.0.0
RUN npm install --global "pnpm@${PNPM_VERSION}" "vite-plus@${VITE_PLUS_VERSION}"

# Go's binaries, and those go install writes.
ENV PATH="/usr/local/go/bin:/home/vscode/go/bin:$PATH"

# Install Oh My Zsh
# renovate: datasource=github-releases depName=deluan/zsh-in-docker
ARG ZSH_IN_DOCKER_VERSION=1.2.1
RUN sh -c "$(curl -fsSL https://github.com/deluan/zsh-in-docker/releases/download/v${ZSH_IN_DOCKER_VERSION}/zsh-in-docker.sh)" -- \
  -p git \
  -x

# Copy zsh configuration
COPY --chown=vscode:vscode .zshrc /home/vscode/.zshrc.custom

# Append custom zshrc to the main one
RUN echo 'source ~/.zshrc.custom' >> /home/vscode/.zshrc

# Copy post_install script
COPY --chown=vscode:vscode post_install.py /opt/post_install.py

# The allowlist on every start, before the container's command: rules do not survive a restart.
# A failure leaves egress closed (itos-firewall closes first) and is in docker logs.
ENTRYPOINT ["/bin/sh", "-c", "sudo -n /usr/local/sbin/itos-firewall || echo 'itos-firewall failed: egress stays closed' >&2; exec \"$@\"", "itos-sandbox"]
CMD ["sleep", "infinity"]

# The itos git shim ahead of the image's gits (/usr/local/bin/git, /usr/bin/git). Last, so
# no later ENV PATH goes before it. Login bash (~/.profile) and interactive zsh (.zshrc)
# still prepend ~/.local/bin and fnm's folders, which hold no git but are the user's own.
ENV PATH="/usr/local/lib/itos/bin:$PATH"
