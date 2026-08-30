package cmd

import (
	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/compose"
)

func newLogsCmd() *cobra.Command {
	var (
		flags      profileFlags
		follow     bool
		tail       int
		timestamps bool
	)

	cmd := &cobra.Command{
		Use:   "logs <stack> [service...]",
		Short: "Show a stack's logs",
		Long: "logs prints the logs of every service in a stack, or of the services you\n" +
			"name. With -f it follows them until you interrupt it.",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeOneStack,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := requireDocker(ctx); err != nil {
				return err
			}

			p, err := prepare(ctx, args[0], flags)
			if err != nil {
				return err
			}

			runner := compose.New(cmd.OutOrStdout(), cmd.ErrOrStderr())
			err = runner.Logs(ctx, p.project, compose.LogsOptions{
				Follow:     follow,
				Tail:       tail,
				Timestamps: timestamps,
				Services:   args[1:],
			})
			if err != nil {
				// Following logs ends when the user interrupts it, and that is
				// not a failure.
				if ctx.Err() != nil {
					return nil
				}
				return runCompose(err)
			}
			return nil
		},
	}

	flags.register(cmd)
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow the log output")
	cmd.Flags().IntVar(&tail, "tail", 0, "show only this many trailing lines")
	cmd.Flags().BoolVarP(&timestamps, "timestamps", "t", false, "show timestamps")

	return cmd
}
