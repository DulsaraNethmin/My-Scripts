package compose

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// UpOptions are the knobs `spinup up` turns.
type UpOptions struct {
	// Wait blocks until every service with a healthcheck reports healthy,
	// which is what lets spinup print a connection string that works.
	Wait    bool
	Timeout time.Duration

	// Build rebuilds images that are built from a Dockerfile (nginx-static).
	Build bool

	// Pull fetches newer images before starting.
	Pull bool

	// Services limits the run to some of the stack's services.
	Services []string

	// Extra is passed straight to compose, so a power user is never blocked.
	Extra []string
}

// Args are the compose arguments these options produce.
func (o UpOptions) Args() []string {
	args := []string{"up", "--detach"}

	if o.Wait {
		args = append(args, "--wait")
		if o.Timeout > 0 {
			args = append(args, "--wait-timeout", strconv.Itoa(int(o.Timeout.Seconds())))
		}
	}
	if o.Build {
		args = append(args, "--build")
	}
	if o.Pull {
		args = append(args, "--pull", "always")
	}

	args = append(args, o.Extra...)
	return append(args, o.Services...)
}

// Up starts the stack in the background.
func (r *Runner) Up(ctx context.Context, p Project, opts UpOptions) error {
	return r.Run(ctx, p, opts.Args()...)
}

// DownOptions are the knobs `spinup down` and `spinup destroy` turn.
type DownOptions struct {
	// Volumes deletes the stack's data. Only `spinup destroy` sets it, and
	// only after asking.
	Volumes bool

	// RemoveOrphans cleans up containers left by a service that has since been
	// removed from the compose file.
	RemoveOrphans bool

	Timeout time.Duration
	Extra   []string
}

// Args are the compose arguments these options produce.
func (o DownOptions) Args() []string {
	args := []string{"down"}

	if o.Volumes {
		args = append(args, "--volumes")
	}
	if o.RemoveOrphans {
		args = append(args, "--remove-orphans")
	}
	if o.Timeout > 0 {
		args = append(args, "--timeout", strconv.Itoa(int(o.Timeout.Seconds())))
	}
	return append(args, o.Extra...)
}

// Down stops the stack. Without Volumes the data survives, which is the whole
// difference between `spinup down` and `spinup destroy`.
func (r *Runner) Down(ctx context.Context, p Project, opts DownOptions) error {
	return r.Run(ctx, p, opts.Args()...)
}

// Restart restarts the stack, or just the named services.
func (r *Runner) Restart(ctx context.Context, p Project, services ...string) error {
	return r.Run(ctx, p, append([]string{"restart"}, services...)...)
}

// LogsOptions are the knobs `spinup logs` turns.
type LogsOptions struct {
	Follow     bool
	Tail       int // 0 means compose's default
	Timestamps bool
	Services   []string
}

// Args are the compose arguments these options produce.
func (o LogsOptions) Args() []string {
	args := []string{"logs"}

	if o.Follow {
		args = append(args, "--follow")
	}
	if o.Tail > 0 {
		args = append(args, "--tail", strconv.Itoa(o.Tail))
	}
	if o.Timestamps {
		args = append(args, "--timestamps")
	}
	return append(args, o.Services...)
}

// Logs streams container logs.
func (r *Runner) Logs(ctx context.Context, p Project, opts LogsOptions) error {
	return r.Run(ctx, p, opts.Args()...)
}

// Config returns the stack's fully resolved compose file, which is also the
// cheapest way to prove a stack is valid without starting anything.
func (r *Runner) Config(ctx context.Context, p Project, args ...string) ([]byte, error) {
	return r.Output(ctx, p, append([]string{"config"}, args...)...)
}

// Publisher is a published port of a container.
type Publisher struct {
	URL           string `json:"URL"`
	TargetPort    int    `json:"TargetPort"`
	PublishedPort int    `json:"PublishedPort"`
	Protocol      string `json:"Protocol"`
}

// Container is one service's container, as compose reports it.
type Container struct {
	Name       string      `json:"Name"`
	Service    string      `json:"Service"`
	Image      string      `json:"Image"`
	State      string      `json:"State"`
	Status     string      `json:"Status"`
	Health     string      `json:"Health"`
	ExitCode   int         `json:"ExitCode"`
	Publishers []Publisher `json:"Publishers"`
}

// Running reports whether the container is up.
func (c Container) Running() bool { return c.State == "running" }

// Healthy reports whether the container is up and, if it has a healthcheck,
// passing it. A container without a healthcheck counts as healthy once it is
// running — that is all the information there is.
func (c Container) Healthy() bool {
	if !c.Running() {
		return false
	}
	return c.Health == "" || c.Health == "healthy"
}

// PS lists the stack's containers.
func (r *Runner) PS(ctx context.Context, p Project) ([]Container, error) {
	out, err := r.Output(ctx, p, "ps", "--all", "--format", "json")
	if err != nil {
		return nil, err
	}
	return ParsePS(out)
}

// ParsePS reads `compose ps --format json`, which is a JSON array in some
// versions of Compose and one object per line in others. Both have shipped in
// v2, so both are handled.
func ParsePS(out []byte) ([]Container, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, nil
	}

	if trimmed[0] == '[' {
		var containers []Container
		if err := json.Unmarshal(trimmed, &containers); err != nil {
			return nil, fmt.Errorf("reading compose ps: %w", err)
		}
		return containers, nil
	}

	var containers []Container
	for _, line := range strings.Split(string(trimmed), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var c Container
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, fmt.Errorf("reading compose ps: %w", err)
		}
		containers = append(containers, c)
	}
	return containers, nil
}

// ProjectSummary is one compose project as `docker compose ls` reports it.
type ProjectSummary struct {
	Name        string `json:"Name"`
	Status      string `json:"Status"`
	ConfigFiles string `json:"ConfigFiles"`
}

// Stack returns the spinup stack this project belongs to, and whether it is
// one of spinup's at all — the user's own compose projects are none of our
// business.
func (p ProjectSummary) Stack() (string, bool) {
	name, ok := strings.CutPrefix(p.Name, ProjectPrefix)
	return name, ok
}

// Running reports whether any of the project's containers are up. compose
// reports a status like "running(2)" or "exited(1)", and both can appear at
// once while a stack is coming up.
func (p ProjectSummary) Running() bool {
	return strings.Contains(p.Status, "running")
}

// ListProjects returns every compose project on the machine, spinup's and the
// user's alike. One call answers "what is running?" for the whole catalog,
// which is what `spinup list` needs.
func (r *Runner) ListProjects(ctx context.Context) ([]ProjectSummary, error) {
	out, err := r.capture(ctx, "", []string{"compose", "ls", "--all", "--format", "json"})
	if err != nil {
		return nil, err
	}
	return parseProjects(out)
}

func parseProjects(out []byte) ([]ProjectSummary, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, nil
	}

	var projects []ProjectSummary
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &projects); err != nil {
			return nil, fmt.Errorf("reading compose ls: %w", err)
		}
		return projects, nil
	}

	for _, line := range strings.Split(string(trimmed), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var p ProjectSummary
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			return nil, fmt.Errorf("reading compose ls: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, nil
}
