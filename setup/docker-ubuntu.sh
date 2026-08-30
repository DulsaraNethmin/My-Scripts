#!/usr/bin/env bash
# Install Docker Engine and the Compose v2 plugin on Ubuntu or Debian,
# from Docker's official apt repository.
#
#   curl -fsSL .../setup/docker-ubuntu.sh | bash
#
# Idempotent: safe to re-run. Touches only Docker's repository and packages —
# it does not upgrade the rest of your system.
set -euo pipefail

log()  { printf '\033[1m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[33mwarning:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[31merror:\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -ne 0 ] || die "run this as a normal user; it calls sudo where needed"
command -v sudo >/dev/null 2>&1 || die "sudo is required"
command -v apt-get >/dev/null 2>&1 || die "this script is for Debian/Ubuntu; see docker-fedora.sh"

# shellcheck disable=SC1091  # provided by the OS, not in this repo
. /etc/os-release
distro="${ID:-ubuntu}"
case "$distro" in
  ubuntu|debian) ;;
  linuxmint|pop|elementary|zorin) distro="ubuntu" ;;
  *) warn "untested distro '$distro'; continuing as ubuntu"; distro="ubuntu" ;;
esac
codename="${UBUNTU_CODENAME:-${VERSION_CODENAME:-}}"
[ -n "$codename" ] || die "cannot determine release codename from /etc/os-release"

if docker compose version >/dev/null 2>&1; then
  log "Docker with Compose v2 is already installed:"
  docker --version
  docker compose version
else
  log "Installing prerequisites"
  sudo apt-get update -y
  sudo apt-get install -y ca-certificates curl gnupg

  log "Adding Docker's GPG key"
  sudo install -m 0755 -d /etc/apt/keyrings
  curl -fsSL "https://download.docker.com/linux/$distro/gpg" \
    | sudo gpg --batch --yes --dearmor -o /etc/apt/keyrings/docker.gpg
  sudo chmod a+r /etc/apt/keyrings/docker.gpg

  log "Adding Docker's apt repository ($distro $codename)"
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/$distro $codename stable" \
    | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null

  log "Installing Docker Engine and the Compose plugin"
  sudo apt-get update -y
  # docker-compose-plugin is Compose v2 (`docker compose`). The standalone
  # docker-compose v1 binary the old script installed reached end of life in
  # 2023 and is not installed here.
  sudo apt-get install -y \
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
