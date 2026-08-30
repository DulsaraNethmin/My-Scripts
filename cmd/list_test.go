package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func listCatalog() *catalog.Catalog {
	stack := func(name, desc, category string) []byte {
		return []byte("name: " + name + "\ndescription: " + desc + "\ncategory: " + category +
			"\nprimary: " + name + "\nurl: http://localhost:${X_PORT}\nports:\n  - name: X_PORT\n    default: 1234\n")
	}
	return catalog.New(fstest.MapFS{
		"postgres/spinup.yaml": &fstest.MapFile{Data: stack("postgres", "PostgreSQL 16", "database")},
		"redis/spinup.yaml":    &fstest.MapFile{Data: stack("redis", "Redis 7", "database")},
		"broken/spinup.yaml":   &fstest.MapFile{Data: []byte("name: broken\n")},
	})
}

// -q is the scripting interface: names, one per line, nothing else.
func TestListQuiet(t *testing.T) {
	var out, errOut bytes.Buffer

	c := newListCmd()
	c.SetOut(&out)
	c.SetErr(&errOut)
	c.SetArgs([]string{"--quiet"})

	err := c.ExecuteContext(catalog.NewContext(context.Background(), listCatalog()))
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if got := strings.Fields(out.String()); len(got) != 2 || got[0] != "postgres" || got[1] != "redis" {
		t.Errorf("list -q = %q", out.String())
	}
}

// One unparseable stack in ~/.spinup/stacks must not take the catalog down
// with it — it warns and lists the rest.
func TestListWarnsAboutABrokenStack(t *testing.T) {
	ui.SetColor(false)
	t.Cleanup(func() { ui.SetColor(true) })

	var out, errOut bytes.Buffer

	c := newListCmd()
	c.SetOut(&out)
	c.SetErr(&errOut)
	c.SetArgs([]string{"--quiet"})

	if err := c.ExecuteContext(catalog.NewContext(context.Background(), listCatalog())); err != nil {
		t.Fatalf("list: %v", err)
	}

	if !strings.Contains(errOut.String(), "broken") {
		t.Errorf("stderr does not mention the broken stack: %q", errOut.String())
	}
	if !strings.Contains(out.String(), "postgres") {
		t.Errorf("the good stacks were not listed: %q", out.String())
	}
}
