package cmd

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
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

// archiveFor builds the release archive this platform would download, so the
// test exercises the same path a real update takes on the machine it runs on.
func archiveFor(t *testing.T, binary []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	if runtime.GOOS == "windows" {
		zw := zip.NewWriter(&buf)
		w, err := zw.Create("spinup.exe")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(binary); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}

	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name: "spinup", Mode: 0o755, Size: int64(len(binary)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// release stands up a GitHub-shaped release for one tag, and points spinup at
// it through the environment. checksums is what the server will publish, which
// is not always the truth — that is the point of one of the tests.
func release(t *testing.T, tag string, archive []byte, checksums string) {
	t.Helper()

	name := update.ArchiveName(tag, runtime.GOOS, runtime.GOARCH)

	mux := http.NewServeMux()
	mux.HandleFunc("/download/"+name, func(w http.ResponseWriter, _ *http.Request) {
		w.Write(archive) //nolint:errcheck // a test server
	})
	mux.HandleFunc("/download/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, checksums)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/repos/someone/spinup/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"tag_name":%q,"assets":[
			{"name":%q,"browser_download_url":"%s/download/%s"},
			{"name":"checksums.txt","browser_download_url":"%s/download/checksums.txt"}]}`,
			tag, name, srv.URL, name, srv.URL)
	})

	t.Setenv("SPINUP_API", srv.URL)
	t.Setenv("SPINUP_REPO", "someone/spinup")
}

func checksumsFor(tag string, archive []byte) string {
	sum := sha256.Sum256(archive)
	return fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]),
		update.ArchiveName(tag, runtime.GOOS, runtime.GOARCH))
}

// installed puts a stand-in for the running binary somewhere writable and
// points the command at it, so an update in a test replaces that file rather
// than the test binary.
func installed(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "spinup")
	if err := os.WriteFile(path, []byte("the old binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	old := executablePath
	executablePath = func() (string, error) { return path, nil }
	t.Cleanup(func() { executablePath = old })

	return path
}

func TestUpdateInstallsTheLatestRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Replace moves the running binary aside on Windows; not this file")
	}

	path := installed(t)
	binary := []byte("the new binary")
	archive := archiveFor(t, binary)
	release(t, "v1.9.0", archive, checksumsFor("v1.9.0", archive))

	out, err := run(t, "update")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !strings.Contains(out, "v1.9.0") {
		t.Errorf("output does not name the new version:\n%s", out)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, binary) {
		t.Errorf("the installed binary is %q, want %q", got, binary)
	}
}

// The checksum is the only thing standing between a user and whatever a proxy
// felt like serving, so a mismatch has to stop the update and leave the working
// binary alone.
func TestUpdateRefusesAWrongChecksum(t *testing.T) {
	path := installed(t)
	archive := archiveFor(t, []byte("the new binary"))
	release(t, "v1.9.0", archive, checksumsFor("v1.9.0", []byte("something else entirely")))

	if _, err := run(t, "update"); err == nil {
		t.Fatal("update installed a binary that does not match its checksum")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "the old binary" {
		t.Errorf("the working binary was replaced anyway: %q", got)
	}
}

func TestUpdateWhenAlreadyCurrent(t *testing.T) {
	installed(t)
	archive := archiveFor(t, []byte("same"))
	release(t, "v1.2.3", archive, checksumsFor("v1.2.3", archive)) // run() builds version 1.2.3

	out, err := run(t, "update")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !strings.Contains(out, "latest release") {
		t.Errorf("output does not say it is up to date:\n%s", out)
	}
}

func TestUpdateCheckDoesNotInstall(t *testing.T) {
	path := installed(t)
	archive := archiveFor(t, []byte("the new binary"))
	release(t, "v1.9.0", archive, checksumsFor("v1.9.0", archive))

	out, err := run(t, "update", "--check")
	if err != nil {
		t.Fatalf("update --check: %v", err)
	}
	if !strings.Contains(out, "v1.9.0") {
		t.Errorf("--check does not report the new version:\n%s", out)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "the old binary" {
		t.Errorf("--check replaced the binary: %q", got)
	}
}

// Overwriting a binary Homebrew owns is undone by the next `brew upgrade`, so
// update points at brew rather than doing it.
func TestUpdateDefersToThePackageManager(t *testing.T) {
	old := executablePath
	executablePath = func() (string, error) {
		return "/opt/homebrew/Cellar/spinup/1.2.3/bin/spinup", nil
	}
	t.Cleanup(func() { executablePath = old })

	_, err := run(t, "update")
	if err == nil {
		t.Fatal("update overwrote a Homebrew-managed binary")
	}
	if !strings.Contains(err.Error(), "brew upgrade spinup") {
		t.Errorf("the error does not say what to run instead: %v", err)
	}
}
