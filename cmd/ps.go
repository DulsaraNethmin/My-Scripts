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
	var asJSON bool

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
				if asJSON {
					// An empty array, not prose: a script that asks what is
					// running has to be able to read the answer "nothing".
					return writeJSON(out, []containerJSON{})
				}
				fmt.Fprintln(out, ui.Dim("no spinup stacks are running — try `spin up postgres`"))
				return nil
			}

			paths, err := config.DefaultPaths()
			if err != nil {
				return err
			}

			table := ui.NewTable("stack", "service", "status", "health", "ports")
			rows := []containerJSON{}

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
					if asJSON {
						rows = append(rows, containerToJSON(name, c))
						continue
					}
					table.Row(name, c.Service, statusCellFor(c), healthCell(c), portsCell(c))
				}
			}

			if asJSON {
				return writeJSON(out, rows)
			}

			if table.Rows() == 0 {
				fmt.Fprintln(out, ui.Dim("no containers"))
				return nil
			}
			table.Render(out)

			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "print the containers as JSON")

	return cmd
}

// containerJSON is `spin ps --json`: compose's own view of a container,
// named the way spinup names things, with the published ports flattened into
// something a script can index.
type containerJSON struct {
	Stack     string     `json:"stack"`
	Service   string     `json:"service"`
	Container string     `json:"container"`
	Image     string     `json:"image"`
	State     string     `json:"state"`
	Status    string     `json:"status"`
	Health    string     `json:"health,omitempty"`
	Running   bool       `json:"running"`
	Healthy   bool       `json:"healthy"`
	Ports     []portJSON `json:"ports"`
}

type portJSON struct {
	Published int    `json:"published"`
	Target    int    `json:"target"`
	Protocol  string `json:"protocol,omitempty"`
}

func containerToJSON(stack string, c compose.Container) containerJSON {
	row := containerJSON{
		Stack:     stack,
		Service:   c.Service,
		Container: c.Name,
		Image:     c.Image,
		State:     c.State,
		Status:    c.Status,
		Health:    c.Health,
		Running:   c.Running(),
		Healthy:   c.Healthy(),
		Ports:     []portJSON{},
	}

	// compose reports one publisher per address family, so a port bound on
	// both IPv4 and IPv6 arrives twice.
	seen := map[int]bool{}
	for _, p := range c.Publishers {
		if p.PublishedPort == 0 || seen[p.PublishedPort] {
			continue
		}
		seen[p.PublishedPort] = true
		row.Ports = append(row.Ports, portJSON{
			Published: p.PublishedPort,
			Target:    p.TargetPort,
			Protocol:  p.Protocol,
		})
	}
	return row
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
