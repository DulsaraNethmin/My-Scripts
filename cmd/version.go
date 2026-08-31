package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func newVersionCmd(b Build) *cobra.Command {
	var short bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			if short {
				fmt.Fprintln(out, b.Version)
				return nil
			}

			fmt.Fprintf(out, "%s %s\n", ui.Bold("spin"), b.Version)
			for _, f := range [][2]string{
				{"commit", b.Commit},
				{"built", b.Date},
				{"go", runtime.Version()},
				{"platform", runtime.GOOS + "/" + runtime.GOARCH},
			} {
				fmt.Fprintf(out, "  %-9s %s\n", ui.Dim(f[0]), f[1])
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&short, "short", false, "print just the version number")

	return cmd
}
