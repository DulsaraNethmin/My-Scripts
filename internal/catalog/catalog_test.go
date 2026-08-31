package catalog_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
)

// stackFiles is a complete, valid stack directory.
func stackFiles(name, description string) fstest.MapFS {
	return fstest.MapFS{
		name + "/compose.yaml": &fstest.MapFile{Data: []byte("services:\n  " + name + ":\n    image: " + name + ":1\n")},
		name + "/.env.example": &fstest.MapFile{Data: []byte("# " + name + "\n")},
		name + "/README.md":    &fstest.MapFile{Data: []byte("# " + name + "\n")},
		name + "/spinup.yaml": &fstest.MapFile{Data: []byte(fmt.Sprintf(
			"name: %s\ndescription: %s\ncategory: database\nprimary: %s\n"+
				"url: %s://localhost:${%s_PORT}\nports:\n  - name: %s_PORT\n    default: 5432\n",
			name, description, name, name, envName(name), envName(name)))},
	}
}

// envName turns a stack name into the env-var prefix its ports use.
func envName(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

func merge(all ...fstest.MapFS) fstest.MapFS {
	out := fstest.MapFS{}
	for _, m := range all {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// fixture is a two-stack catalog plus the junk a real directory accumulates: a
// loose file, a folder that is not a stack, and an incomplete stack.
func fixture() *catalog.Catalog {
	return catalog.New(merge(
		stackFiles("postgres", "PostgreSQL 16"),
		stackFiles("nginx-static", "nginx serving a static site"),
		fstest.MapFS{
			"README.md":            &fstest.MapFile{Data: []byte("not a stack")},
			"Not-A-Stack/file.txt": &fstest.MapFile{Data: []byte("wrong case")},
			"half/compose.yaml":    &fstest.MapFile{Data: []byte("incomplete")},
		},
	))
}

func TestNames(t *testing.T) {
	got, err := fixture().Names()
	if err != nil {
		t.Fatalf("Names: %v", err)
	}

	// Sorted, directories only, and nothing that is not a valid stack name.
	want := []string{"half", "nginx-static", "postgres"}
	if !slices.Equal(got, want) {
		t.Errorf("Names = %v, want %v", got, want)
	}
}

func TestHas(t *testing.T) {
	c := fixture()
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"postgres", true},
		{"nginx-static", true},
		{"redis", false},       // absent
		{"README.md", false},   // a file, not a stack
		{"Not-A-Stack", false}, // present, but not a valid name
		{"", false},            // empty
		{"../etc", false},      // traversal
		{"postgres/..", false}, // traversal
		{"POSTGRES", false},    // wrong case
	} {
		if got := c.Has(tc.name); got != tc.want {
			t.Errorf("Has(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestReadFile(t *testing.T) {
	c := fixture()

	got, err := c.ReadFile("postgres", "compose.yaml")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(got), "image: postgres:1") {
		t.Errorf("ReadFile = %q", got)
	}

	// A dotfile must be readable: .env.example is the one every stack ships,
	// and it is exactly what a careless go:embed pattern drops.
	if _, err := c.ReadFile("postgres", catalog.EnvExample); err != nil {
		t.Errorf("ReadFile(.env.example): %v", err)
	}

	if _, err := c.ReadFile("nope", "compose.yaml"); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("ReadFile of a missing stack = %v, want ErrNotFound", err)
	}
	if _, err := c.ReadFile("postgres", "no-such-file"); err == nil {
		t.Error("ReadFile of a missing file: want an error")
	}
}

func TestFS(t *testing.T) {
	sub, err := fixture().FS("postgres")
	if err != nil {
		t.Fatalf("FS: %v", err)
	}
	if _, err := fs.Stat(sub, "spinup.yaml"); err != nil {
		t.Errorf("stack FS is not rooted at the stack directory: %v", err)
	}

	if _, err := fixture().FS("nope"); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("FS of a missing stack = %v, want ErrNotFound", err)
	}
}

// HasBuiltin answers a question Origin cannot: whether the binary still has a
// copy of a stack the user has since edited. `spin reset` turns on it —
// restoring means deleting the user's copy, which for a stack that exists only
// there would be a delete, not a restore.
func TestHasBuiltin(t *testing.T) {
	c := catalog.New(stackFiles("postgres", "PostgreSQL 16")).
		WithUserStacks(merge(stackFiles("postgres", "my edited postgres"), stackFiles("mine", "all my own")))

	if !c.HasBuiltin("postgres") {
		t.Error("HasBuiltin(postgres) = false, but the built-in copy is still there")
	}
	if c.HasBuiltin("mine") {
		t.Error("HasBuiltin(mine) = true, but it only exists in the user's catalog")
	}
	if c.HasBuiltin("nosuchstack") {
		t.Error("HasBuiltin of a stack nothing has = true")
	}
	if c.HasBuiltin("../escape") {
		t.Error("HasBuiltin accepted a path as a stack name")
	}

	// The user's copy still wins for everything else.
	if origin, err := c.Origin("postgres"); err != nil || origin != catalog.OriginUser {
		t.Errorf("Origin(postgres) = %q, %v; want user", origin, err)
	}
}

// A scaffolded stack has to load, or `spin new` hands the user something
// broken and blames them for it.
func TestScaffoldLoads(t *testing.T) {
	files := catalog.Scaffold("my-thing")

	mapfs := fstest.MapFS{}
	for path, data := range files {
		mapfs["my-thing/"+path] = &fstest.MapFile{Data: data}
	}

	for _, want := range catalog.RequiredFiles {
		if _, ok := files[want]; !ok {
			t.Errorf("Scaffold did not produce %s", want)
		}
	}

	s, err := catalog.New(mapfs).Load("my-thing")
	if err != nil {
		t.Fatalf("a scaffolded stack does not load: %v", err)
	}
	if s.Primary == "" || len(s.Ports) == 0 || s.URL == "" {
		t.Errorf("Scaffold produced an under-specified stack: %+v", s)
	}
}

func TestMissingFiles(t *testing.T) {
	c := fixture()

	if missing, err := c.MissingFiles("postgres"); err != nil || len(missing) != 0 {
		t.Errorf("MissingFiles(postgres) = %v, %v; want none", missing, err)
	}

	missing, err := c.MissingFiles("half")
	if err != nil {
		t.Fatalf("MissingFiles: %v", err)
	}
	want := []string{".env.example", "spinup.yaml", "README.md"}
	if !slices.Equal(missing, want) {
		t.Errorf("MissingFiles(half) = %v, want %v", missing, want)
	}
}

func TestValidName(t *testing.T) {
	for name, want := range map[string]bool{
		"postgres":            true,
		"nginx-proxy-manager": true,
		"pytorch":             true,
		"s3":                  true,
		"":                    false,
		"-leading":            false,
		"Upper":               false,
		"under_score":         false,
		"dot.name":            false,
		"with space":          false,
		"..":                  false,
		"a/b":                 false,
	} {
		if got := catalog.ValidName(name); got != want {
			t.Errorf("ValidName(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestContext(t *testing.T) {
	if _, ok := catalog.FromContext(context.Background()); ok {
		t.Error("FromContext on a bare context: want not ok")
	}

	c := fixture()
	if got, ok := catalog.FromContext(catalog.NewContext(context.Background(), c)); !ok || got != c {
		t.Error("FromContext did not return the catalog that was stored")
	}
}

func TestLoad(t *testing.T) {
	s, err := fixture().Load("postgres")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Name != "postgres" || s.Description != "PostgreSQL 16" {
		t.Errorf("Load = %+v", s)
	}
	if s.Origin != catalog.OriginBuiltin {
		t.Errorf("Origin = %q, want %q", s.Origin, catalog.OriginBuiltin)
	}

	if _, err := fixture().Load("nope"); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("Load of a missing stack = %v, want ErrNotFound", err)
	}
	if _, err := fixture().Load("half"); err == nil {
		t.Error("Load of a stack with no spinup.yaml: want an error")
	}
}

// One broken stack in ~/.spinup/stacks must not take `spin list` down with
// it: the good ones still come back, and the error names the bad one.
func TestAllKeepsGoingPastABrokenStack(t *testing.T) {
	stacks, err := fixture().All()
	if err == nil {
		t.Error("want an error naming the incomplete stack")
	} else if !strings.Contains(err.Error(), "half") {
		t.Errorf("error does not name the broken stack: %v", err)
	}

	var names []string
	for _, s := range stacks {
		names = append(names, s.Name)
	}
	if want := []string{"nginx-static", "postgres"}; !slices.Equal(names, want) {
		t.Errorf("All returned %v, want %v", names, want)
	}
}

func TestUserStacksShadowBuiltins(t *testing.T) {
	builtin := catalog.New(merge(
		stackFiles("postgres", "PostgreSQL 16"),
		stackFiles("redis", "Redis 7"),
	))
	user := merge(
		stackFiles("postgres", "my patched postgres"),
		stackFiles("my-thing", "a stack of my own"),
	)
	c := builtin.WithUserStacks(user)

	names, err := c.Names()
	if err != nil {
		t.Fatalf("Names: %v", err)
	}
	if want := []string{"my-thing", "postgres", "redis"}; !slices.Equal(names, want) {
		t.Errorf("Names = %v, want %v (de-duplicated across layers)", names, want)
	}

	s, err := c.Load("postgres")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Description != "my patched postgres" {
		t.Errorf("the built-in postgres shadowed the user's: %q", s.Description)
	}
	if s.Origin != catalog.OriginUser {
		t.Errorf("Origin = %q, want %q", s.Origin, catalog.OriginUser)
	}

	if origin, err := c.Origin("redis"); err != nil || origin != catalog.OriginBuiltin {
		t.Errorf("Origin(redis) = %q, %v; want builtin", origin, err)
	}
	if _, err := c.Origin("nope"); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("Origin of a missing stack = %v, want ErrNotFound", err)
	}
}

// ~/.spinup/stacks does not exist until someone makes it, which is the common
// case and must not be an error.
func TestUserStacksDirectoryMayNotExist(t *testing.T) {
	c := catalog.New(stackFiles("postgres", "PostgreSQL 16")).
		WithUserStacks(os.DirFS(filepath.Join(t.TempDir(), "does-not-exist")))

	names, err := c.Names()
	if err != nil {
		t.Fatalf("Names with an absent user directory: %v", err)
	}
	if !slices.Equal(names, []string{"postgres"}) {
		t.Errorf("Names = %v, want [postgres]", names)
	}
	if !c.Has("postgres") {
		t.Error("Has(postgres) = false")
	}
}
