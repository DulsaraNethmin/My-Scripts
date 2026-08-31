package compose_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/DulsaraNethmin/spinup/internal/compose"
)

func project() compose.Project {
	return compose.Project{
		Stack:   "postgres",
		Dir:     filepath.Join("/home", "u", ".spinup", "stacks", "postgres"),
		EnvFile: filepath.Join("/home", "u", ".spinup", "env", "postgres.env"),
	}
}

// The project name is what keeps a spinup stack from colliding with a project
// the user made themselves, and what makes `docker ps` legible.
func TestProjectName(t *testing.T) {
	if got := project().Name(); got != "spinup-postgres" {
		t.Errorf("Name = %q, want spinup-postgres", got)
	}
}

func TestArgs(t *testing.T) {
	got := project().Args("up", "--detach")

	want := []string{
		"compose",
		"--project-name", "spinup-postgres",
		"--file", filepath.Join("/home", "u", ".spinup", "stacks", "postgres", "compose.yaml"),
		"--env-file", filepath.Join("/home", "u", ".spinup", "env", "postgres.env"),
		"up", "--detach",
	}
	if !slices.Equal(got, want) {
		t.Errorf("Args =\n%v\nwant\n%v", got, want)
	}
}

func TestArgsProfiles(t *testing.T) {
	p := project()
	p.Profiles = []string{"gui", "gpu"}

	got := strings.Join(p.Args("up"), " ")
	if !strings.Contains(got, "--profile gui --profile gpu") {
		t.Errorf("profiles are not passed through: %s", got)
	}
}

// Without an env file compose still has the inline defaults in compose.yaml,
// so an empty EnvFile must not produce a dangling flag.
func TestArgsWithoutEnvFile(t *testing.T) {
	p := project()
	p.EnvFile = ""

	for _, arg := range p.Args("ps") {
		if arg == "--env-file" {
			t.Errorf("--env-file was passed with no file: %v", p.Args("ps"))
		}
	}
}

func TestUpArgs(t *testing.T) {
	opts := compose.UpOptions{
		Wait: true, Timeout: 90 * time.Second, Build: true, Services: []string{"postgres"},
	}

	got := strings.Join(opts.Args(), " ")
	for _, want := range []string{"up --detach", "--wait", "--wait-timeout 90", "--build", "postgres"} {
		if !strings.Contains(got, want) {
			t.Errorf("Up args %q are missing %q", got, want)
		}
	}
}

// down keeps data and destroy does not: the difference is one flag, and
// getting it backwards is the bug the old mysql-stop.sh shipped with.
func TestDownVolumes(t *testing.T) {
	keep := compose.DownOptions{}.Args()
	if slices.Contains(keep, "--volumes") {
		t.Errorf("plain down passes --volumes, which would delete the user's data: %v", keep)
	}

	destroy := compose.DownOptions{Volumes: true}.Args()
	if !slices.Contains(destroy, "--volumes") {
		t.Errorf("down --volumes did not pass it: %v", destroy)
	}
}

// exec is how `spin shell` and `spin cli` reach a container. -T is the
// flag that matters: compose asks for a TTY by default and fails outright
// without one, which is every scripted invocation.
func TestExecArgs(t *testing.T) {
	got := strings.Join(compose.ExecOptions{
		Service: "postgres",
		Command: []string{"psql", "-U", "spinup"},
		NoTTY:   true,
		User:    "root",
		Env:     []string{"PAGER=cat"},
	}.Args(), " ")

	for _, want := range []string{"exec", "--no-TTY", "--user root", "--env PAGER=cat", "postgres psql -U spinup"} {
		if !strings.Contains(got, want) {
			t.Errorf("Exec args %q are missing %q", got, want)
		}
	}

	// The service comes last, before the command: anything after it is the
	// command's own, and a flag that lands there is passed to the wrong program.
	plain := compose.ExecOptions{Service: "redis", Command: []string{"redis-cli", "ping"}}.Args()
	if want := []string{"exec", "redis", "redis-cli", "ping"}; !slices.Equal(plain, want) {
		t.Errorf("Exec args = %v, want %v", plain, want)
	}
}

func TestLogsArgs(t *testing.T) {
	opts := compose.LogsOptions{Follow: true, Tail: 100, Services: []string{"postgres"}}

	got := strings.Join(opts.Args(), " ")
	for _, want := range []string{"logs", "--follow", "--tail 100", "postgres"} {
		if !strings.Contains(got, want) {
			t.Errorf("Logs args %q are missing %q", got, want)
		}
	}
}

// os/exec copies a command's stdout and stderr on two goroutines. Handing the
// runner one writer for both — what any caller collecting the whole output into
// a single buffer does, the CLI's own integration test included — must not put
// two goroutines into that writer at once. Under -race this is the test that
// fails if stream() stops serializing them.
func TestStreamSharesOneWriterSafely(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker is a shell script")
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-docker")
	script := "#!/bin/sh\ni=0\nwhile [ $i -lt 300 ]; do echo stdout; echo stderr >&2; i=$((i+1)); done\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	r := compose.New(&out, &out)
	r.Bin = bin

	p := project()
	p.Dir = dir

	if err := r.Up(t.Context(), p, compose.UpOptions{}); err != nil {
		t.Fatalf("Up: %v", err)
	}
	if want := 600; bytes.Count(out.Bytes(), []byte("\n")) != want {
		t.Errorf("collected %d lines, want %d", bytes.Count(out.Bytes(), []byte("\n")), want)
	}
}

// A failed compose run has to carry its exit code and compose's own stderr:
// that message is the only thing that tells the user what went wrong.
func TestRunErrorCarriesExitCodeAndStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker is a shell script")
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-docker")
	script := "#!/bin/sh\necho 'no configuration file provided' >&2\nexit 14\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	r := compose.New(nil, nil)
	r.Bin = bin

	// The project directory has to exist, or the run fails before the fake
	// docker gets to say anything.
	p := project()
	p.Dir = dir

	err := r.Up(t.Context(), p, compose.UpOptions{})
	if err == nil {
		t.Fatal("want an error")
	}

	var cerr *compose.Error
	if !errors.As(err, &cerr) {
		t.Fatalf("error is %T, want *compose.Error", err)
	}
	if cerr.ExitCode != 14 {
		t.Errorf("ExitCode = %d, want 14", cerr.ExitCode)
	}
	if !strings.Contains(cerr.Error(), "no configuration file provided") {
		t.Errorf("the error dropped compose's message: %v", cerr)
	}
}

// The stack directory is the compose project directory, which is what makes
// relative paths in compose.yaml — nginx-static's build context and its site
// bind mount — resolve the way they do when run by hand.
func TestRunsInTheStackDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake docker is a shell script")
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-docker")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\npwd\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	stackDir := filepath.Join(dir, "stacks", "nginx-static")
	if err := os.MkdirAll(stackDir, 0o755); err != nil {
		t.Fatal(err)
	}

	r := compose.New(nil, nil)
	r.Bin = bin

	out, err := r.Output(t.Context(), compose.Project{Stack: "nginx-static", Dir: stackDir}, "config")
	if err != nil {
		t.Fatalf("Output: %v", err)
	}

	got, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(stackDir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("compose ran in %q, want %q", got, want)
	}
}
