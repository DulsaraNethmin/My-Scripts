#!/usr/bin/env bash
# Install Docker Engine and the Compose v2 plugin on Fedora, RHEL, CentOS
# Stream, Rocky or Alma, from Docker's official dnf repository.
#
#   curl -fsSL .../setup/docker-fedora.sh | bash
#
# Idempotent: safe to re-run.
set -euo pipefail

log()  { printf '\033[1m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[33mwarning:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[31merror:\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -ne 0 ] || die "run this as a normal user; it calls sudo where needed"
command -v sudo >/dev/null 2>&1 || die "sudo is required"
command -v dnf  >/dev/null 2>&1 || die "this script needs dnf; see docker-ubuntu.sh for Debian/Ubuntu"

# shellcheck disable=SC1091  # provided by the OS, not in this repo
. /etc/os-release
case "${ID:-}" in
  fedora)                         repo="fedora" ;;
  rhel|centos|rocky|almalinux)    repo="centos" ;;
  *) warn "untested distro '${ID:-unknown}'; continuing with the centos repo"; repo="centos" ;;
esac

if docker compose version >/dev/null 2>&1; then
  log "Docker with Compose v2 is already installed:"
  docker --version
  docker compose version
else
  # Fedora ships a conflicting "docker" package from the podman stack.
  if rpm -q docker >/dev/null 2>&1; then
    warn "removing the distro 'docker' package, which conflicts with docker-ce"
    sudo dnf remove -y docker docker-client docker-common docker-engine \
      docker-latest docker-logrotate podman-docker || true
  fi

  log "Adding Docker's dnf repository ($repo)"
  sudo dnf install -y dnf-plugins-core
  sudo dnf config-manager --add-repo "https://download.docker.com/linux/$repo/docker-ce.repo" 2>/dev/null \
    || sudo dnf config-manager addrepo --from-repofile="https://download.docker.com/linux/$repo/docker-ce.repo"

  log "Installing Docker Engine and the Compose plugin"
  # docker-compose-plugin is Compose v2 (`docker compose`), not the
  # end-of-life standalone docker-compose v1 binary.
  sudo dnf install -y \
    docker-ce docker-ce-cli containerd.io \
    docker-buildx-plugin docker-compose-plugin
fi

log "Enabling the Docker service"
sudo systemctl enable --now docker

if ! id -nG "$USER" | tr ' ' '\n' | grep -qx docker; then
  log "Adding $USER to the docker group"
  sudo usermod -aG docker "$USER"
  warn "log out and back in (or run: newgrp docker) before using docker without sudo"
fi

log "Installed:"
docker --version
docker compose version
