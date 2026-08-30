package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/config"
)

func TestCodeFor(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want int
	}{
		"plain error":     {errors.New("boom"), ExitUsage},
		"coded":           {failf(ExitCompose, "compose said no"), ExitCompose},
		"docker missing":  {failf(ExitDocker, "no daemon"), ExitDocker},
		"wrapped coded":   {fmt.Errorf("up: %w", failf(ExitCompose, "boom")), ExitCompose},
		"stack not found": {fmt.Errorf("%q: %w", "nope", catalog.ErrNotFound), ExitNotFound},
	} {
		if got := codeFor(tc.err); got != tc.want {
			t.Errorf("%s: codeFor(%v) = %d, want %d", name, tc.err, got, tc.want)
		}
	}
}

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	root := newRootCmd(Build{Version: "1.2.3", Commit: "abc1234", Date: "2026-01-01T00:00:00Z"})
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)

	err := root.Execute()
	return out.String(), err
}

func TestVersion(t *testing.T) {
	out, err := run(t, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	for _, want := range []string{"1.2.3", "abc1234", "2026-01-01T00:00:00Z", "go1."} {
		if !strings.Contains(out, want) {
			t.Errorf("version output is missing %q:\n%s", want, out)
		}
	}
}

func TestVersionShort(t *testing.T) {
	// --short is what a script parses, so it must be the bare version and
	// nothing else.
	out, err := run(t, "version", "--short")
	if err != nil {
		t.Fatalf("version --short: %v", err)
	}
	if out != "1.2.3\n" {
		t.Errorf("version --short = %q, want %q", out, "1.2.3\n")
	}
}

func TestUnknownCommandIsUsageError(t *testing.T) {
	_, err := run(t, "nosuchcommand")
	if err == nil {
		t.Fatal("unknown command: want an error")
	}
	if got := codeFor(err); got != ExitUsage {
		t.Errorf("unknown command exits %d, want %d", got, ExitUsage)
	}
}

func TestUnknownFlagIsUsageError(t *testing.T) {
	_, err := run(t, "--nosuchflag")
	if err == nil {
		t.Fatal("unknown flag: want an error")
	}
	if got := codeFor(err); got != ExitUsage {
		t.Errorf("unknown flag exits %d, want %d", got, ExitUsage)
	}
}

// The user's own stacks in ~/.spinup/stacks shadow the built-in ones. This is
// the wiring between config's paths and the catalog's layers, which nothing
// else exercises until `up` lands.
func TestUserCatalogLayersUserStacks(t *testing.T) {
	home := t.TempDir()
	t.Setenv(config.HomeEnv, home)

	dir := filepath.Join(home, "stacks", "my-thing")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const meta = "name: my-thing\ndescription: a stack of my own\ncategory: tooling\nprimary: thing\n" +
		"url: http://localhost:${THING_PORT}\nports:\n  - name: THING_PORT\n    default: 9999\n"
	if err := os.WriteFile(filepath.Join(dir, "spinup.yaml"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}

	cat := userCatalog(fstest.MapFS{
		"postgres/spinup.yaml": &fstest.MapFile{
			Data: []byte("name: postgres\ndescription: PostgreSQL 16\ncategory: database\nprimary: postgres\n" +
				"url: postgres://localhost:${POSTGRES_PORT}\nports:\n  - name: POSTGRES_PORT\n    default: 5432\n"),
		},
	})

	names, err := cat.Names()
	if err != nil {
		t.Fatalf("Names: %v", err)
	}
	if want := []string{"my-thing", "postgres"}; !slices.Equal(names, want) {
		t.Fatalf("Names = %v, want %v", names, want)
	}

	s, err := cat.Load("my-thing")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Origin != catalog.OriginUser {
		t.Errorf("Origin = %q, want %q", s.Origin, catalog.OriginUser)
	}
}
