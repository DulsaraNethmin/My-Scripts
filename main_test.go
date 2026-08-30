package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
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

// The CLI is generic: it reads spinup.yaml and nothing else. A stack whose
// metadata does not parse breaks every command that touches it, so the whole
// shipped catalog is checked here rather than one stack at a time.
func TestEmbeddedStacksParse(t *testing.T) {
	cat := catalog.New(embeddedStacks())

	stacks, err := cat.All()
	if err != nil {
		t.Fatalf("parsing the embedded catalog:\n%v", err)
	}
	if len(stacks) == 0 {
		t.Fatal("the embedded catalog is empty")
	}

	for _, s := range stacks {
		if s.Origin != catalog.OriginBuiltin {
			t.Errorf("%s: Origin = %q, want builtin", s.Name, s.Origin)
		}
		// A GUI in a container of its own is optional and belongs behind the
		// gui profile; a GUI served by the primary service itself has nothing
		// to gate.
		if s.HasGUI() && s.GUI.Service != s.Primary && !s.HasProfile("gui") {
			t.Errorf("%s: GUI %q is a separate container but is not behind the gui profile",
				s.Name, s.GUI.Service)
		}
		t.Logf("%-20s %-8s %v", s.Name, s.Category, s.PortNames())
	}
}

// stacks/pytorch puts both of its services behind profiles, and they share
// ports — so with no profile selected the stack starts nothing at all.
// default_profiles is what makes `spinup up pytorch` do something.
func TestPytorchDeclaresDefaultProfiles(t *testing.T) {
	s, err := catalog.New(embeddedStacks()).Load("pytorch")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(s.DefaultProfiles) == 0 {
		t.Error("pytorch declares no default_profiles: `spinup up pytorch` would start nothing")
	}
	for _, p := range s.DefaultProfiles {
		if !s.HasProfile(p) {
			t.Errorf("default profile %q is not one of %v", p, s.Profiles)
		}
	}
	if !s.HasGPU() {
		t.Error("pytorch declares no gpu block: `spinup up --gpu` would have nothing to select")
	}
}

// Materialising is how a stack gets from the binary into ~/.spinup/stacks,
// where the user can read and edit it. What lands there must be exactly what
// the repo ships — including init/ subdirectories and dotfiles.
func TestMaterializeEveryStackMatchesTheRepo(t *testing.T) {
	cat := catalog.New(embeddedStacks())
	names, err := cat.Names()
	if err != nil {
		t.Fatalf("Names: %v", err)
	}

	root := t.TempDir()
	for _, name := range names {
		dir := filepath.Join(root, name)
		if _, err := cat.Materialize(name, dir); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}

		want := walk(t, os.DirFS(filepath.Join("stacks", name)))
		got := walk(t, os.DirFS(dir))
		for f := range want {
			if !got[f] {
				t.Errorf("%s: %s was not materialised", name, f)
				continue
			}
			a, err := os.ReadFile(filepath.Join("stacks", name, filepath.FromSlash(f)))
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f)))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(a, b) {
				t.Errorf("%s: %s differs from the repo copy", name, f)
			}
		}
		for f := range got {
			if !want[f] {
				t.Errorf("%s: materialised %s, which is not in the repo", name, f)
			}
		}
	}
}
