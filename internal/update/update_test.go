package update_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/update"
)

// TarGz builds a release-shaped tar.gz: the binary plus the paperwork the real
// archives carry, so the extractor is tested against the shape it will meet.
func tarGz(t *testing.T, binary []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	files := []struct {
		name string
		data []byte
	}{
		{"LICENSE", []byte("MIT")},
		{"README.md", []byte("# spinup")},
		{"spinup", binary},
	}
	for _, f := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: f.name, Mode: 0o755, Size: int64(len(f.data)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(f.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipped(t *testing.T, binary []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range map[string][]byte{"README.md": []byte("# spinup"), "spinup.exe": binary} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sum(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}

// The names have to match .goreleaser.yaml's name_template exactly — a release
// spinup cannot name is a release it cannot install.
func TestArchiveName(t *testing.T) {
	for _, tc := range []struct{ version, goos, goarch, want string }{
		{"v1.1.0", "darwin", "arm64", "spinup_1.1.0_darwin_arm64.tar.gz"},
		{"1.1.0", "darwin", "arm64", "spinup_1.1.0_darwin_arm64.tar.gz"},
		{"v1.1.0", "linux", "amd64", "spinup_1.1.0_linux_amd64.tar.gz"},
		{"v1.1.0", "windows", "amd64", "spinup_1.1.0_windows_amd64.zip"},
		{"v1.1.0", "windows", "arm64", "spinup_1.1.0_windows_arm64.zip"},
	} {
		if got := update.ArchiveName(tc.version, tc.goos, tc.goarch); got != tc.want {
			t.Errorf("ArchiveName(%q, %q, %q) = %q, want %q", tc.version, tc.goos, tc.goarch, got, tc.want)
		}
	}
}

// The binary prints a v-prefixed version and the tags carry one; nothing else
// does, so the comparison has to ignore it.
func TestSame(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"v1.1.0", "v1.1.0", true},
		{"1.1.0", "v1.1.0", true},
		{"v1.1.0", "v1.2.0", false},
		{"dev", "v1.1.0", false},
	} {
		if got := update.Same(tc.a, tc.b); got != tc.want {
			t.Errorf("Same(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestVerify(t *testing.T) {
	data := []byte("the binary")
	name := "spinup_1.1.0_linux_amd64.tar.gz"
	checksums := fmt.Sprintf("%s  %s\n%s  checksums-other.txt\n", sum(data), name, sum([]byte("x")))

	if err := update.Verify([]byte(checksums), data, name); err != nil {
		t.Errorf("Verify on matching data: %v", err)
	}

	// The failure that matters: something else arrived under the right name.
	if err := update.Verify([]byte(checksums), []byte("a different binary"), name); err == nil {
		t.Error("Verify accepted data that does not match its checksum")
	}

	// An unlisted file is not "no checksum, therefore fine".
	if err := update.Verify([]byte(checksums), data, "spinup_1.1.0_plan9_386.tar.gz"); err == nil {
		t.Error("Verify accepted a file with no checksum entry")
	}
}

func TestBinaryFromArchives(t *testing.T) {
	want := []byte("#!/bin/sh\necho spinup\n")

	got, err := update.Binary(tarGz(t, want), "linux")
	if err != nil {
		t.Fatalf("Binary from tar.gz: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("tar.gz gave %q, want %q", got, want)
	}

	got, err = update.Binary(zipped(t, want), "windows")
	if err != nil {
		t.Fatalf("Binary from zip: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("zip gave %q, want %q", got, want)
	}

	if _, err := update.Binary([]byte("not an archive"), "linux"); err == nil {
		t.Error("Binary accepted something that is not an archive")
	}
	if _, err := update.Binary(tarGz(t, want), "windows"); err == nil {
		t.Error("Binary read a tar.gz as a zip")
	}
}

// Replacing has to keep the file's permissions: a spinup that comes back
// without its executable bit is worse than one that is out of date.
func TestReplaceKeepsMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the running binary cannot be replaced on Windows, and modes differ")
	}

	path := filepath.Join(t.TempDir(), "spinup")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := update.Replace(path, []byte("new")); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("file holds %q, want %q", got, "new")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode is %v, want 0755", info.Mode().Perm())
	}

	// The temporary file it writes beside the target has to be gone either way.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("Replace left files behind: %v", names)
	}
}

// A directory spinup cannot write to is the common case — /usr/local/bin owned
// by root — and it has to fail without destroying what is already installed.
func TestReplaceLeavesTheOldBinaryOnFailure(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("root can write anywhere, and Windows permissions do not work this way")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "spinup")
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if err := update.Replace(path, []byte("new")); err == nil {
		t.Fatal("Replace into an unwritable directory succeeded")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old" {
		t.Errorf("the old binary is now %q, want %q", got, "old")
	}
}

func TestLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/someone/spinup/releases/latest" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("the request has no user agent; GitHub rejects those")
		}
		fmt.Fprint(w, `{"tag_name":"v1.2.0","assets":[
			{"name":"spinup_1.2.0_linux_amd64.tar.gz","browser_download_url":"`+r.Host+`/a"},
			{"name":"checksums.txt","browser_download_url":"`+r.Host+`/c"}]}`)
	}))
	defer srv.Close()

	c := &update.Client{API: srv.URL, Repo: "someone/spinup"}

	rel, err := c.Latest(t.Context())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if rel.Tag != "v1.2.0" {
		t.Errorf("Tag = %q, want v1.2.0", rel.Tag)
	}
	if len(rel.Assets) != 2 {
		t.Errorf("Assets = %v, want two of them", rel.Assets)
	}

	// An asset the release does not have is a specific failure, because it is
	// the one a new platform hits.
	if _, err := c.Asset(t.Context(), rel, "spinup_1.2.0_plan9_386.tar.gz"); !errors.Is(err, update.ErrNoAsset) {
		t.Errorf("Asset for a missing platform = %v, want ErrNoAsset", err)
	}
}

func TestLatestWithoutReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	defer srv.Close()

	c := &update.Client{API: srv.URL, Repo: "someone/spinup"}
	if _, err := c.Latest(t.Context()); err == nil {
		t.Fatal("Latest on a repository with no releases succeeded")
	} else if !strings.Contains(err.Error(), "404") {
		t.Errorf("error is %q, which does not say what the API answered", err)
	}
}

// Updating a binary Homebrew or Scoop owns is undone by the next upgrade, so
// spinup has to recognise those paths and say so instead.
func TestManagedBy(t *testing.T) {
	for _, tc := range []struct {
		path    string
		want    string
		managed bool
	}{
		{"/opt/homebrew/Cellar/spinup/1.1.0/bin/spinup", "Homebrew", true},
		{"/opt/homebrew/Caskroom/spinup/1.1.0/spinup", "Homebrew", true},
		{"/home/linuxbrew/.linuxbrew/bin/spinup", "Homebrew", true},
		{`C:\Users\u\scoop\apps\spinup\current\spinup.exe`, "Scoop", true},
		{"/usr/local/bin/spinup", "", false},
		{"/home/u/.local/bin/spinup", "", false},
	} {
		mgr, ok := update.ManagedBy(tc.path)
		if ok != tc.managed {
			t.Errorf("ManagedBy(%q) managed = %v, want %v", tc.path, ok, tc.managed)
		}
		if mgr.Name != tc.want {
			t.Errorf("ManagedBy(%q) = %q, want %q", tc.path, mgr.Name, tc.want)
		}
	}
}
