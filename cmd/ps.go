package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/compose"
	"github.com/DulsaraNethmin/spinup/internal/config"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func newPSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ps [stack]",
		Short: "Show the containers of running stacks",
		Long: "ps lists the containers of a stack, or of every spinup stack that is\n" +
			"running, with their health and published ports.",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeOneStack,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			if err := requireDocker(ctx); err != nil {
				return err
			}

			runner := compose.New(nil, nil)

			var names []string
			if len(args) == 1 {
				names = args
			} else {
				projects, err := runner.ListProjects(ctx)
				if err != nil {
					return runCompose(err)
				}
				for _, p := range projects {
					if name, ok := p.Stack(); ok {
						names = append(names, name)
					}
				}
			}

			if len(names) == 0 {
				fmt.Fprintln(out, ui.Dim("no spinup stacks are running — try `spinup up postgres`"))
				return nil
			}

			paths, err := config.DefaultPaths()
			if err != nil {
				return err
			}

			table := ui.NewTable("stack", "service", "status", "health", "ports")
			for _, name := range names {
				project := compose.Project{
					Stack:   name,
					Dir:     paths.StackDir(name),
					EnvFile: paths.EnvFile(name),
				}

				containers, err := runner.PS(ctx, project)
				if err != nil {
					return runCompose(err)
				}
				for _, c := range containers {
					table.Row(name, c.Service, statusCellFor(c), healthCell(c), portsCell(c))
				}
			}

			if table.Rows() == 0 {
				fmt.Fprintln(out, ui.Dim("no containers"))
				return nil
			}
			table.Render(out)

			return nil
		},
	}

	return cmd
}

func healthCell(c compose.Container) string {
	switch {
	case c.Health == "" && c.Running():
		return ui.Dim("no check")
	case c.Health == "healthy":
		return ui.Success(c.Health)
	case c.Health == "":
		return ui.Dim("-")
	case c.Healthy():
		return c.Health
	default:
		return ui.Warn(c.Health)
	}
}

// portsCell lists a container's published ports. compose reports one publisher
// per address family, so a port bound on both IPv4 and IPv6 arrives twice and
// would otherwise be printed twice.
func portsCell(c compose.Container) string {
	var ports []string
	seen := map[string]bool{}

	for _, p := range c.Publishers {
		if p.PublishedPort == 0 {
			continue
		}

		port := fmt.Sprintf("%d->%d", p.PublishedPort, p.TargetPort)
		if seen[port] {
			continue
		}
		seen[port] = true
		ports = append(ports, port)
	}
	return strings.Join(ports, ", ")
}

// statusCellFor is compose's status without the health it repeats, since
// health has a column of its own. Only the health suffix is dropped — the
// exit code of a stopped container is worth keeping.
func statusCellFor(c compose.Container) string {
	status := c.Status
	for _, suffix := range []string{" (healthy)", " (unhealthy)", " (health: starting)", " (starting)"} {
		status = strings.TrimSuffix(status, suffix)
	}
	return status
}
