package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func newEnvCmd() *cobra.Command {
	var (
		flags    profileFlags
		edit     bool
		pathOnly bool
	)

	cmd := &cobra.Command{
		Use:   "env <stack>",
		Short: "Show or edit a stack's ports and credentials",
		Long: "env prints the environment a stack will start with: its ports, passwords\n" +
			"and database names, resolved exactly as `up` resolves them.\n\n" +
			"--edit opens the stack's env file in $EDITOR. Changes take effect on the\n" +
			"next `spinup up`.",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeOneStack,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			// env is about spinup's own files, so it works with Docker
			// stopped — which is often exactly when you want to change a port.
			p, err := prepare(ctx, args[0], flags)
			if err != nil {
				return err
			}

			file := p.paths.EnvFile(p.stack.Name)

			switch {
			case pathOnly:
				fmt.Fprintln(out, file)
				return nil

			case edit:
				return editFile(cmd, p.stack.Name, file)
			}

			fmt.Fprintf(out, "%s %s\n", ui.Dim("#"), file)

			keys := make([]string, 0, len(p.env))
			for k := range p.env {
				keys = append(keys, k)
			}
			slices.Sort(keys)

			for _, k := range keys {
				fmt.Fprintf(out, "%s=%s\n", k, p.env[k])
			}
			return nil
		},
	}

	flags.register(cmd)
	cmd.Flags().BoolVar(&edit, "edit", false, "open the env file in $EDITOR")
	cmd.Flags().BoolVar(&pathOnly, "path", false, "print the path of the env file and nothing else")
	cmd.MarkFlagsMutuallyExclusive("edit", "path")

	return cmd
}

// editFile opens a file in the user's editor, wired to the real terminal so an
// editor like vim works.
func editFile(cmd *cobra.Command, stack, file string) error {
	editor := firstNonEmpty(os.Getenv("VISUAL"), os.Getenv("EDITOR"))
	if editor == "" {
		return failf(ExitUsage, "no editor: set $EDITOR, or open %s yourself", file)
	}

	// $EDITOR can carry flags, e.g. "code --wait".
	fields := strings.Fields(editor)
	args := append(fields[1:], file)

	editing := exec.CommandContext(cmd.Context(), fields[0], args...)
	editing.Stdin = os.Stdin
	editing.Stdout = os.Stdout
	editing.Stderr = os.Stderr

	if err := editing.Run(); err != nil {
		return failf(ExitUsage, "%s: %w", editor, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s\n", ui.Dim("run `spinup up "+stack+"` to apply the changes"))
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
