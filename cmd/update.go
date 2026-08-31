package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/ui"
	"github.com/DulsaraNethmin/spinup/internal/update"
)

// executablePath is os.Executable, with the symlinks resolved: on Homebrew a
// binary is reached through a symlink, and it is the real file that has to be
// replaced. The tests swap it so they can update a scratch file instead of the
// test binary.
var executablePath = func() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path, nil //nolint:nilerr // an unresolvable symlink is still a usable path
	}
	return resolved, nil
}

func newUpdateCmd(b Build) *cobra.Command {
	var check, force bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update spinup to the latest release",
		Long: "update replaces this binary with the latest release, after checking it\n" +
			"against the release's checksums.\n\n" +
			"If spinup came from Homebrew or Scoop, update says so and stops: the\n" +
			"package manager owns that file, and overwriting it behind its back is\n" +
			"undone by the next upgrade. --force overrides that.\n\n" +
			"SPINUP_REPO and SPINUP_API point it at another repository or API.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			path, err := executablePath()
			if err != nil {
				return failf(ExitUsage, "cannot find the running spinup: %w", err)
			}

			if mgr, ok := update.ManagedBy(path); ok && !force && !check {
				return failf(ExitUsage,
					"spinup was installed with %s, which owns %s\n\n  %s\n\n"+
						"(--force replaces the binary anyway)",
					mgr.Name, path, mgr.Command)
			}

			client := update.NewClient()
			rel, err := client.Latest(ctx)
			if err != nil {
				return failf(ExitUsage, "%w", err)
			}

			if update.Same(b.Version, rel.Tag) {
				fmt.Fprintf(out, "%s spinup %s is the latest release\n", ui.Success("✓"), b.Version)
				return nil
			}

			fmt.Fprintf(out, "%s is available — you have %s\n", ui.Bold(rel.Tag), b.Version)
			if check {
				if mgr, ok := update.ManagedBy(path); ok {
					fmt.Fprintf(out, "  update it with: %s\n", mgr.Command)
					return nil
				}
				fmt.Fprintln(out, "  update it with: spin update")
				return nil
			}

			name := update.ArchiveName(rel.Tag, runtime.GOOS, runtime.GOARCH)

			archive, err := client.Asset(ctx, rel, name)
			if err != nil {
				return failf(ExitUsage, "%w", err)
			}

			// The checksum is the whole reason this is safe to do unattended: it
			// is the file the release signs with cosign, so a mirror or a proxy
			// cannot hand spinup a different binary.
			checksums, err := client.Asset(ctx, rel, "checksums.txt")
			if err != nil {
				return failf(ExitUsage, "%w", err)
			}
			if err := update.Verify(checksums, archive, name); err != nil {
				return failf(ExitUsage, "%w", err)
			}

			binary, err := update.Binary(archive, runtime.GOOS, update.SelfName(path))
			if err != nil {
				return failf(ExitUsage, "%w", err)
			}
			if err := update.Replace(path, binary); err != nil {
				return failf(ExitUsage, "%w\n\nspinup is at %s — installing there may need sudo", err, path)
			}

			fmt.Fprintf(out, "%s spinup %s installed at %s\n", ui.Success("✓"), rel.Tag, path)
			return nil
		},
	}

	cmd.Flags().BoolVar(&check, "check", false, "only report whether a newer release exists")
	cmd.Flags().BoolVar(&force, "force", false, "replace the binary even if a package manager owns it")

	return cmd
}
