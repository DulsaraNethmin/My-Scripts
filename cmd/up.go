package cmd

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/compose"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func newUpCmd() *cobra.Command {
	var (
		flags   profileFlags
		build   bool
		pull    bool
		noWait  bool
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
				return failf(ExitUsage, "which stack? try `spinup up postgres`")
			}

			for _, name := range names {
				p, err := prepare(ctx, name, flags)
				if err != nil {
					return err
				}

				heading(out, name)

				runner := compose.New(out, cmd.ErrOrStderr())
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

				summarise(out, p)
			}
			return nil
		},
	}

	flags.register(cmd)
	cmd.Flags().BoolVar(&build, "build", false, "build images before starting")
	cmd.Flags().BoolVar(&pull, "pull", false, "pull newer images before starting")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "do not wait for the stack to become healthy")
	cmd.Flags().DurationVar(&timeout, "timeout", 3*time.Minute, "how long to wait for healthy")

	return cmd
}

// catalogExpand resolves the ${VAR} references a spinup.yaml value is written
// in, against the stack's resolved environment.
func catalogExpand(value string, p *prepared) string {
	return catalog.Expand(value, p.env)
}

// summarise prints what the user came for: where the thing they just started
// is, and how to connect to it. The fuller card is task 4.3.
func summarise(out io.Writer, p *prepared) {
	s := p.stack

	url := catalogExpand(s.URL, p)
	if url != "" {
		fmt.Fprintf(out, "  %-9s %s\n", ui.Dim("url"), url)
	}

	// For a stack whose GUI is its primary service the two are the same
	// address, and repeating it back adds nothing.
	if s.HasGUI() {
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
