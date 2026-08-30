// Package catalog provides access to the stack catalog: the stacks compiled
// into the binary with go:embed, later overlaid by the user's own stacks in
// ~/.spinup/stacks (task 2.2).
//
// A catalog is just an fs.FS whose top level is one directory per stack, so
// the embedded tree, a materialised ~/.spinup/stacks and a test fixture are
// interchangeable.
package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
)

// ErrNotFound is returned when a stack is not in the catalog. Commands map it
// to exit code 3.
var ErrNotFound = errors.New("stack not found")

// RequiredFiles are the four files every stack ships; see docs/PLAN.md §4.
// scripts/lint-stacks.sh enforces the same list in CI.
var RequiredFiles = []string{"compose.yaml", ".env.example", "spinup.yaml", "README.md"}

// Stack names are kebab-case, which also makes them safe as path elements and
// as Compose project names. Same rule as scripts/lint-stacks.sh.
var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ValidName reports whether name is a well-formed stack name.
func ValidName(name string) bool { return namePattern.MatchString(name) }

// Catalog is a set of stacks backed by a file tree.
type Catalog struct {
	fsys fs.FS
}

// New returns a catalog reading from fsys, whose root holds one directory per
// stack.
func New(fsys fs.FS) *Catalog { return &Catalog{fsys: fsys} }

// Names returns every stack in the catalog, sorted. Directories that are not
// valid stack names are ignored, so a stray file cannot break `spinup list`.
func (c *Catalog) Names() ([]string, error) {
	entries, err := fs.ReadDir(c.fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("reading catalog: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() && ValidName(e.Name()) {
			names = append(names, e.Name())
		}
	}
	return names, nil // fs.ReadDir sorts by filename
}

// Has reports whether the catalog contains the named stack.
func (c *Catalog) Has(name string) bool {
	if !ValidName(name) {
		return false
	}
	info, err := fs.Stat(c.fsys, name)
	return err == nil && info.IsDir()
}

// FS returns the file tree of one stack, rooted at the stack's directory.
func (c *Catalog) FS(name string) (fs.FS, error) {
	if !c.Has(name) {
		return nil, fmt.Errorf("%q: %w", name, ErrNotFound)
	}
	sub, err := fs.Sub(c.fsys, name)
	if err != nil {
		return nil, fmt.Errorf("opening stack %q: %w", name, err)
	}
	return sub, nil
}

// ReadFile reads one file from a stack, e.g. ReadFile("postgres", "compose.yaml").
func (c *Catalog) ReadFile(stack, file string) ([]byte, error) {
	if !c.Has(stack) {
		return nil, fmt.Errorf("%q: %w", stack, ErrNotFound)
	}
	b, err := fs.ReadFile(c.fsys, path.Join(stack, file))
	if err != nil {
		return nil, fmt.Errorf("reading %s/%s: %w", stack, file, err)
	}
	return b, nil
}

// MissingFiles lists the RequiredFiles a stack does not have. An empty result
// means the stack is structurally complete.
func (c *Catalog) MissingFiles(stack string) ([]string, error) {
	if !c.Has(stack) {
		return nil, fmt.Errorf("%q: %w", stack, ErrNotFound)
	}

	var missing []string
	for _, f := range RequiredFiles {
		if _, err := fs.Stat(c.fsys, path.Join(stack, f)); err != nil {
			missing = append(missing, f)
		}
	}
	return missing, nil
}
