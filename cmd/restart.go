package cmd

import (
	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/compose"
)

func newRestartCmd() *cobra.Command {
	var flags profileFlags

	cmd := &cobra.Command{
		Use:   "restart <stack>... [-- docker compose args]",
		Short: "Restart one or more stacks",
		Long: "restart restarts a stack's containers in place. Data is untouched, and so\n" +
			"is anything you changed in the stack's env file — those need an up.",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completeStacks,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			if err := requireDocker(ctx); err != nil {
				return err
			}

			names, extra := splitDashArgs(cmd, args)
			for _, name := range names {
				p, err := prepare(ctx, name, flags)
				if err != nil {
					return err
				}

				heading(out, name)

				runner := compose.New(out, cmd.ErrOrStderr())
				if err := runner.Restart(ctx, p.project, extra...); err != nil {
					return runCompose(err)
				}
			}
			return nil
		},
	}

	flags.register(cmd)

	return cmd
}
