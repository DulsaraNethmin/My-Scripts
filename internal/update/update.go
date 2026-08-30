// Package update finds and installs newer spinup releases.
//
// It reads the GitHub releases API and downloads the same archives a user would
// download by hand, verified against the same checksums.txt the release
// publishes. Nothing here prints, prompts or exits — cmd/update.go owns that.
package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DefaultRepo is where spinup's releases live. SPINUP_REPO overrides it, which
// is what makes the CLI usable against a fork — or against this repository
// before it is renamed away from My-Scripts.
const DefaultRepo = "DulsaraNethmin/spinup"

// defaultAPI is GitHub's API. SPINUP_API overrides it; the tests point it at a
// local server.
const defaultAPI = "https://api.github.com"

// maxDownload caps what a single download may be. A spinup archive is ~2.5 MB;
// this is generous enough to never bite and small enough that a wrong URL
// cannot fill a disk.
const maxDownload = 64 << 20

// ErrNoAsset means the release exists but has nothing for this platform.
var ErrNoAsset = errors.New("the release has no archive for this platform")

// Release is a published release: its tag, and the assets attached to it by
// name.
type Release struct {
	Tag    string
	Assets map[string]string // asset name -> download URL
}

// Client reads releases from the GitHub API.
type Client struct {
	HTTP *http.Client
	API  string // API base URL; empty means GitHub
	Repo string // owner/name; empty means DefaultRepo
}

// NewClient returns a client configured from the environment: SPINUP_REPO and
// SPINUP_API override where releases are looked for.
func NewClient() *Client {
	return &Client{
		HTTP: &http.Client{Timeout: 60 * time.Second},
		API:  os.Getenv("SPINUP_API"),
		Repo: os.Getenv("SPINUP_REPO"),
	}
}

func (c *Client) api() string {
	if c.API == "" {
		return defaultAPI
	}
	return strings.TrimSuffix(c.API, "/")
}

func (c *Client) repo() string {
	if c.Repo == "" {
		return DefaultRepo
	}
	return c.Repo
}

func (c *Client) http() *http.Client {
	if c.HTTP == nil {
		return &http.Client{Timeout: 60 * time.Second}
	}
	return c.HTTP
}

// Latest returns the repository's latest release. GitHub excludes drafts and
// prereleases from this endpoint, so a release candidate never offers itself to
// everyone running the stable version.
func (c *Client) Latest(ctx context.Context) (Release, error) {
	body, err := c.get(ctx, fmt.Sprintf("%s/repos/%s/releases/latest", c.api(), c.repo()))
	if err != nil {
		return Release{}, err
	}

	var payload struct {
		Tag    string `json:"tag_name"`
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return Release{}, fmt.Errorf("the release API returned something unreadable: %w", err)
	}
	if payload.Tag == "" {
		return Release{}, fmt.Errorf("%s has no published releases", c.repo())
	}

	rel := Release{Tag: payload.Tag, Assets: make(map[string]string, len(payload.Assets))}
	for _, a := range payload.Assets {
		rel.Assets[a.Name] = a.URL
	}
	return rel, nil
}

// Asset downloads one of a release's assets by name.
func (c *Client) Asset(ctx context.Context, rel Release, name string) ([]byte, error) {
	url, ok := rel.Assets[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s is not in %s", ErrNoAsset, name, rel.Tag)
	}
	return c.get(ctx, url)
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// GitHub rejects requests without a user agent, and the accept header keeps
	// the JSON shape stable across API versions.
	req.Header.Set("User-Agent", "spinup")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck // reading is done; a close error says nothing

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned %s", url, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDownload+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", url, err)
	}
	if len(body) > maxDownload {
		return nil, fmt.Errorf("%s is larger than %d bytes", url, maxDownload)
	}
	return body, nil
}

// ArchiveName is the archive a release publishes for one platform. It has to
// match .goreleaser.yaml's name_template exactly, so the two are checked
// against each other in the tests.
func ArchiveName(version, goos, goarch string) string {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("spinup_%s_%s_%s.%s", Number(version), goos, goarch, ext)
}

// Number strips the leading v from a version, which the tags and the binary
// carry but the archive names do not.
func Number(version string) string { return strings.TrimPrefix(version, "v") }

// Same reports whether two versions are the same release, ignoring whether
// either spells it with a leading v.
func Same(a, b string) bool { return Number(a) == Number(b) }

// Verify checks data against the sha256 recorded for name in a checksums.txt.
// A missing entry is a failure: an unverified binary is not installed.
func Verify(checksums, data []byte, name string) error {
	want, err := checksumFor(checksums, name)
	if err != nil {
		return err
	}

	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("%s does not match its checksum: got %s, want %s", name, got, want)
	}
	return nil
}

// checksumFor finds name's line in a checksums.txt: "<sha256>  <name>".
func checksumFor(checksums []byte, name string) (string, error) {
	for line := range strings.SplitSeq(string(checksums), "\n") {
		sum, file, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok {
			continue
		}
		if strings.TrimSpace(file) == name {
			return sum, nil
		}
	}
	return "", fmt.Errorf("checksums.txt has no entry for %s", name)
}

// Binary extracts the spinup executable from a release archive: a tar.gz
// everywhere, a zip on Windows.
func Binary(archive []byte, goos string) ([]byte, error) {
	if goos == "windows" {
		return fromZip(archive, "spinup.exe")
	}
	return fromTarGz(archive, "spinup")
}

func fromTarGz(archive []byte, want string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("the archive is not a gzip: %w", err)
	}
	defer gz.Close() //nolint:errcheck // reading only

	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("the archive is unreadable: %w", err)
		}
		if filepath.Base(h.Name) != want || h.Typeflag != tar.TypeReg {
			continue
		}
		return io.ReadAll(io.LimitReader(tr, maxDownload))
	}
	return nil, fmt.Errorf("the archive has no %s in it", want)
}

func fromZip(archive []byte, want string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("the archive is not a zip: %w", err)
	}

	for _, f := range zr.File {
		if filepath.Base(f.Name) != want {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("the archive is unreadable: %w", err)
		}
		defer rc.Close() //nolint:errcheck // reading only

		return io.ReadAll(io.LimitReader(rc, maxDownload))
	}
	return nil, fmt.Errorf("the archive has no %s in it", want)
}

// Replace swaps the executable at path for data, keeping its permissions.
//
// The new binary is written beside the old one and renamed over it, so a failed
// download or a full disk cannot leave a half-written spinup behind: either the
// rename happens or the old binary is untouched.
func Replace(path string, data []byte) error {
	dir := filepath.Dir(path)

	mode := os.FileMode(0o755)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, ".spinup-update-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s: %w", dir, err)
	}
	name := tmp.Name()
	defer os.Remove(name) //nolint:errcheck // only matters if the rename failed

	if _, err := tmp.Write(data); err != nil {
		tmp.Close() //nolint:errcheck // the write error is the one to report
		return fmt.Errorf("cannot write %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cannot write %s: %w", name, err)
	}
	if err := os.Chmod(name, mode); err != nil {
		return fmt.Errorf("cannot make %s executable: %w", name, err)
	}

	// Windows will not let a running executable be replaced, but it will let it
	// be renamed. The leftover is removed on the next update.
	if runtime.GOOS == "windows" {
		old := path + ".old"
		os.Remove(old)                               //nolint:errcheck // a leftover from last time, if any
		if err := os.Rename(path, old); err != nil { //nolint:staticcheck // the rename below is what matters
			return fmt.Errorf("cannot move the old %s aside: %w", path, err)
		}
	}

	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("cannot install into %s: %w", path, err)
	}
	return nil
}

// Manager is a package manager that owns an installed spinup, and the command
// that updates it.
type Manager struct {
	Name    string
	Command string
}

// ManagedBy reports the package manager that installed the binary at path, so
// `spinup update` can send a Homebrew user to Homebrew rather than overwriting
// a file brew believes it owns — the next `brew upgrade` would undo it, and
// `brew doctor` would complain in the meantime.
func ManagedBy(path string) (Manager, bool) {
	// Backslashes are normalised by hand rather than with filepath.ToSlash: a
	// Windows path can be examined from a test running on Linux, where ToSlash
	// leaves it alone.
	p := strings.ReplaceAll(path, `\`, "/")

	switch {
	case strings.Contains(p, "/Cellar/"), strings.Contains(p, "/Caskroom/"),
		strings.Contains(p, "/homebrew/"), strings.Contains(p, "/linuxbrew/"):
		return Manager{Name: "Homebrew", Command: "brew upgrade spinup"}, true
	case strings.Contains(strings.ToLower(p), "/scoop/"):
		return Manager{Name: "Scoop", Command: "scoop update spinup"}, true
	}
	return Manager{}, false
}
