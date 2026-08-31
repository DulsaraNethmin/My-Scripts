package cmd

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/compose"
	"github.com/DulsaraNethmin/spinup/internal/docker"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func newUpCmd() *cobra.Command {
	var (
		flags   profileFlags
		build   bool
		pull    bool
		noWait  bool
		ports   []string
		timeout time.Duration
	)

	cmd := &cobra.Command{
		Use:   "up <stack>... [-- docker compose args]",
		Short: "Start one or more stacks",
		Long: "up starts a stack and waits for it to be healthy. It is idempotent and\n" +
			"never removes anything: running it again on a running stack is a no-op.\n\n" +
			"The stack's files are written to ~/.spinup/stacks/<name>/ and its ports and\n" +
			"credentials to ~/.spinup/env/<name>.env, where you can edit them. Neither is\n" +
			"ever overwritten once it exists.",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeStacks,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			if err := requireDocker(ctx); err != nil {
				return err
			}

			names, extra := splitDashArgs(cmd, args)
			if len(names) == 0 {
				return failf(ExitUsage, "which stack? try `spin up postgres`")
			}

			for _, name := range names {
				p, err := prepare(ctx, name, flags)
				if err != nil {
					return err
				}

				overrides, err := portOverrides(p, ports)
				if err != nil {
					return err
				}

				heading(out, name)

				runner := compose.New(out, cmd.ErrOrStderr())
				runner.Env = overrides

				if err := portPreflight(ctx, p, runner); err != nil {
					return err
				}

				err = runner.Up(ctx, p.project, compose.UpOptions{
					Wait:    !noWait,
					Timeout: timeout,
					Build:   build,
					Pull:    pull,
					Extra:   extra,
				})
				if err != nil {
					return runCompose(err)
				}

				summarise(ctx, out, p, runner)
			}
			return nil
		},
	}

	flags.register(cmd)
	cmd.Flags().BoolVar(&build, "build", false, "build images before starting")
	cmd.Flags().BoolVar(&pull, "pull", false, "pull newer images before starting")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "do not wait for the stack to become healthy")
	cmd.Flags().DurationVar(&timeout, "timeout", 3*time.Minute, "how long to wait for healthy")
	cmd.Flags().StringArrayVar(&ports, "port", nil,
		"override a host port for this run: --port POSTGRES_PORT=15432 (repeatable)")

	return cmd
}

// portOverrides turns --port NAME=1234 into environment assignments for this
// run, and applies them to the stack's resolved environment so the summary and
// the connection string show the port compose is actually going to bind.
//
// Compose gives the process environment precedence over --env-file, which is
// what makes a one-off override possible without editing anything. A name the
// stack does not declare is refused rather than passed through: it would be
// accepted silently by compose and the stack would come up on the port the
// user was trying to change.
func portOverrides(p *prepared, flags []string) ([]string, error) {
	if len(flags) == 0 {
		return nil, nil
	}

	valid := p.stack.PortNames()
	env := make([]string, 0, len(flags))

	for _, flag := range flags {
		name, value, ok := strings.Cut(flag, "=")
		if !ok || name == "" || value == "" {
			return nil, failf(ExitUsage, "--port takes NAME=PORT, as in --port %s=15432", firstPortName(valid))
		}
		if !slices.Contains(valid, name) {
			return nil, failf(ExitUsage, "%s has no port called %s — it has %s",
				p.stack.Name, name, strings.Join(valid, ", "))
		}

		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return nil, failf(ExitUsage, "%s=%s is not a port number", name, value)
		}

		p.env[name] = value
		env = append(env, name+"="+value)
	}
	return env, nil
}

func firstPortName(names []string) string {
	if len(names) == 0 {
		return "SERVICE_PORT"
	}
	return names[0]
}

// catalogExpand resolves the ${VAR} references a spinup.yaml value is written
// in, against the stack's resolved environment.
func catalogExpand(value string, p *prepared) string {
	return catalog.Expand(value, p.env)
}

// summarise prints what the user came for: what came up, where it is listening,
// and how to connect to it — the information people otherwise go digging
// through a compose file and a password manager for.
//
// The services line comes from compose rather than from the stack's metadata,
// so it reports what is actually running: a stack started without its GUI does
// not advertise one, and a port that had to be overridden shows the port that
// was bound.
func summarise(ctx context.Context, out io.Writer, p *prepared, runner *compose.Runner) {
	s := p.stack

	// A failure here is not worth failing an otherwise good `up` for: it only
	// costs the services line, and the GUI is then reported from the stack's
	// metadata as it was before compose could be asked.
	containers, psErr := runner.PS(ctx, p.project)

	if services := runningServices(containers); services != "" {
		fmt.Fprintf(out, "  %-9s %s\n", ui.Dim("services"), services)
	}

	url := catalogExpand(s.URL, p)
	if url != "" {
		fmt.Fprintf(out, "  %-9s %s\n", ui.Dim("url"), url)
	}

	// For a stack whose GUI is its primary service the two are the same
	// address, and repeating it back adds nothing. Nor is a GUI advertised
	// when it was not started — `up --no-gui` leaves nothing on that port.
	if s.HasGUI() && (psErr != nil || serviceIsRunning(containers, s.GUI.Service)) {
		if gui := catalogExpand(s.GUI.URL, p); gui != "" && gui != url {
			line := gui
			if login := catalogExpand(s.GUI.Login, p); login != "" && login != "none" {
				line = fmt.Sprintf("%s  (%s)", gui, login)
			}
			fmt.Fprintf(out, "  %-9s %s\n", ui.Dim("gui"), line)
		}
	}
	fmt.Fprintf(out, "  %-9s %s\n", ui.Dim("env"), p.paths.EnvFile(s.Name))
}

// runningServices is "postgres 5432, pgadmin 8080" — each service that came up,
// with the host port it bound. Asking compose costs one more call than reading
// the stack's metadata, and buys the difference between what was asked for and
// what is true.
func runningServices(containers []compose.Container) string {
	var services []string
	for _, c := range containers {
		if !c.Running() {
			continue
		}

		seen := map[int]bool{}
		var ports []string
		for _, pub := range c.Publishers {
			if pub.PublishedPort == 0 || seen[pub.PublishedPort] {
				continue
			}
			seen[pub.PublishedPort] = true
			ports = append(ports, strconv.Itoa(pub.PublishedPort))
		}

		if len(ports) == 0 {
			services = append(services, c.Service)
			continue
		}
		services = append(services, c.Service+" "+strings.Join(ports, "/"))
	}
	return strings.Join(services, ", ")
}

// serviceIsRunning reports whether one of a stack's services has a running
// container.
func serviceIsRunning(containers []compose.Container, service string) bool {
	for _, c := range containers {
		if c.Service == service && c.Running() {
			return true
		}
	}
	return false
}

// portPreflight refuses a start that would fail on an allocated host port, and
// says which port and what holds it.
//
// Without it the user gets compose's own failure: a daemon error about
// "programming external connectivity", printed twice, with the port buried in
// it and no hint of what to do. docs/PLAN.md has promised this check since the
// first draft.
//
// It never invents a failure. Anything it cannot resolve — compose declining to
// render the config, a port it cannot decide about — falls through to the real
// `up`, which produces the error it always did.
func portPreflight(ctx context.Context, p *prepared, runner *compose.Runner) error {
	wanted, err := runner.HostPorts(ctx, p.project)
	if err != nil || len(wanted) == 0 {
		return nil
	}

	ports := make([]int, 0, len(wanted))
	for _, w := range wanted {
		ports = append(ports, w.Port)
	}

	ctx, cancel := context.WithTimeout(ctx, dockerTimeout)
	defer cancel()

	conflicts := docker.New().PortConflicts(ctx, ports, p.project.Name())
	if len(conflicts) == 0 {
		return nil
	}

	// The port's env var is what the user needs to change, so map back to it
	// from the number compose gave us.
	names := make(map[int]string, len(p.stack.Ports))
	for _, port := range p.stack.Ports {
		names[p.env.Int(port.Name, port.Default)] = port.Name
	}

	held := func(c docker.Conflict) string {
		if c.Holder == "" {
			return fmt.Sprintf("%d — in use", c.Port)
		}
		return fmt.Sprintf("%d — %s", c.Port, c.Holder)
	}

	// One override flag per conflict, so the suggested command fixes the whole
	// start rather than the first port and then failing on the second.
	var flags []string
	for _, c := range conflicts {
		if name, ok := names[c.Port]; ok {
			flags = append(flags, fmt.Sprintf("--port %s=%d", name, c.Port+1))
		}
	}

	var b strings.Builder
	if len(conflicts) == 1 {
		c := conflicts[0]
		if c.Holder == "" {
			fmt.Fprintf(&b, "%s needs port %d, which is already in use", p.stack.Name, c.Port)
		} else {
			fmt.Fprintf(&b, "%s needs port %d, which %s is using", p.stack.Name, c.Port, c.Holder)
		}
	} else {
		fmt.Fprintf(&b, "%s needs %d ports that are already in use:", p.stack.Name, len(conflicts))
		for _, c := range conflicts {
			fmt.Fprintf(&b, "\n  %s", held(c))
		}
	}
	if len(flags) > 0 {
		fmt.Fprintf(&b, "\n\n  start it elsewhere:  spin up %s %s"+
			"\n  or change it for good:  spin env %s --edit",
			p.stack.Name, strings.Join(flags, " "), p.stack.Name)
	}
	return failf(ExitUsage, "%s", b.String())
}
