//go:build !windows

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// install.sh is the one part of spinup a user runs before they have spinup, so
// it is worth testing rather than reading. These tests stand up a GitHub-shaped
// release on a local server and run the real script against it.

const installMarker = "this is the installed spinup"

func fakeRelease(t *testing.T, tag string) (archive []byte, name string) {
	t.Helper()

	binary := []byte("#!/bin/sh\necho '" + installMarker + "'\n")

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range []struct {
		name string
		data []byte
	}{{"LICENSE", []byte("MIT")}, {"spin", binary}, {"spinup", binary}} {
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

	name = fmt.Sprintf("spinup_%s_%s_%s.tar.gz", strings.TrimPrefix(tag, "v"), runtime.GOOS, runtime.GOARCH)
	return buf.Bytes(), name
}

// serveRelease publishes one release, with whatever checksums it is given —
// which is not always the truth, so the verification can be tested too.
func serveRelease(t *testing.T, tag, name string, archive []byte, checksums string) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/download/"+name, func(w http.ResponseWriter, _ *http.Request) {
		w.Write(archive) //nolint:errcheck // a test server
	})
	mux.HandleFunc("/download/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, checksums)
	})
	mux.HandleFunc("/repos/test/spinup/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"tag_name":%q,"assets":[
			{"name":%q,"browser_download_url":"%s/download/%s"},
			{"name":"checksums.txt","browser_download_url":"%s/download/checksums.txt"}]}`,
			tag, name, srv.URL, name, srv.URL)
	})

	return srv
}

func checksums(name string, archive []byte) string {
	sum := sha256.Sum256(archive)
	return fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), name)
}

func runInstall(t *testing.T, api, dir string, args ...string) (string, error) {
	t.Helper()

	if _, err := exec.LookPath("curl"); err != nil {
		if _, err := exec.LookPath("wget"); err != nil {
			t.Skip("install.sh needs curl or wget")
		}
	}

	cmd := exec.Command("sh", append([]string{"install.sh", "--dir", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"SPINUP_API="+api,
		"SPINUP_REPO=test/spinup",
		"NO_COLOR=1",
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestInstallScript(t *testing.T) {
	tag := "v9.9.9"
	archive, name := fakeRelease(t, tag)
	srv := serveRelease(t, tag, name, archive, checksums(name, archive))

	dir := t.TempDir()
	out, err := runInstall(t, srv.URL, dir)
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, tag) {
		t.Errorf("install.sh does not report the version it installed:\n%s", out)
	}

	// Both names, because both are what a release ships and the alias is the
	// only thing keeping `spinup ...` working for anyone already on it.
	for _, name := range []string{"spin", "spinup"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("%s was not installed: %v\n%s", name, err, out)
		}
		if info.Mode().Perm()&0o111 == 0 {
			t.Errorf("the installed %s is not executable: %v", name, info.Mode())
		}
	}

	path := filepath.Join(dir, "spin")

	// The point of the whole script: what lands on the PATH runs.
	got, err := exec.Command(path).Output()
	if err != nil {
		t.Fatalf("running the installed binary: %v", err)
	}
	if strings.TrimSpace(string(got)) != installMarker {
		t.Errorf("the installed binary printed %q, want %q", got, installMarker)
	}
}

// A download that does not match the release's checksums is the case the whole
// verification exists for: nothing may be installed.
func TestInstallScriptRefusesAWrongChecksum(t *testing.T) {
	tag := "v9.9.9"
	archive, name := fakeRelease(t, tag)
	srv := serveRelease(t, tag, name, archive, checksums(name, []byte("something else entirely")))

	dir := t.TempDir()
	out, err := runInstall(t, srv.URL, dir)
	if err == nil {
		t.Fatalf("install.sh installed an archive that does not match its checksum:\n%s", out)
	}
	if !strings.Contains(out, "checksum") {
		t.Errorf("the failure does not mention the checksum:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, "spin")); !os.IsNotExist(err) {
		t.Error("install.sh installed the binary anyway")
	}
}

// A tag with no build for this platform has to say so, not fail on a 404 from
// somewhere deeper in the script.
func TestInstallScriptWithNoBuildForThisPlatform(t *testing.T) {
	tag := "v9.9.9"
	archive, _ := fakeRelease(t, tag)
	name := "spinup_9.9.9_plan9_386.tar.gz"
	srv := serveRelease(t, tag, name, archive, checksums(name, archive))

	out, err := runInstall(t, srv.URL, t.TempDir())
	if err == nil {
		t.Fatalf("install.sh succeeded with no archive for this platform:\n%s", out)
	}
	if !strings.Contains(out, runtime.GOOS) {
		t.Errorf("the failure does not name the platform it looked for:\n%s", out)
	}
}
