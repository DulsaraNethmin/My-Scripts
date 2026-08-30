// Package config owns everything under ~/.spinup: the materialised stacks in
// stacks/, the per-stack env files in env/, and (task 2.3) config.yaml.
//
// Nothing is ever written into the repo or the install location — a user's
// state lives in one directory they can delete.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// HomeEnv overrides the spinup directory. Tests use it, and so can anyone who
// keeps their dotfiles somewhere unusual.
const HomeEnv = "SPINUP_HOME"

// DirName is the directory spinup keeps its state in, under the user's home.
const DirName = ".spinup"

// Paths resolves the files and directories under the spinup home.
type Paths struct {
	Root string
}

// DefaultPaths returns the paths rooted at $SPINUP_HOME, or ~/.spinup.
func DefaultPaths() (Paths, error) {
	if root := os.Getenv(HomeEnv); root != "" {
		abs, err := filepath.Abs(root)
		if err != nil {
			return Paths{}, fmt.Errorf("%s=%q: %w", HomeEnv, root, err)
		}
		return Paths{Root: abs}, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("locating the home directory (set %s to override): %w", HomeEnv, err)
	}
	return Paths{Root: filepath.Join(home, DirName)}, nil
}

// At returns paths rooted at an explicit directory.
func At(root string) Paths { return Paths{Root: root} }

// ConfigFile is the user's config.yaml.
func (p Paths) ConfigFile() string { return filepath.Join(p.Root, "config.yaml") }

// StacksDir holds the materialised stacks, one directory per stack. Anything
// here shadows the copy embedded in the binary.
func (p Paths) StacksDir() string { return filepath.Join(p.Root, "stacks") }

// StackDir is where one stack is materialised.
func (p Paths) StackDir(name string) string { return filepath.Join(p.StacksDir(), name) }

// EnvDir holds the per-stack env files.
func (p Paths) EnvDir() string { return filepath.Join(p.Root, "env") }

// EnvFile is a stack's env file: its ports and credentials.
func (p Paths) EnvFile(name string) string {
	return filepath.Join(p.EnvDir(), name+".env")
}

// Ensure creates the spinup directory tree. Everything else assumes it exists.
func (p Paths) Ensure() error {
	for _, dir := range []string{p.Root, p.StacksDir(), p.EnvDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}
	return nil
}
