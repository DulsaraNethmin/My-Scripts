package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/config"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func newResetCmd() *cobra.Command {
	var (
		yes     bool
		withEnv bool
	)

	cmd := &cobra.Command{
		Use:   "reset <stack>",
		Short: "Restore a built-in stack to the version inside spinup",
		Long: "reset throws away your edits to a built-in stack and puts back the copy\n" +
			"compiled into this binary. It is the way out of a compose.yaml that no\n" +
			"longer works.\n\n" +
			"It does not touch your data: the volumes stay, and so does\n" +
			"~/.spinup/env/<stack>.env with its ports and passwords, unless you pass\n" +
			"--env. A stack of your own is never reset — there would be nothing to\n" +
			"restore it from.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeOneStack,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()
			name := args[0]

			cat, ok := catalog.FromContext(ctx)
			if !ok {
				return fmt.Errorf("no catalog: this is a bug in spinup")
			}

			if !cat.Has(name) {
				return failf(ExitNotFound, "%q: %w", name, catalog.ErrNotFound)
			}

			paths, err := config.DefaultPaths()
			if err != nil {
				return failf(ExitUsage, "%w", err)
			}
			dir := paths.StackDir(name)
			envFile := paths.EnvFile(name)

			if !cat.HasBuiltin(name) {
				return failf(ExitUsage,
					"%s is your own stack, not a built-in one — there is no copy to restore it from\n"+
						"(its files are yours to edit or delete: %s)",
					name, dir)
			}

			if _, err := os.Stat(dir); os.IsNotExist(err) && !withEnv {
				fmt.Fprintf(out, "%s is already the built-in version\n", name)
				return nil
			}

			if !yes {
				ok, err := confirmReset(cmd, dir, envFile, withEnv)
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(out, "nothing was reset")
					return nil
				}
			}

			if err := os.RemoveAll(dir); err != nil {
				return failf(ExitUsage, "removing %s: %w", dir, err)
			}
			if withEnv {
				if err := os.Remove(envFile); err != nil && !os.IsNotExist(err) {
					return failf(ExitUsage, "removing %s: %w", envFile, err)
				}
			}

			// Writing the stack back out now, rather than leaving it to the
			// next `up`, is what makes the result inspectable: the user can
			// diff their editor's undo history against a file that exists.
			if _, err := cat.Materialize(name, dir); err != nil {
				return failf(ExitUsage, "%w", err)
			}
			if _, err := cat.SeedEnv(name, envFile); err != nil {
				return failf(ExitUsage, "%w", err)
			}

			fmt.Fprintf(out, "%s %s restored from the built-in copy\n", ui.Success("✓"), name)
			fmt.Fprintf(out, "  %s %s\n", ui.Dim("files"), dir)
			if withEnv {
				fmt.Fprintf(out, "  %s %s\n", ui.Dim("env"), envFile)
			} else {
				fmt.Fprintf(out, "  %s %s %s\n", ui.Dim("env"), envFile, ui.Dim("(kept)"))
			}
			fmt.Fprintf(out, "  %s\n", ui.Dim("data volumes are untouched"))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "do not ask for confirmation")
	cmd.Flags().BoolVar(&withEnv, "env", false, "also restore the env file, losing edited ports and passwords")

	return cmd
}

// confirmReset asks before throwing away edits. Like destroy, the default is
// no: what is being deleted is the user's own work, even if it is only a
// changed port.
func confirmReset(cmd *cobra.Command, dir, envFile string, withEnv bool) (bool, error) {
	out := cmd.OutOrStdout()

	fmt.Fprintf(out, "%s replaces %s with the copy inside spinup.\n", ui.Warn("reset"), ui.Bold(dir))
	if withEnv {
		fmt.Fprintf(out, "%s is rewritten too — edited ports and passwords are lost.\n", envFile)
	}
	fmt.Fprintf(out, "Data volumes are not touched. Continue? [y/N]: ")

	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(out)
		return false, failf(ExitUsage, "reset needs an answer; pass -y to skip the prompt")
	}

	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
