package cmd

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/compose"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func newDestroyCmd() *cobra.Command {
	var (
		flags profileFlags
		yes   bool
	)

	cmd := &cobra.Command{
		Use:   "destroy <stack>... [-- docker compose args]",
		Short: "Stop one or more stacks and delete their data",
		Long: "destroy stops a stack and removes its data volumes. This is the only\n" +
			"command that deletes anything, and it asks first unless you pass -y.\n\n" +
			"The stack's env file and its copy in ~/.spinup/stacks are left alone;\n" +
			"only the data goes.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			if err := requireDocker(ctx); err != nil {
				return err
			}

			names, extra := splitDashArgs(cmd, args)

			if !yes {
				ok, err := confirmDestroy(cmd, names)
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(out, "nothing was destroyed")
					return nil
				}
			}

			for _, name := range names {
				p, err := prepare(ctx, name, flags)
				if err != nil {
					return err
				}

				heading(out, name)

				runner := compose.New(out, cmd.ErrOrStderr())
				err = runner.Down(ctx, p.project, compose.DownOptions{
					Volumes:       true,
					RemoveOrphans: true,
					Extra:         extra,
				})
				if err != nil {
					return runCompose(err)
				}

				fmt.Fprintf(out, "  %s\n", ui.Warn("data deleted"))
			}
			return nil
		},
	}

	flags.register(cmd)
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do not ask for confirmation")

	return cmd
}

// confirmDestroy asks before deleting data. The default is no: a stray return
// key must not wipe a database.
func confirmDestroy(cmd *cobra.Command, names []string) (bool, error) {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "%s deletes the data volumes of %s.\n",
		ui.Warn("destroy"), ui.Bold(strings.Join(names, ", ")))
	fmt.Fprint(out, "This cannot be undone. Continue? [y/N]: ")

	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && line == "" {
		// No answer available — a pipe with nothing in it, say. Treat it as no
		// and point at the flag that means yes.
		fmt.Fprintln(out)
		return false, failf(ExitUsage, "destroy needs an answer; pass -y to skip the prompt")
	}

	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
