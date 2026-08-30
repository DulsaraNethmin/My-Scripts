package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/config"
)

func TestDefaultUsesHome(t *testing.T) {
	t.Setenv(config.HomeEnv, "")

	p, err := config.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory here: %v", err)
	}
	if want := filepath.Join(home, config.DirName); p.Root != want {
		t.Errorf("Root = %q, want %q", p.Root, want)
	}
}

func TestDefaultHonoursSpinupHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.HomeEnv, dir)

	p, err := config.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	if p.Root != dir {
		t.Errorf("Root = %q, want %q", p.Root, dir)
	}
}

// A relative SPINUP_HOME has to be resolved once, up front: spinup runs
// docker compose from the stack directory, so a relative path would move.
func TestDefaultResolvesRelativeHome(t *testing.T) {
	t.Setenv(config.HomeEnv, "relative-spinup-home")

	p, err := config.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	if !filepath.IsAbs(p.Root) {
		t.Errorf("Root = %q, want an absolute path", p.Root)
	}
	if !strings.HasSuffix(p.Root, "relative-spinup-home") {
		t.Errorf("Root = %q, want it to end in the configured directory", p.Root)
	}
}

func TestPaths(t *testing.T) {
	p := config.At(filepath.Join("/tmp", "spinup-home"))

	for name, tc := range map[string]struct{ got, want string }{
		"config":  {p.ConfigFile(), filepath.Join(p.Root, "config.yaml")},
		"stacks":  {p.StacksDir(), filepath.Join(p.Root, "stacks")},
		"stack":   {p.StackDir("postgres"), filepath.Join(p.Root, "stacks", "postgres")},
		"env dir": {p.EnvDir(), filepath.Join(p.Root, "env")},
		"env":     {p.EnvFile("postgres"), filepath.Join(p.Root, "env", "postgres.env")},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", name, tc.got, tc.want)
		}
	}
}

func TestEnsure(t *testing.T) {
	p := config.At(filepath.Join(t.TempDir(), "spinup"))

	if err := p.Ensure(); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	for _, dir := range []string{p.Root, p.StacksDir(), p.EnvDir()} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("%s: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}

	// Ensure runs on every command, so it has to be idempotent.
	if err := p.Ensure(); err != nil {
		t.Errorf("second Ensure: %v", err)
	}
}
