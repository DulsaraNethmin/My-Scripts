// Package compose wraps the `docker compose` CLI.
//
// spinup shells out rather than using the Docker SDK, so a user gets exactly
// the behaviour they would get running compose by hand, and every stack stays
// a plain compose.yaml that works without spinup installed. The wrapper's job
// is to always pass the same project name, compose file and env file, so a
// stack cannot collide with a user's own projects.
package compose

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProjectPrefix namespaces every compose project spinup creates, so `docker
// ps` and Docker Desktop show where a container came from and spinup never
// touches a project the user made themselves.
const ProjectPrefix = "spinup-"

// ComposeFile is the file every stack ships.
const ComposeFile = "compose.yaml"

// Project identifies one stack's compose invocation.
type Project struct {
	// Stack is the stack name; the compose project is ProjectPrefix + Stack.
	Stack string

	// Dir is the stack's directory — normally ~/.spinup/stacks/<name>. compose
	// resolves relative paths in compose.yaml against it, which stacks like
	// nginx-static rely on for their build context and bind mounts.
	Dir string

	// EnvFile is the stack's env file. Optional: without one, compose falls
	// back to the inline defaults in compose.yaml.
	EnvFile string

	// Profiles are the compose profiles to enable.
	Profiles []string
}

// Name is the compose project name.
func (p Project) Name() string { return ProjectPrefix + p.Stack }

// File is the path to the stack's compose file.
func (p Project) File() string { return filepath.Join(p.Dir, ComposeFile) }

// Args builds the full docker argv for a compose subcommand: the global flags
// that make this project this project, then the subcommand.
func (p Project) Args(args ...string) []string {
	out := []string{"compose", "--project-name", p.Name(), "--file", p.File()}

	if p.EnvFile != "" {
		out = append(out, "--env-file", p.EnvFile)
	}
	for _, profile := range p.Profiles {
		out = append(out, "--profile", profile)
	}
	return append(out, args...)
}

// Error is a failed `docker compose` run. Commands map it to exit code 4.
type Error struct {
	Args     []string
	ExitCode int
	Stderr   string
	err      error
}

func (e *Error) Error() string {
	msg := fmt.Sprintf("docker %s failed", strings.Join(e.Args, " "))
	if e.ExitCode > 0 {
		msg = fmt.Sprintf("%s (exit %d)", msg, e.ExitCode)
	}
	switch {
	case e.Stderr != "":
		msg = fmt.Sprintf("%s: %s", msg, e.Stderr)
	case e.err != nil:
		// compose printed nothing, so the failure is about starting it at all
		// — a missing binary, or a stack directory that is not there.
		msg = fmt.Sprintf("%s: %s", msg, e.err)
	}
	return msg
}

func (e *Error) Unwrap() error { return e.err }

// Runner runs compose commands for a project.
type Runner struct {
	// Bin is the docker executable; empty means "docker" on PATH.
	Bin string

	// Stdout and Stderr receive compose's output as it arrives. Leaving them
	// nil discards it, which is what the JSON-parsing calls want.
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader

	// Env is added to the environment compose runs with. Compose gives the
	// environment precedence over --env-file, so this is how a one-off
	// override like `--port` reaches it.
	Env []string
}

// New returns a runner that streams compose's output to the given writers.
func New(stdout, stderr io.Writer) *Runner {
	return &Runner{Stdout: stdout, Stderr: stderr}
}

func (r *Runner) bin() string {
	if r.Bin == "" {
		return "docker"
	}
	return r.Bin
}

// Run runs a compose subcommand for a project, streaming its output.
func (r *Runner) Run(ctx context.Context, p Project, args ...string) error {
	return r.stream(ctx, p.Dir, p.Args(args...))
}

// Output runs a compose subcommand for a project and returns its stdout, for
// the commands that parse rather than display it.
func (r *Runner) Output(ctx context.Context, p Project, args ...string) ([]byte, error) {
	return r.capture(ctx, p.Dir, p.Args(args...))
}

// stream runs docker with its output going to the user as it arrives.
func (r *Runner) stream(ctx context.Context, dir string, args []string) error {
	cmd := r.command(ctx, dir, args)
	cmd.Stdout = r.Stdout
	cmd.Stdin = r.Stdin

	// compose writes its progress to stderr, so it has to be streamed as well
	// as captured: the user sees it live, and a failure can quote it.
	var captured bytes.Buffer
	if r.Stderr != nil {
		cmd.Stderr = io.MultiWriter(r.Stderr, &captured)
	} else {
		cmd.Stderr = &captured
	}

	if err := cmd.Run(); err != nil {
		return composeError(args, captured.String(), err)
	}
	return nil
}

// capture runs docker and returns its stdout.
func (r *Runner) capture(ctx context.Context, dir string, args []string) ([]byte, error) {
	cmd := r.command(ctx, dir, args)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), composeError(args, stderr.String(), err)
	}
	return stdout.Bytes(), nil
}

func (r *Runner) command(ctx context.Context, dir string, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, r.bin(), args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), r.Env...)
	return cmd
}

func composeError(args []string, stderr string, err error) error {
	e := &Error{Args: args, Stderr: strings.TrimSpace(stderr), err: err}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		e.ExitCode = exitErr.ExitCode()
	}
	return e
}
