package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/compose"
	"github.com/DulsaraNethmin/spinup/internal/config"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func newListCmd() *cobra.Command {
	var (
		quiet  bool
		asJSON bool
	)

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

			if asJSON {
				return writeJSON(out, stacksJSON(cat, stacks, running))
			}

			table := ui.NewTable("stack", "category", "status", "ports", "description")
			for _, s := range stacks {
				table.Row(
					s.Name,
					string(s.Category),
					statusCell(running, s.Name),
					strings.Join(stackPorts(cat, s), ", "),
					s.Description,
				)
			}
			table.Render(out)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "print only the stack names")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the catalog as JSON")
	cmd.MarkFlagsMutuallyExclusive("quiet", "json")

	return cmd
}

// stackJSON is `spin list --json`. It is an interface other programs read,
// so the fields are the ones a script would otherwise parse out of the table —
// and the ports are resolved, not the defaults, because a script that connects
// to the default port when the user has changed it is worse than no script.
type stackJSON struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Origin      string         `json:"origin"`
	Running     bool           `json:"running"`
	Status      string         `json:"status,omitempty"`
	Ports       map[string]int `json:"ports"`
	URL         string         `json:"url,omitempty"`
	GUI         string         `json:"gui,omitempty"`
	Profiles    []string       `json:"profiles,omitempty"`
}

func stacksJSON(cat *catalog.Catalog, stacks []*catalog.Stack, running map[string]string) []stackJSON {
	out := make([]stackJSON, 0, len(stacks))

	for _, s := range stacks {
		env := stackEnv(cat, s)

		ports := make(map[string]int, len(s.Ports))
		for _, p := range s.Ports {
			ports[p.Name] = env.Int(p.Name, p.Default)
		}

		status := running[s.Name]
		row := stackJSON{
			Name:        s.Name,
			Description: s.Description,
			Category:    string(s.Category),
			Origin:      string(s.Origin),
			Running:     strings.Contains(status, "running"),
			Status:      status,
			Ports:       ports,
			URL:         catalog.Expand(s.URL, env),
			Profiles:    s.Profiles,
		}
		if s.HasGUI() {
			row.GUI = catalog.Expand(s.GUI.URL, env)
		}
		out = append(out, row)
	}
	return out
}

// writeJSON prints a value as indented JSON with a trailing newline, which is
// what a terminal and a pipe both want.
func writeJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// stackEnv resolves a stack's environment for display, the same way `up`
// resolves it — including the stack's own .env.example, which is where the
// credentials in a connection string come from before a stack has ever been
// started. Anything unreadable leaves the display falling back to defaults
// rather than failing the command.
func stackEnv(cat *catalog.Catalog, s *catalog.Stack) config.Env {
	paths, err := config.DefaultPaths()
	if err != nil {
		return config.Env{}
	}

	example, err := cat.ReadFile(s.Name, catalog.EnvExample)
	if err != nil {
		example = nil
	}

	env, err := config.ResolveEnv(s, example, paths.EnvFile(s.Name))
	if err != nil {
		return config.Env{}
	}
	return env
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
func stackPorts(cat *catalog.Catalog, s *catalog.Stack) []string {
	env := stackEnv(cat, s)

	ports := make([]string, 0, len(s.Ports))
	for _, p := range s.Ports {
		ports = append(ports, fmt.Sprint(env.Int(p.Name, p.Default)))
	}
	return ports
}
