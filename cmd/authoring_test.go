package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/config"
)

// authoringCtx is the fixture catalog layered the way the real one is: the
// user's ~/.spinup/stacks over the built-in stacks. new and reset both turn on
// which layer a stack came from, so a builtin-only catalog would not exercise
// them.
func authoringCtx(t *testing.T) (context.Context, config.Paths) {
	t.Helper()

	home := t.TempDir()
	t.Setenv(config.HomeEnv, home)

	paths := config.At(home)
	if err := os.MkdirAll(paths.StacksDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	cat := prepareCatalog().WithUserStacks(os.DirFS(paths.StacksDir()))
	return catalog.NewContext(context.Background(), cat), paths
}

func execute(t *testing.T, c *cobra.Command, ctx context.Context, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetIn(strings.NewReader(""))
	c.SetArgs(args)

	err := c.ExecuteContext(ctx)
	return out.String(), err
}

// A scaffolded stack has to be a stack: four files, and a spinup.yaml the
// catalog accepts. A template that has to be fixed before `spinup up` works is
// not much of a starting point.
func TestNewScaffoldsAValidStack(t *testing.T) {
	ctx, paths := authoringCtx(t)

	out, err := execute(t, newNewCmd(), ctx, "my-thing")
	if err != nil {
		t.Fatalf("new: %v\n%s", err, out)
	}

	dir := paths.StackDir("my-thing")
	for _, f := range catalog.RequiredFiles {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("new did not write %s: %v", f, err)
		}
	}

	// Loading it through a catalog is the same validation `spinup up` does.
	s, err := catalog.New(os.DirFS(paths.StacksDir())).Load("my-thing")
	if err != nil {
		t.Fatalf("the scaffolded stack does not load: %v", err)
	}
	if s.Name != "my-thing" {
		t.Errorf("name = %q, want my-thing", s.Name)
	}
	if len(s.Ports) == 0 || s.Ports[0].Name != "MY_THING_PORT" {
		t.Errorf("ports = %v, want MY_THING_PORT", s.Ports)
	}
}

func TestNewRefusesNamesItCannotUse(t *testing.T) {
	for name, args := range map[string][]string{
		"an existing stack": {"postgres"},
		"not kebab-case":    {"My Thing"},
		"a leading dash":    {"-thing"},
		"a path":            {"../escape"},
	} {
		ctx, _ := authoringCtx(t)

		if out, err := execute(t, newNewCmd(), ctx, args...); err == nil {
			t.Errorf("%s: new %q succeeded\n%s", name, args, out)
		}
	}
}

// --from is how you keep both a built-in stack and a variant of it. The copy's
// spinup.yaml has to carry the new name, or the catalog rejects it for
// disagreeing with its directory.
func TestNewFromAnExistingStack(t *testing.T) {
	ctx, paths := authoringCtx(t)

	out, err := execute(t, newNewCmd(), ctx, "pg-copy", "--from", "postgres")
	if err != nil {
		t.Fatalf("new --from: %v\n%s", err, out)
	}

	s, err := catalog.New(os.DirFS(paths.StacksDir())).Load("pg-copy")
	if err != nil {
		t.Fatalf("the copy does not load: %v", err)
	}
	if s.Name != "pg-copy" {
		t.Errorf("name = %q, want pg-copy", s.Name)
	}
	if s.Description != "PostgreSQL 16 with pgAdmin" {
		t.Errorf("the copy lost the original's description: %q", s.Description)
	}
	if !strings.Contains(out, "same ports") {
		t.Errorf("new --from does not warn that the ports collide:\n%s", out)
	}
}

func TestNewFromAStackThatDoesNotExist(t *testing.T) {
	ctx, _ := authoringCtx(t)

	if _, err := execute(t, newNewCmd(), ctx, "copy", "--from", "nosuchstack"); err == nil {
		t.Fatal("new --from an unknown stack succeeded")
	}
}

// reset is the way out of an edit that broke a stack. What it must not do is
// take the env file — the ports and passwords — with it.
func TestResetRestoresTheBuiltinAndKeepsTheEnvFile(t *testing.T) {
	ctx, paths := authoringCtx(t)

	if _, err := execute(t, newEnvCmd(), ctx, "postgres"); err != nil {
		t.Fatalf("env: %v", err)
	}

	compose := filepath.Join(paths.StackDir("postgres"), "compose.yaml")
	if err := os.WriteFile(compose, []byte("this is not yaml at all\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	envFile := paths.EnvFile("postgres")
	if err := os.WriteFile(envFile, []byte("POSTGRES_PORT=15432\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := execute(t, newResetCmd(), ctx, "postgres", "-y")
	if err != nil {
		t.Fatalf("reset: %v\n%s", err, out)
	}

	data, err := os.ReadFile(compose)
	if err != nil {
		t.Fatalf("reset did not write the stack back: %v", err)
	}
	if strings.Contains(string(data), "not yaml") {
		t.Error("reset kept the broken compose.yaml")
	}

	env, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("reset removed the env file: %v", err)
	}
	if !strings.Contains(string(env), "15432") {
		t.Errorf("reset threw away the edited port: %q", env)
	}
}

func TestResetWithEnvRestoresTheEnvFileToo(t *testing.T) {
	ctx, paths := authoringCtx(t)

	if _, err := execute(t, newEnvCmd(), ctx, "postgres"); err != nil {
		t.Fatalf("env: %v", err)
	}
	envFile := paths.EnvFile("postgres")
	if err := os.WriteFile(envFile, []byte("POSTGRES_PORT=15432\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if out, err := execute(t, newResetCmd(), ctx, "postgres", "-y", "--env"); err != nil {
		t.Fatalf("reset --env: %v\n%s", err, out)
	}

	env, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("env file: %v", err)
	}
	if strings.Contains(string(env), "15432") {
		t.Errorf("reset --env kept the edited port: %q", env)
	}
	if !strings.Contains(string(env), "POSTGRES_PORT=5432") {
		t.Errorf("reset --env did not restore the default: %q", env)
	}
}

// Resetting a stack of the user's own would not restore it, it would delete
// it: there is no other copy anywhere.
func TestResetRefusesAStackOfYourOwn(t *testing.T) {
	ctx, paths := authoringCtx(t)

	if out, err := execute(t, newNewCmd(), ctx, "mine"); err != nil {
		t.Fatalf("new: %v\n%s", err, out)
	}

	out, err := execute(t, newResetCmd(), ctx, "mine", "-y")
	if err == nil {
		t.Fatalf("reset deleted a stack of the user's own:\n%s", out)
	}
	if !strings.Contains(err.Error(), "your own stack") {
		t.Errorf("the error does not explain why: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(paths.StackDir("mine"), "spinup.yaml")); statErr != nil {
		t.Errorf("reset removed the stack anyway: %v", statErr)
	}
}

// Completion is what the `completion` command is for. Offering a name that is
// already on the line, or one that does not match what has been typed, is
// worse than offering nothing.
func TestCompleteStacks(t *testing.T) {
	ctx, _ := authoringCtx(t)

	c := newUpCmd()
	c.SetContext(ctx)

	got, directive := completeStacks(c, nil, "")
	if !slices.Contains(got, "postgres") {
		t.Errorf("completion = %v, want it to include postgres", got)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("directive = %v, want NoFileComp — stack names are not paths", directive)
	}

	if got, _ := completeStacks(c, nil, "post"); !slices.Equal(got, []string{"postgres"}) {
		t.Errorf("completion of %q = %v, want [postgres]", "post", got)
	}
	if got, _ := completeStacks(c, nil, "zzz"); len(got) != 0 {
		t.Errorf("completion of a prefix nothing matches = %v, want nothing", got)
	}
	if got, _ := completeStacks(c, []string{"postgres"}, ""); slices.Contains(got, "postgres") {
		t.Errorf("completion offered a stack already on the line: %v", got)
	}

	// One-stack commands stop offering names once they have one.
	if got, _ := completeOneStack(c, []string{"postgres"}, ""); len(got) != 0 {
		t.Errorf("completion after the one stack a command takes = %v, want nothing", got)
	}
}
