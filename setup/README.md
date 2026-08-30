# setup

One-shot Docker installers, for machines that do not have it yet. `spinup`
itself needs nothing but Docker and Compose v2.

| Script             | For                                                    |
|--------------------|--------------------------------------------------------|
| `docker-ubuntu.sh` | Ubuntu, Debian, Mint, Pop!_OS, elementary, Zorin       |
| `docker-fedora.sh` | Fedora, RHEL, CentOS Stream, Rocky, Alma               |
| `docker-macos.sh`  | macOS (Docker Desktop via Homebrew)                    |

```
./setup/docker-ubuntu.sh
```

All three are idempotent — re-running them on a machine that already has Docker
prints the versions and exits without changing anything.

Windows has no script: install
[Docker Desktop](https://docs.docker.com/desktop/install/windows-install/),
which ships Compose v2. `spinup` runs natively on Windows and does not need WSL.

## What changed from the old `docker-setup.sh`

The original was one Ubuntu-only script with three problems:

- **It installed Compose v1.** It fetched the standalone `docker-compose`
  binary, which reached end of life in 2023. These scripts install the
  `docker-compose-plugin` package instead — Compose v2, invoked as
  `docker compose`, which is what every stack in this repo expects.
- **It ran `apt-get upgrade -y`.** Installing Docker should not upgrade every
  package on the machine. These scripts touch only Docker's repository and
  packages.
- **It used the deprecated `apt-key` path** via `/usr/share/keyrings`. The
  current scripts use `/etc/apt/keyrings` with `signed-by`, matching Docker's
  current documented install.

They also detect the distribution rather than assuming Ubuntu, skip the install
entirely when Docker is already present, remove Fedora's conflicting
podman-based `docker` package, and only add you to the `docker` group if you
are not already in it.

## After installing on Linux

Being added to the `docker` group does not affect your current shell. Log out
and back in, or:

```
newgrp docker
docker run --rm hello-world
```

## Verifying

```
docker --version
docker compose version      # must be v2 or newer — "docker compose", not "docker-compose"
make doctor                 # from the repo root: checks the whole toolchain
```
