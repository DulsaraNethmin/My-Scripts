#!/usr/bin/env bash
# Install Docker Desktop on macOS via Homebrew.
#
#   ./setup/docker-macos.sh
#
# Docker Desktop bundles Compose v2, so there is nothing else to install.
# Idempotent: safe to re-run.
set -euo pipefail

log()  { printf '\033[1m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[33mwarning:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[31merror:\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(uname -s)" = "Darwin" ] || die "this script is for macOS"

if docker compose version >/dev/null 2>&1; then
  log "Docker with Compose v2 is already installed:"
  docker --version
  docker compose version
  exit 0
fi

if ! command -v brew >/dev/null 2>&1; then
  die "Homebrew is required. Install it from https://brew.sh, or download
       Docker Desktop directly from https://docs.docker.com/desktop/install/mac-install/"
fi

if [ -d "/Applications/Docker.app" ]; then
  log "Docker Desktop is installed but not running"
else
  log "Installing Docker Desktop"
  brew install --cask docker
fi

log "Starting Docker Desktop"
open -a Docker

log "Waiting for the Docker daemon (up to 120s)"
for _ in $(seq 1 60); do
  if docker info >/dev/null 2>&1; then
    log "Installed:"
    docker --version
    docker compose version
    exit 0
  fi
  sleep 2
done

warn "Docker Desktop did not become ready in time."
warn "Open it from Applications, accept the terms, then re-run this script."
exit 1
