#!/bin/sh
# Install spinup on macOS or Linux.
#
#   curl -fsSL https://raw.githubusercontent.com/DulsaraNethmin/spinup/main/install.sh | sh
#
# Downloads the release archive for this platform, checks it against the
# release's checksums.txt, and installs one binary. The stack catalog is
# compiled into that binary, so there is nothing else to place.
#
# Options (flags, or the environment variables in brackets):
#   --version <v>   install a specific release, e.g. v1.1.0   [SPINUP_VERSION]
#   --dir <path>    install into this directory               [SPINUP_INSTALL_DIR]
#   --help          this text
#
# SPINUP_REPO and SPINUP_API point the script at another repository or API,
# which is how its own test drives it against a local server.

set -eu

REPO="${SPINUP_REPO:-DulsaraNethmin/spinup}"
API="${SPINUP_API:-https://api.github.com}"
VERSION="${SPINUP_VERSION:-latest}"
INSTALL_DIR="${SPINUP_INSTALL_DIR:-}"

BOLD=''
DIM=''
RESET=''
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
	BOLD=$(printf '\033[1m')
	DIM=$(printf '\033[2m')
	RESET=$(printf '\033[0m')
fi

die() {
	printf '%s\n' "install.sh: $*" >&2
	exit 1
}

say() { printf '%s\n' "$*"; }

# usage prints this script's own header comment, so the two cannot drift.
usage() {
	awk 'NR>1 && /^#/ { sub(/^# ?/, ""); print; next } NR>1 { exit }' "$0"
}

have() { command -v "$1" >/dev/null 2>&1; }

# fetch prints a URL's body on stdout. curl and wget are both common enough
# that requiring a particular one would fail on someone's machine.
fetch() {
	if have curl; then
		curl -fsSL "$1"
	elif have wget; then
		wget -qO- "$1"
	else
		die "neither curl nor wget is installed"
	fi
}

download() {
	if have curl; then
		curl -fsSL -o "$2" "$1"
	elif have wget; then
		wget -qO "$2" "$1"
	else
		die "neither curl nor wget is installed"
	fi
}

sha256() {
	if have sha256sum; then
		sha256sum "$1" | cut -d' ' -f1
	elif have shasum; then
		shasum -a 256 "$1" | cut -d' ' -f1
	else
		die "no sha256sum or shasum, so the download cannot be verified"
	fi
}

# json_field pulls one string value out of a JSON object. The releases API
# answers with a single object and this script needs two fields out of it;
# depending on jq being installed would be a worse trade than this.
#
# api.github.com pretty-prints: `"tag_name": "v1.5.1"`, with a space after the
# colon. The pattern has to allow that, or every install dies with "no release
# for 'latest'" — install_test.go serves the same shape so that cannot pass CI.
json_field() {
	tr ',' '\n' | sed -n 's/.*"'"$1"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1
}

platform() {
	os=$(uname -s | tr '[:upper:]' '[:lower:]')
	case "$os" in
	linux | darwin) ;;
	*) die "unsupported operating system: $os (spinup ships macOS, Linux and Windows builds)" ;;
	esac

	arch=$(uname -m)
	case "$arch" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) die "unsupported architecture: $arch (spinup ships amd64 and arm64)" ;;
	esac

	printf '%s_%s' "$os" "$arch"
}

# install_dir picks where the binary goes: what the user asked for, else the
# first of the usual directories that can be written to.
install_dir() {
	if [ -n "$INSTALL_DIR" ]; then
		printf '%s' "$INSTALL_DIR"
		return
	fi
	for dir in /usr/local/bin "$HOME/.local/bin"; do
		if [ -d "$dir" ] && [ -w "$dir" ]; then
			printf '%s' "$dir"
			return
		fi
	done
	# Nothing writable: ~/.local/bin is the one this script may create.
	printf '%s' "$HOME/.local/bin"
}

while [ $# -gt 0 ]; do
	case "$1" in
	--version)
		[ $# -ge 2 ] || die "--version needs a value"
		VERSION="$2"
		shift 2
		;;
	--dir)
		[ $# -ge 2 ] || die "--dir needs a value"
		INSTALL_DIR="$2"
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*) die "unknown option: $1 (try --help)" ;;
	esac
done

target=$(platform)

if [ "$VERSION" = latest ]; then
	release_url="$API/repos/$REPO/releases/latest"
else
	release_url="$API/repos/$REPO/releases/tags/$VERSION"
fi

release=$(fetch "$release_url") || die "cannot read $release_url"
tag=$(printf '%s' "$release" | json_field tag_name)
[ -n "$tag" ] || die "$REPO has no release for '$VERSION'"

# The archive names have no leading v; the tags do.
number=${tag#v}
archive="spinup_${number}_${target}.tar.gz"

# Splitting on commas puts each asset's download URL on its own line, which is
# enough structure to pick one out without a JSON parser. The closing quote is
# part of the match: `/checksums.txt` alone is a prefix of `/checksums.txt.sig`.
asset_url() {
	printf '%s' "$release" | tr ',' '\n' | grep browser_download_url | grep -F "/$1\"" |
		sed -n 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1
}

archive_url=$(asset_url "$archive")
[ -n "$archive_url" ] || die "$tag has no $archive — spinup may not ship a build for $target yet"

checksums_url=$(asset_url checksums.txt)
[ -n "$checksums_url" ] || die "$tag has no checksums.txt, so the download cannot be verified"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

say "${DIM}downloading${RESET} $archive ($tag)"
# Under set -e a failed curl would end the script with nothing but curl's own
# line, which reads like a bug in the installer when it is usually the network.
download "$archive_url" "$tmp/$archive" || die "cannot download $archive_url"
download "$checksums_url" "$tmp/checksums.txt" || die "cannot download $checksums_url"

want=$(grep " $archive\$" "$tmp/checksums.txt" | cut -d' ' -f1 | head -n 1)
[ -n "$want" ] || die "checksums.txt has no entry for $archive"
got=$(sha256 "$tmp/$archive")
[ "$got" = "$want" ] || die "$archive does not match its checksum
  got  $got
  want $want"

tar -xzf "$tmp/$archive" -C "$tmp" spin spinup || die "the archive has no spin/spinup binaries in it"
chmod +x "$tmp/spin" "$tmp/spinup"

dir=$(install_dir)
mkdir -p "$dir" 2>/dev/null || true

if [ -w "$dir" ]; then
	mv "$tmp/spin" "$dir/spin"
	mv "$tmp/spinup" "$dir/spinup"
elif have sudo; then
	say "${DIM}$dir needs root — using sudo${RESET}"
	sudo mv "$tmp/spin" "$dir/spin"
	sudo mv "$tmp/spinup" "$dir/spinup"
else
	die "cannot write to $dir. Re-run with --dir \$HOME/.local/bin, or as root."
fi

say ""
say "${BOLD}spin $tag${RESET} installed at $dir/spin, with $dir/spinup beside it"

case ":$PATH:" in
*":$dir:"*) ;;
*)
	say ""
	say "$dir is not on your PATH. Add it:"
	say "  ${DIM}export PATH=\"$dir:\$PATH\"${RESET}"
	;;
esac

say ""
say "Next:"
say "  ${DIM}spin doctor${RESET}          check Docker is ready"
say "  ${DIM}spin list${RESET}            the stack catalog"
say "  ${DIM}spin up postgres${RESET}     Postgres 16 + pgAdmin"
say ""
say "Shell completion: ${DIM}spin completion --help${RESET}"
