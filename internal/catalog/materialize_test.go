package catalog_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
)

// withInit is a stack that has a subdirectory, like the real postgres and
// mysql stacks do for seed data.
func withInit() *catalog.Catalog {
	return catalog.New(merge(
		stackFiles("postgres", "PostgreSQL 16"),
		fstest.MapFS{
			"postgres/init/01-seed.sql": &fstest.MapFile{Data: []byte("select 1;\n")},
		},
	))
}

func TestMaterialize(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "stacks", "postgres")

	written, err := withInit().Materialize("postgres", dir)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	slices.Sort(written)
	want := []string{".env.example", "README.md", "compose.yaml", "init/01-seed.sql", "spinup.yaml"}
	if !slices.Equal(written, want) {
		t.Errorf("wrote %v, want %v", written, want)
	}

	for _, f := range want {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); err != nil {
			t.Errorf("%s was not written: %v", f, err)
		}
	}
}

// Materialising happens on every `up`, and the whole point of writing the
// stack out is that the user can edit it. A second run must not undo that.
func TestMaterializeKeepsUserEdits(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "postgres")

	if _, err := withInit().Materialize("postgres", dir); err != nil {
		t.Fatalf("first Materialize: %v", err)
	}

	edited := filepath.Join(dir, "compose.yaml")
	const mine = "services:\n  postgres:\n    image: postgres:17\n"
	if err := os.WriteFile(edited, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}

	written, err := withInit().Materialize("postgres", dir)
	if err != nil {
		t.Fatalf("second Materialize: %v", err)
	}
	if len(written) != 0 {
		t.Errorf("second Materialize rewrote %v, want nothing", written)
	}

	got, err := os.ReadFile(edited)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != mine {
		t.Errorf("compose.yaml was overwritten:\n%s", got)
	}
}

func TestMaterializeMissingStack(t *testing.T) {
	if _, err := fixture().Materialize("nope", t.TempDir()); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("Materialize of a missing stack = %v, want ErrNotFound", err)
	}
}

func TestSeedEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env", "postgres.env")

	wrote, err := fixture().SeedEnv("postgres", path)
	if err != nil {
		t.Fatalf("SeedEnv: %v", err)
	}
	if !wrote {
		t.Error("SeedEnv reported no write on a fresh directory")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the seeded env file: %v", err)
	}
	if string(got) != "# postgres\n" {
		t.Errorf("seeded content = %q", got)
	}

	// The env file holds the stack's passwords.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("env file mode = %04o, want 0600", perm)
		}
	}
}

// A user's ports and passwords live in the env file; re-seeding it on every up
// would throw them away.
func TestSeedEnvKeepsAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "postgres.env")
	const mine = "POSTGRES_PORT=5555\n"
	if err := os.WriteFile(path, []byte(mine), 0o600); err != nil {
		t.Fatal(err)
	}

	wrote, err := fixture().SeedEnv("postgres", path)
	if err != nil {
		t.Fatalf("SeedEnv: %v", err)
	}
	if wrote {
		t.Error("SeedEnv overwrote an existing env file")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != mine {
		t.Errorf("env file = %q, want it untouched", got)
	}
}

func TestSeedEnvMissingStack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.env")
	if _, err := fixture().SeedEnv("nope", path); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("SeedEnv of a missing stack = %v, want ErrNotFound", err)
	}
}
