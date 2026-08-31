package cmd

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/config"
	"github.com/DulsaraNethmin/spinup/internal/docker"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func newDoctorCmd(dc *docker.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check that Docker and spinup are ready",
		Long: "doctor checks the things that stop a stack from starting: the docker CLI,\n" +
			"a running daemon, Compose v2, the NVIDIA runtime, spinup's own catalog and\n" +
			"configuration, and whether anything else already holds the ports the\n" +
			"catalog wants.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			ctx := cmd.Context()

			dockerChecks := dc.Diagnose(ctx)
			section(out, "docker", dockerChecks)

			spinupChecks := diagnoseSpinup(ctx)
			section(out, "spinup", spinupChecks)

			switch {
			case !docker.OK(dockerChecks):
				// Exit 2 is "docker unavailable", which is the useful signal
				// for anything scripting spinup.
				return failf(ExitDocker, "docker is not ready")
			case !docker.OK(spinupChecks):
				return failf(ExitUsage, "spinup is not ready")
			}

			// Warnings are not failures, but saying "everything checks out"
			// over three of them is how a tool teaches people to ignore it.
			if n := warnings(dockerChecks) + warnings(spinupChecks); n > 0 {
				fmt.Fprintf(out, "\n%s\n", ui.Warn(fmt.Sprintf("ready, with %s above", plural(n, "warning"))))
				return nil
			}

			fmt.Fprintf(out, "\n%s\n", ui.Success("everything checks out"))
			return nil
		},
	}
}

func warnings(checks []docker.Check) int {
	n := 0
	for _, c := range checks {
		if c.Status == docker.StatusWarn {
			n++
		}
	}
	return n
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// diagnoseSpinup checks the half of the setup that is spinup's own: where its
// state lives, whether the catalog parses, and whether config.yaml is valid.
func diagnoseSpinup(ctx context.Context) []docker.Check {
	var checks []docker.Check

	paths, err := config.DefaultPaths()
	if err != nil {
		checks = append(checks, docker.Check{
			Name: "home", Detail: err.Error(), Status: docker.StatusFail,
			Hint: "set " + config.HomeEnv + " to a writable directory",
		})
		return checks
	}
	checks = append(checks, docker.Check{Name: "home", Detail: paths.Root, Status: docker.StatusOK})

	switch _, err := config.LoadConfig(paths.ConfigFile()); {
	case err != nil:
		checks = append(checks, docker.Check{
			Name: "config", Detail: err.Error(), Status: docker.StatusFail,
			Hint: "fix or delete " + paths.ConfigFile(),
		})
	default:
		detail := paths.ConfigFile()
		if _, err := os.Stat(paths.ConfigFile()); err != nil {
			detail = "defaults (no config.yaml yet)"
		}
		checks = append(checks, docker.Check{Name: "config", Detail: detail, Status: docker.StatusOK})
	}

	cat, ok := catalog.FromContext(ctx)
	if !ok {
		return checks
	}

	stacks, err := cat.All()
	check := docker.Check{Name: "catalog", Detail: fmt.Sprintf("%d stacks", len(stacks)), Status: docker.StatusOK}
	if err != nil {
		// A broken stack in ~/.spinup/stacks is the user's own; the built-in
		// ones still work, so this is a warning rather than a failure.
		check.Status = docker.StatusWarn
		check.Detail = fmt.Sprintf("%d stacks, and one that does not parse", len(stacks))
		check.Hint = err.Error()
	}
	checks = append(checks, check)

	return append(checks, checkPorts(ctx, cat))
}

// checkPorts is the "why will this not start" check. Two things go wrong with
// host ports: something else on the machine already holds one, or two stacks
// want the same one — which docs/PORTS.md prevents for the built-in catalog but
// cannot for a stack of the user's own.
//
// Ports held by a spinup stack that is already running are not collisions: that
// is the stack doing its job.
func checkPorts(ctx context.Context, cat *catalog.Catalog) docker.Check {
	stacks, _ := cat.All() // a stack that does not parse is already reported by the catalog check

	type claim struct {
		stack string
		port  int
	}

	var claims []claim
	owners := map[int][]string{}
	for _, s := range stacks {
		env := stackEnv(cat, s)
		for _, p := range s.Ports {
			port := env.Int(p.Name, p.Default)
			claims = append(claims, claim{stack: s.Name, port: port})
			owners[port] = append(owners[port], s.Name)
		}
	}

	check := docker.Check{
		Name:   "ports",
		Detail: fmt.Sprintf("%d host ports, none in use", len(claims)),
		Status: docker.StatusOK,
	}
	if len(claims) == 0 {
		check.Detail = "no stacks to check"
		return check
	}

	running := runningStacks(ctx)

	// Probing concurrently: a dozen stacks is a dozen ports, and doctor should
	// answer in the time of the slowest one rather than the sum of them.
	inUse := make([]bool, len(claims))
	var wg sync.WaitGroup
	for i, c := range claims {
		if _, isRunning := running[c.stack]; isRunning {
			continue // the port is held by this stack, which is the point of it
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			inUse[i] = portInUse(c.port)
		}()
	}
	wg.Wait()

	var problems []string
	for i, c := range claims {
		if inUse[i] {
			problems = append(problems, fmt.Sprintf("%d (%s)", c.port, c.stack))
		}
	}

	var shared []string
	for port, stacks := range owners {
		if len(stacks) > 1 {
			slices.Sort(stacks)
			shared = append(shared, fmt.Sprintf("%d (%s)", port, strings.Join(stacks, " and ")))
		}
	}
	slices.Sort(shared)

	switch {
	case len(problems) > 0:
		check.Status = docker.StatusWarn
		check.Detail = "in use: " + strings.Join(problems, ", ")
		check.Hint = "free the port, or move the stack's: `spinup env <stack> --edit`, " +
			"or `spinup up <stack> --port NAME=<port>` for one run"
	case len(shared) > 0:
		check.Status = docker.StatusWarn
		check.Detail = "claimed twice: " + strings.Join(shared, ", ")
		check.Hint = "those stacks cannot run at the same time; move one with `spinup env <stack> --edit`"
	default:
		check.Detail = fmt.Sprintf("%d host ports, all free", len(claims))
	}
	return check
}

// portInUse reports whether something is already listening on a host port.
//
// Connecting, not binding. Binding gets both cases wrong on a developer
// machine: a port below 1024 fails with "permission denied" for a non-root
// user and looks taken when it is free, and Docker Desktop's published ports
// do not stop a second bind on 127.0.0.1, so a port that is very much in use
// looks free. A connection that is accepted is proof either way.
func portInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), portProbeTimeout)
	if err != nil {
		return false
	}
	conn.Close() //nolint:errcheck // nothing was written to it
	return true
}

// portProbeTimeout bounds one probe. A refused connection comes back in under a
// millisecond; the timeout only matters for a port something is filtering, and
// the probes run concurrently, so it bounds the whole check rather than each
// port in turn.
const portProbeTimeout = 300 * time.Millisecond

func section(out io.Writer, title string, checks []docker.Check) {
	fmt.Fprintf(out, "\n%s\n", ui.Bold(title))
	for _, c := range checks {
		// A check's detail comes from docker or a yaml parser and can arrive
		// with newlines in it, which would wreck the column layout.
		detail := strings.Join(strings.Fields(c.Detail), " ")
		fmt.Fprintf(out, "  %s  %-9s %s\n", status(c.Status), c.Name, detail)
		if c.Hint != "" {
			fmt.Fprintf(out, "        %s %s\n", ui.Dim("->"), c.Hint)
		}
	}
}

func status(s docker.Status) string {
	switch s {
	case docker.StatusOK:
		return ui.Success("ok  ")
	case docker.StatusWarn:
		return ui.Warn("warn")
	default:
		return ui.Error("fail")
	}
}
