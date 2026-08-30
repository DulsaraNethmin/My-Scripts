package main

import (
	"io/fs"
	"os"
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
)

// walk returns every file (not directory) in fsys, as a set of paths.
func walk(t *testing.T, fsys fs.FS) map[string]bool {
	t.Helper()

	files := map[string]bool{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files[p] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking: %v", err)
	}
	return files
}

// The catalog is compiled in, so a mistake in the go:embed pattern — dropping
// every .env.example by leaving off the all: prefix, say — is invisible until
// someone runs a released binary. Compare the embedded tree to stacks/ on disk.
func TestEmbeddedCatalogMatchesRepo(t *testing.T) {
	onDisk := walk(t, os.DirFS("stacks"))
	embedded := walk(t, embeddedStacks())

	if len(onDisk) == 0 {
		t.Fatal("stacks/ is empty — the test is not looking where it thinks")
	}

	for f := range onDisk {
		if !embedded[f] {
			t.Errorf("stacks/%s is not embedded in the binary", f)
		}
	}
	for f := range embedded {
		if !onDisk[f] {
			t.Errorf("%s is embedded but not in stacks/", f)
		}
	}
}

// Every embedded stack must be structurally complete, or `spinup up` on it
// fails in the field rather than in CI.
func TestEmbeddedStacksAreComplete(t *testing.T) {
	cat := catalog.New(embeddedStacks())

	names, err := cat.Names()
	if err != nil {
		t.Fatalf("Names: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("the embedded catalog is empty")
	}

	for _, name := range names {
		missing, err := cat.MissingFiles(name)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(missing) > 0 {
			t.Errorf("%s is missing %v", name, missing)
		}
	}
	t.Logf("%d stacks embedded: %v", len(names), names)
}
