package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/compose"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func newDownCmd() *cobra.Command {
	var (
		flags   profileFlags
		timeout time.Duration
	)

	cmd := &cobra.Command{
		Use:   "down <stack>... [-- docker compose args]",
		Short: "Stop one or more stacks, keeping their data",
		Long: "down stops a stack's containers and leaves its data volumes alone, so\n" +
			"`spin up` brings it back with everything still in it.\n\n" +
			"To delete the data, use `spin destroy`.",
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
				err = runner.Down(ctx, p.project, compose.DownOptions{
					Timeout: timeout,
					Extra:   extra,
				})
				if err != nil {
					return runCompose(err)
				}

				fmt.Fprintf(out, "  %s\n", ui.Dim("data kept — `spin destroy "+name+"` deletes it"))
			}
			return nil
		},
	}

	flags.register(cmd)
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "how long to wait for containers to stop")

	return cmd
}
