// Package catalog provides access to the stack catalog: the stacks compiled
// into the binary with go:embed, shadowed by the user's own stacks in
// ~/.spinup/stacks.
//
// A catalog layer is just an fs.FS whose top level is one directory per stack,
// so the embedded tree, a materialised ~/.spinup/stacks and a test fixture are
// interchangeable.
package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
)

// ErrNotFound is returned when a stack is not in the catalog. Commands map it
// to exit code 3.
var ErrNotFound = errors.New("stack not found")

// RequiredFiles are the four files every stack ships; see docs/PLAN.md §4.
// scripts/lint-stacks.sh enforces the same list in CI.
var RequiredFiles = []string{"compose.yaml", ".env.example", "spinup.yaml", "README.md"}

// EnvExample is the file a stack's env file is seeded from.
const EnvExample = ".env.example"

// Stack names are kebab-case, which also makes them safe as path elements and
// as Compose project names. Same rule as scripts/lint-stacks.sh.
var namePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ValidName reports whether name is a well-formed stack name.
func ValidName(name string) bool { return namePattern.MatchString(name) }

// Catalog is a set of stacks backed by one or more file trees. Earlier layers
// shadow later ones, so a user's copy of a stack wins over the built-in.
type Catalog struct {
	layers []layer
}

type layer struct {
	origin Origin
	fsys   fs.FS
}

// New returns a catalog of built-in stacks reading from fsys, whose root holds
// one directory per stack.
func New(fsys fs.FS) *Catalog {
	return &Catalog{layers: []layer{{origin: OriginBuiltin, fsys: fsys}}}
}

// WithUserStacks returns a catalog in which stacks from fsys shadow the ones c
// already has. The directory not existing is not an error — most users never
// add a stack of their own.
func (c *Catalog) WithUserStacks(fsys fs.FS) *Catalog {
	layers := make([]layer, 0, len(c.layers)+1)
	layers = append(layers, layer{origin: OriginUser, fsys: fsys})
	layers = append(layers, c.layers...)
	return &Catalog{layers: layers}
}

// find returns the layer that owns a stack. Shadowing is per stack, not per
// file: whichever layer has the directory owns all of it, so deleting a file
// from a materialised stack does not silently fall back to the built-in one.
func (c *Catalog) find(name string) (layer, bool) {
	if !ValidName(name) {
		return layer{}, false
	}
	for _, l := range c.layers {
		if info, err := fs.Stat(l.fsys, name); err == nil && info.IsDir() {
			return l, true
		}
	}
	return layer{}, false
}

// Names returns every stack in the catalog, sorted and de-duplicated across
// layers. Directories that are not valid stack names are ignored, so a stray
// file cannot break `spin list`.
func (c *Catalog) Names() ([]string, error) {
	seen := map[string]bool{}

	for _, l := range c.layers {
		entries, err := fs.ReadDir(l.fsys, ".")
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue // an empty or absent ~/.spinup/stacks is normal
			}
			return nil, fmt.Errorf("reading %s catalog: %w", l.origin, err)
		}
		for _, e := range entries {
			if e.IsDir() && ValidName(e.Name()) {
				seen[e.Name()] = true
			}
		}
	}

	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	slices.Sort(names)
	return names, nil
}

// Has reports whether the catalog contains the named stack.
func (c *Catalog) Has(name string) bool {
	_, ok := c.find(name)
	return ok
}

// HasBuiltin reports whether the binary ships a copy of the stack, whatever the
// user's own catalog holds. `spin reset` needs it: restoring a stack means
// deleting the user's copy, which for a stack that only exists there would not
// be a restore but a delete.
func (c *Catalog) HasBuiltin(name string) bool {
	if !ValidName(name) {
		return false
	}
	for _, l := range c.layers {
		if l.origin != OriginBuiltin {
			continue
		}
		if info, err := fs.Stat(l.fsys, name); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// Origin says which layer a stack comes from.
func (c *Catalog) Origin(name string) (Origin, error) {
	l, ok := c.find(name)
	if !ok {
		return "", fmt.Errorf("%q: %w", name, ErrNotFound)
	}
	return l.origin, nil
}

// FS returns the file tree of one stack, rooted at the stack's directory.
func (c *Catalog) FS(name string) (fs.FS, error) {
	l, ok := c.find(name)
	if !ok {
		return nil, fmt.Errorf("%q: %w", name, ErrNotFound)
	}
	sub, err := fs.Sub(l.fsys, name)
	if err != nil {
		return nil, fmt.Errorf("opening stack %q: %w", name, err)
	}
	return sub, nil
}

// ReadFile reads one file from a stack, e.g. ReadFile("postgres", "compose.yaml").
func (c *Catalog) ReadFile(stack, file string) ([]byte, error) {
	l, ok := c.find(stack)
	if !ok {
		return nil, fmt.Errorf("%q: %w", stack, ErrNotFound)
	}
	b, err := fs.ReadFile(l.fsys, path.Join(stack, file))
	if err != nil {
		return nil, fmt.Errorf("reading %s/%s: %w", stack, file, err)
	}
	return b, nil
}

// MissingFiles lists the RequiredFiles a stack does not have. An empty result
// means the stack is structurally complete.
func (c *Catalog) MissingFiles(stack string) ([]string, error) {
	l, ok := c.find(stack)
	if !ok {
		return nil, fmt.Errorf("%q: %w", stack, ErrNotFound)
	}

	var missing []string
	for _, f := range RequiredFiles {
		if _, err := fs.Stat(l.fsys, path.Join(stack, f)); err != nil {
			missing = append(missing, f)
		}
	}
	return missing, nil
}

// Load parses a stack's spinup.yaml.
func (c *Catalog) Load(name string) (*Stack, error) {
	l, ok := c.find(name)
	if !ok {
		return nil, fmt.Errorf("%q: %w", name, ErrNotFound)
	}

	data, err := fs.ReadFile(l.fsys, path.Join(name, "spinup.yaml"))
	if err != nil {
		return nil, fmt.Errorf("reading %s/spinup.yaml: %w", name, err)
	}

	s, err := ParseStack(name, data)
	if err != nil {
		return nil, err
	}
	s.Origin = l.origin
	return s, nil
}

// All parses every stack in the catalog. Stacks that fail to parse are
// reported in the error but do not hide the ones that are fine — one broken
// stack in ~/.spinup/stacks must not break `spin list`.
func (c *Catalog) All() ([]*Stack, error) {
	names, err := c.Names()
	if err != nil {
		return nil, err
	}

	stacks := make([]*Stack, 0, len(names))
	var errs []error
	for _, name := range names {
		s, err := c.Load(name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		stacks = append(stacks, s)
	}
	return stacks, errors.Join(errs...)
}

// Materialize copies a stack's files into dir, creating it if needed, and
// returns the paths it wrote relative to dir.
//
// Existing files are never overwritten: materialising happens on every `up`,
// and the whole point of writing the stack out is that the user can edit it.
// `spin reset` (task 4.2) is the way back to the built-in copy.
func (c *Catalog) Materialize(name, dir string) ([]string, error) {
	src, err := c.FS(name)
	if err != nil {
		return nil, err
	}

	var written []string
	err = fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		target := filepath.Join(dir, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		if _, err := os.Stat(target); err == nil {
			return nil // the user's copy wins
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		data, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
		written = append(written, p)
		return nil
	})
	if err != nil {
		return written, fmt.Errorf("materialising %s into %s: %w", name, dir, err)
	}
	return written, nil
}

// SeedEnv writes a stack's .env.example to path unless it already exists, and
// reports whether it wrote it. The env file is where a user's ports and
// passwords live, so overwriting it is never the right move.
func (c *Catalog) SeedEnv(name, path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}

	data, err := c.ReadFile(name, EnvExample)
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	// 0600: this file holds the stack's passwords.
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return false, fmt.Errorf("writing %s: %w", path, err)
	}
	return true, nil
}
