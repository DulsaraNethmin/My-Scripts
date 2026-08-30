package catalog_test

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
)

// fixture is a two-stack catalog plus the kind of junk a real directory
// accumulates: a loose file and a folder that is not a stack.
func fixture() *catalog.Catalog {
	stack := func(name string) fstest.MapFS {
		m := fstest.MapFS{}
		for _, f := range catalog.RequiredFiles {
			m[name+"/"+f] = &fstest.MapFile{Data: []byte(name + " " + f)}
		}
		return m
	}

	all := fstest.MapFS{
		"README.md":            &fstest.MapFile{Data: []byte("not a stack")},
		"Not-A-Stack/file.txt": &fstest.MapFile{Data: []byte("wrong case")},
		"half/compose.yaml":    &fstest.MapFile{Data: []byte("incomplete")},
	}
	for _, s := range []fstest.MapFS{stack("postgres"), stack("nginx-static")} {
		for k, v := range s {
			all[k] = v
		}
	}
	return catalog.New(all)
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
	if string(got) != "postgres compose.yaml" {
		t.Errorf("ReadFile = %q", got)
	}

	// A dotfile must be readable: .env.example is the one every stack ships,
	// and it is exactly what a careless go:embed pattern drops.
	if _, err := c.ReadFile("postgres", ".env.example"); err != nil {
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
