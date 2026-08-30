package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/compose"
	"github.com/DulsaraNethmin/spinup/internal/config"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func newListCmd() *cobra.Command {
	var quiet bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List the stacks in the catalog",
		Long: "list shows every stack spinup knows about: the ones built into the binary\n" +
			"and any of your own in ~/.spinup/stacks, with the ports they will use and\n" +
			"whether they are running.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			cat, ok := catalog.FromContext(ctx)
			if !ok {
				return fmt.Errorf("no catalog: this is a bug in spinup")
			}

			stacks, err := cat.All()
			if err != nil {
				// A stack of the user's that does not parse is worth saying
				// out loud, but the rest of the catalog still lists.
				fmt.Fprintf(cmd.ErrOrStderr(), "%s %v\n", ui.Warn("warning:"), err)
			}

			if quiet {
				for _, s := range stacks {
					fmt.Fprintln(out, s.Name)
				}
				return nil
			}

			// Without Docker the catalog is still worth listing, so a failure
			// here only costs the status column.
			running := runningStacks(ctx)

			table := ui.NewTable("stack", "category", "status", "ports", "description")
			for _, s := range stacks {
				table.Row(
					s.Name,
					string(s.Category),
					statusCell(running, s.Name),
					strings.Join(stackPorts(s), ", "),
					s.Description,
				)
			}
			table.Render(out)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "print only the stack names")

	return cmd
}

// runningStacks maps stack name to compose's status string. One `compose ls`
// answers for the whole catalog, rather than one call per stack.
func runningStacks(ctx context.Context) map[string]string {
	projects, err := compose.New(nil, nil).ListProjects(ctx)
	if err != nil {
		return nil
	}

	status := map[string]string{}
	for _, p := range projects {
		if name, ok := p.Stack(); ok {
			status[name] = p.Status
		}
	}
	return status
}

func statusCell(running map[string]string, name string) string {
	status, ok := running[name]
	switch {
	case !ok:
		return ui.Dim("-")
	case strings.Contains(status, "running"):
		return ui.Success(status)
	default:
		return ui.Dim(status)
	}
}

// stackPorts is the host ports a stack will bind, resolved the same way `up`
// resolves them, so what list shows is what compose will publish.
func stackPorts(s *catalog.Stack) []string {
	env := config.Env{}
	if paths, err := config.DefaultPaths(); err == nil {
		if resolved, err := config.ResolveEnv(s, nil, paths.EnvFile(s.Name)); err == nil {
			env = resolved
		}
	}

	ports := make([]string, 0, len(s.Ports))
	for _, p := range s.Ports {
		ports = append(ports, fmt.Sprint(env.Int(p.Name, p.Default)))
	}
	return ports
}
