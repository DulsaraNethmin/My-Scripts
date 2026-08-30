# Releasing spinup

Pushing a `v*` tag is the whole release. `.github/workflows/release.yml` builds
the six binaries, publishes the GitHub release, signs the checksums and pushes
the Homebrew cask and the Scoop manifest. What follows is what has to exist
before that works, and how to check it without publishing anything.

## Before the first release

These are yours to do once; nothing in CI can do them.

1. **Rename the repository** to `spinup` (ledger task 0.3). The cask and the
   Scoop manifest point their `homepage` at `github.com/DulsaraNethmin/spinup`,
   and the release URLs come from the git remote. GitHub redirects the old name,
   so nothing breaks afterwards — but the homepage 404s until the rename.

2. **Create the two package repositories**, public and empty (a README is
   enough):

   - `DulsaraNethmin/homebrew-tap` — `brew install DulsaraNethmin/tap/spinup`
     resolves `DulsaraNethmin/tap` to a repository named `homebrew-tap`, so the
     name is not a choice.
   - `DulsaraNethmin/scoop-bucket`.

   GoReleaser commits into them; it does not create them.

3. **Create a token and add it as a secret.** The workflow's own `GITHUB_TOKEN`
   can only write to this repository, so pushing a manifest to the tap needs a
   personal access token:

   - Fine-grained token, *Only select repositories* → `homebrew-tap` and
     `scoop-bucket`, permission **Contents: Read and write**. (A classic token
     with `repo` works too, but grants far more.)
   - Add it here as **Settings → Secrets and variables → Actions → New
     repository secret**, named `TAP_GITHUB_TOKEN`.

   The workflow checks the secret is set before it publishes anything, so a
   missing token fails the run instead of leaving a release with no cask.

## Dry run

Actions → **release** → **Run workflow**. On anything other than a tag the
workflow builds a snapshot: all six targets, the archives, the checksums and
their cosign signature, the cask and the Scoop manifest — then checks them and
uploads the archives as a workflow artefact. Nothing reaches the Releases page
and nothing is pushed to the tap.

Locally, `make snapshot` does the same build into `dist/` (without signing —
that needs an OIDC token only Actions has), and `make release-check` validates
`.goreleaser.yaml`. CI runs `goreleaser check` on every push.

## Releasing

```sh
# 1. The changelog's [Unreleased] section becomes the release's section.
$EDITOR CHANGELOG.md
git commit -am "docs: changelog for v1.1.0"

# 2. Tag it. Annotated, so `git describe` and the release agree.
git tag -a v1.1.0 -m "spinup v1.1.0"

# 3. Push. This is the release.
git push origin main --follow-tags
```

Then watch Actions. When it is green:

```sh
brew install DulsaraNethmin/tap/spinup
spinup version        # prints the tag, v-prefix and all
```

**Why v1.1.0 and not v0.1.0** — this repository was tagged `v1.0.0` in its
My-Scripts days. A v0 tag would sort below a tag that is already published, and
the old one is staying put, so spinup's releases start above it. `docs/PLAN.md`
§7.6 has the reasoning.

## What a release contains

Six archives — `{darwin,linux,windows}` × `{amd64,arm64}` — each holding one
`spinup` binary plus `LICENSE`, `README.md` and `CHANGELOG.md`. The stack
catalog is **not** in the archive: it is compiled into the binary, so a copy
beside it would go stale the moment the user upgrades. The release job unpacks
every archive and fails if `stacks/` shows up in one.

Also `checksums.txt`, with `checksums.txt.sig` and `checksums.txt.pem` from
cosign keyless signing — no private key, the signature is bound to the workflow's
GitHub OIDC identity. To verify a download:

```sh
sha256sum -c checksums.txt --ignore-missing
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature checksums.txt.sig \
  --certificate-identity-regexp 'https://github\.com/DulsaraNethmin/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

Homebrew is a **cask**, not a formula: GoReleaser deprecated formula generation
in favour of casks, and casks are macOS-only. Linux users get the archives and
`install.sh`; `brew` on Linux cannot install a cask.

## How people get the update

Each of these reads the release the workflow just published, so nothing extra
has to be done for them:

| Installed with | Updates with |
| --- | --- |
| Homebrew (macOS) | `brew upgrade spinup` |
| Scoop (Windows) | `scoop update spinup` |
| `install.sh` / `install.ps1` | `spinup update`, or re-run the installer |
| The archive, by hand | `spinup update` |

`spinup update` downloads the archive for the running platform, checks it
against `checksums.txt`, and replaces the binary in place. It refuses to touch
a binary Homebrew or Scoop owns — that file belongs to the package manager, and
overwriting it is undone by the next upgrade — and prints the right command
instead. `spinup update --check` only reports.

Both installers read the same release through the API, so a release missing its
`checksums.txt` breaks them: nothing installs unverified.

## When a release goes wrong

A tag can be replaced as long as nobody has installed it yet:

```sh
gh release delete v1.1.0 --yes
git push origin :refs/tags/v1.1.0
git tag -d v1.1.0
# fix, commit, tag again
```

Once a version is out in the wild, ship the next number instead — Homebrew and
Scoop both cache by version, and re-pointing a tag at different bytes breaks
the checksums already in the tap.
