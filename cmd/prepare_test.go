package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/compose"
	"github.com/DulsaraNethmin/spinup/internal/config"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

// prepareCatalog is a one-stack catalog with the shape of a real one: a GUI in
// its own container, a subdirectory, and an .env.example.
func prepareCatalog() *catalog.Catalog {
	return catalog.New(fstest.MapFS{
		"postgres/spinup.yaml": &fstest.MapFile{Data: []byte(`name: postgres
description: PostgreSQL 16 with pgAdmin
category: database
primary: postgres
gui:
  service: pgadmin
  url: http://localhost:${PGADMIN_PORT}
  login: ${PGADMIN_EMAIL} / ${PGADMIN_PASSWORD}
url: postgres://${POSTGRES_USER}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}
ports:
  - name: POSTGRES_PORT
    default: 5432
  - name: PGADMIN_PORT
    default: 8080
profiles: [gui]
`)},
		"postgres/compose.yaml":     &fstest.MapFile{Data: []byte("services: {}\n")},
		"postgres/README.md":        &fstest.MapFile{Data: []byte("# postgres\n")},
		"postgres/init/01-seed.sql": &fstest.MapFile{Data: []byte("select 1;\n")},
		"postgres/.env.example": &fstest.MapFile{Data: []byte(
			"POSTGRES_PORT=5432\nPOSTGRES_USER=spinup\nPOSTGRES_DB=spinup\nPGADMIN_PORT=8080\n" +
				"PGADMIN_EMAIL=admin@example.com\nPGADMIN_PASSWORD=spinup\n")},
	})
}

func prepareCtx(t *testing.T) (context.Context, config.Paths) {
	t.Helper()

	home := t.TempDir()
	t.Setenv(config.HomeEnv, home)

	return catalog.NewContext(context.Background(), prepareCatalog()), config.At(home)
}

// prepare is what every lifecycle command runs before touching Docker: it
// writes the stack out, seeds its env file and resolves the environment.
func TestPrepare(t *testing.T) {
	ctx, paths := prepareCtx(t)

	p, err := prepare(ctx, "postgres", profileFlags{})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	// The stack is on disk, subdirectory and dotfile included.
	for _, f := range []string{"compose.yaml", "spinup.yaml", ".env.example", filepath.Join("init", "01-seed.sql")} {
		if _, err := os.Stat(filepath.Join(paths.StackDir("postgres"), f)); err != nil {
			t.Errorf("%s was not materialised: %v", f, err)
		}
	}

	// The env file is seeded from .env.example.
	data, err := os.ReadFile(paths.EnvFile("postgres"))
	if err != nil {
		t.Fatalf("env file: %v", err)
	}
	if !strings.Contains(string(data), "POSTGRES_PORT=5432") {
		t.Errorf("env file was not seeded:\n%s", data)
	}

	if p.project.Name() != "spinup-postgres" {
		t.Errorf("project = %q", p.project.Name())
	}
	if p.project.Dir != paths.StackDir("postgres") {
		t.Errorf("project dir = %q", p.project.Dir)
	}
	if p.project.EnvFile != paths.EnvFile("postgres") {
		t.Errorf("project env file = %q", p.project.EnvFile)
	}
	if got := p.env["POSTGRES_USER"]; got != "spinup" {
		t.Errorf("resolved env is missing the .env.example values: %v", p.env)
	}
}

// Running a command twice must not undo an edit — the whole reason the files
// are written out is so they can be changed.
func TestPrepareKeepsEdits(t *testing.T) {
	ctx, paths := prepareCtx(t)

	if _, err := prepare(ctx, "postgres", profileFlags{}); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	edited := filepath.Join(paths.StackDir("postgres"), "compose.yaml")
	const mine = "services:\n  postgres:\n    image: postgres:17\n"
	if err := os.WriteFile(edited, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.EnvFile("postgres"), []byte("POSTGRES_PORT=5555\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	p, err := prepare(ctx, "postgres", profileFlags{})
	if err != nil {
		t.Fatalf("second prepare: %v", err)
	}

	got, err := os.ReadFile(edited)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != mine {
		t.Errorf("compose.yaml was overwritten:\n%s", got)
	}
	if p.env["POSTGRES_PORT"] != "5555" {
		t.Errorf("the edited port was not picked up: %v", p.env["POSTGRES_PORT"])
	}
}

func TestPrepareUnknownStack(t *testing.T) {
	ctx, _ := prepareCtx(t)

	_, err := prepare(ctx, "nope", profileFlags{})
	if err == nil {
		t.Fatal("want an error")
	}
	if got := codeFor(err); got != ExitNotFound {
		t.Errorf("exit code = %d, want %d", got, ExitNotFound)
	}
}

func TestPrepareProfiles(t *testing.T) {
	ctx, _ := prepareCtx(t)

	p, err := prepare(ctx, "postgres", profileFlags{})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if !slices.Equal(p.project.Profiles, []string{"gui"}) {
		t.Errorf("profiles = %v, want [gui] by default", p.project.Profiles)
	}

	p, err = prepare(ctx, "postgres", profileFlags{noGUI: true})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if len(p.project.Profiles) != 0 {
		t.Errorf("profiles = %v, want none with --no-gui", p.project.Profiles)
	}
}

// The summary is what the user reads after `up`: the address of the thing they
// just started, with the ports and credentials filled in.
func TestSummarise(t *testing.T) {
	ui.SetColor(false)
	t.Cleanup(func() { ui.SetColor(true) })

	ctx, _ := prepareCtx(t)
	p, err := prepare(ctx, "postgres", profileFlags{})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	var out bytes.Buffer
	summarise(&out, p)

	for _, want := range []string{
		"postgres://spinup@localhost:5432/spinup", // ${VAR} resolved from the env
		"http://localhost:8080",
		"admin@example.com / spinup",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("summary is missing %q:\n%s", want, out.String())
		}
	}
}

// A compose failure is exit 4, so a script can tell it from a stack that does
// not exist (3) or Docker being absent (2).
func TestRunComposeExitCode(t *testing.T) {
	err := runCompose(&compose.Error{Args: []string{"compose", "up"}, ExitCode: 1, Stderr: "boom"})
	if got := codeFor(err); got != ExitCompose {
		t.Errorf("exit code = %d, want %d", got, ExitCompose)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("compose's own message was dropped: %v", err)
	}

	// Anything that is not a compose failure keeps its own code.
	other := errors.New("something else")
	if got := runCompose(other); !errors.Is(got, other) {
		t.Errorf("runCompose changed an unrelated error: %v", got)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "code --wait", "vi"); got != "code --wait" {
		t.Errorf("firstNonEmpty = %q", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Errorf("firstNonEmpty = %q, want empty", got)
	}
}
