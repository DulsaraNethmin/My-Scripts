// Package docker inspects the local Docker installation: whether the CLI is
// there, whether the daemon answers, and whether Compose v2 is available.
//
// It shells out to the docker CLI rather than using the Docker SDK, for the
// same reason the rest of spinup does — what spinup sees is exactly what the
// user would see typing the command themselves.
package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The failures a command needs to tell apart. All three mean exit code 2.
var (
	ErrNotInstalled     = errors.New("docker is not installed or not on PATH")
	ErrDaemonNotRunning = errors.New("the docker daemon is not running")
	ErrComposeMissing   = errors.New("docker compose v2 is not available")
)

// ComposeMajor is the Compose major version spinup requires. v1 (the
// `docker-compose` python script) reached end of life in 2023 and does not
// support the profiles and healthcheck conditions every stack uses.
const ComposeMajor = 2

// DefaultTimeout bounds a docker call that would otherwise hang: a wedged
// daemon should fail spinup, not freeze it.
const DefaultTimeout = 20 * time.Second

// Runner runs a command and returns its output. Tests replace it; nothing else
// needs to.
type Runner interface {
	Output(ctx context.Context, name string, args ...string) ([]byte, error)
}

// Client inspects a Docker installation.
type Client struct {
	Bin     string
	Timeout time.Duration
	runner  Runner
}

// New returns a client that runs the docker CLI found on PATH.
func New() *Client {
	return &Client{Bin: "docker", Timeout: DefaultTimeout, runner: execRunner{}}
}

// NewWith returns a client backed by r, for tests.
func NewWith(r Runner) *Client {
	return &Client{Bin: "docker", Timeout: DefaultTimeout, runner: r}
}

type execRunner struct{}

func (execRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// docker reports why it failed on stderr, and that message is the
		// whole value of the error to a user.
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return stdout.Bytes(), fmt.Errorf("%s: %s", err, msg)
		}
		return stdout.Bytes(), err
	}
	return stdout.Bytes(), nil
}

func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	if _, ok := ctx.Deadline(); !ok && c.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
		defer cancel()
	}

	out, err := c.runner.Output(ctx, c.Bin, args...)
	return strings.TrimSpace(string(out)), err
}

// Installed reports whether the docker CLI is on PATH.
func (c *Client) Installed() error {
	if _, err := exec.LookPath(c.Bin); err != nil {
		return ErrNotInstalled
	}
	return nil
}

// ClientVersion returns the version of the docker CLI.
func (c *Client) ClientVersion(ctx context.Context) (string, error) {
	v, err := c.run(ctx, "version", "--format", "{{.Client.Version}}")
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrNotInstalled, err)
	}
	return v, nil
}

// ServerVersion returns the daemon's version. It is the daemon check: the
// docker CLI is happy to exist without one to talk to.
func (c *Client) ServerVersion(ctx context.Context) (string, error) {
	v, err := c.run(ctx, "version", "--format", "{{.Server.Version}}")
	if err != nil || v == "" {
		return "", fmt.Errorf("%w: %w", ErrDaemonNotRunning, err)
	}
	return v, nil
}

// ComposeVersion returns the version of the Compose plugin, and fails if it is
// missing or is v1.
func (c *Client) ComposeVersion(ctx context.Context) (string, error) {
	v, err := c.run(ctx, "compose", "version", "--short")
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrComposeMissing, err)
	}

	major, err := majorVersion(v)
	if err != nil {
		return v, fmt.Errorf("%w: cannot read the version %q: %w", ErrComposeMissing, v, err)
	}
	if major < ComposeMajor {
		return v, fmt.Errorf("%w: found v%s, which is end of life; install the compose plugin (docker compose, not docker-compose)",
			ErrComposeMissing, v)
	}
	return v, nil
}

// SupportsWaitTimeout reports whether `docker compose up` has --wait-timeout,
// which `spinup up` passes on every run. Asking the CLI rather than comparing
// version numbers: the flag arrived in a v2 minor release, and a wrong constant
// here would be a check that lies in both directions.
func (c *Client) SupportsWaitTimeout(ctx context.Context) bool {
	out, err := c.run(ctx, "compose", "up", "--help")
	if err != nil {
		return true // if it cannot be asked, do not invent a problem
	}
	return strings.Contains(out, "--wait-timeout")
}

// Runtimes returns the container runtimes the daemon knows about. The one that
// matters to spinup is nvidia: without it, the pytorch stack's gpu profile
// starts a container that cannot see the card.
func (c *Client) Runtimes(ctx context.Context) ([]string, error) {
	out, err := c.run(ctx, "info", "--format", "{{json .Runtimes}}")
	if err != nil {
		return nil, err
	}

	// The value is a map of name -> runtime object; only the names matter.
	var runtimes map[string]any
	if err := json.Unmarshal([]byte(out), &runtimes); err != nil {
		return nil, fmt.Errorf("reading docker runtimes: %w", err)
	}

	names := make([]string, 0, len(runtimes))
	for name := range runtimes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// HasNVIDIARuntime reports whether the daemon can run GPU containers.
func (c *Client) HasNVIDIARuntime(ctx context.Context) (bool, error) {
	runtimes, err := c.Runtimes(ctx)
	if err != nil {
		return false, err
	}
	for _, r := range runtimes {
		if strings.Contains(r, "nvidia") {
			return true, nil
		}
	}
	return false, nil
}

// HasNVIDIADriver reports whether the machine has an NVIDIA driver installed,
// which is what tells "no GPU here" apart from "a GPU docker cannot use".
func HasNVIDIADriver() bool {
	_, err := exec.LookPath("nvidia-smi")
	return err == nil
}

// majorVersion reads the leading number of a version string, which may be
// prefixed with v and suffixed with anything: "2.40.2-desktop.1" is 2.
func majorVersion(v string) (int, error) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	head, _, _ := strings.Cut(v, ".")
	return strconv.Atoi(head)
}

// Available reports whether spinup can run: docker installed, daemon up, and
// Compose v2 present. Every command that talks to Docker starts here, so the
// failure is one clear message rather than a compose error the user has to
// decode.
func (c *Client) Available(ctx context.Context) error {
	if err := c.Installed(); err != nil {
		return err
	}
	if _, err := c.ServerVersion(ctx); err != nil {
		return err
	}
	if _, err := c.ComposeVersion(ctx); err != nil {
		return err
	}
	return nil
}
