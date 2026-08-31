package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/config"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func newNewCmd() *cobra.Command {
	var from string

	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Scaffold a stack of your own",
		Long: "new writes a new stack into ~/.spinup/stacks/<name>/ — the four files\n" +
			"every stack has — and it runs as it stands, so you can `spin up` it\n" +
			"first and edit it second.\n\n" +
			"--from <stack> starts from a copy of an existing stack instead. A stack\n" +
			"in ~/.spinup/stacks shadows a built-in one of the same name, so copying\n" +
			"under a new name is how you keep both.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()
			name := args[0]

			cat, ok := catalog.FromContext(ctx)
			if !ok {
				return fmt.Errorf("no catalog: this is a bug in spinup")
			}

			if !catalog.ValidName(name) {
				return failf(ExitUsage,
					"%q is not a stack name — lower case letters, digits and dashes", name)
			}
			if cat.Has(name) {
				return failf(ExitUsage, "%s already exists — `spin info %s`, or pick another name", name, name)
			}

			paths, err := config.DefaultPaths()
			if err != nil {
				return failf(ExitUsage, "%w", err)
			}
			dir := paths.StackDir(name)

			// The catalog only sees directories that exist, so a half-written
			// stack from an interrupted run would be invisible to Has above.
			if _, err := os.Stat(dir); err == nil {
				return failf(ExitUsage, "%s already exists", dir)
			}

			var written []string
			if from != "" {
				written, err = copyStack(cat, from, name, dir)
			} else {
				written, err = writeScaffold(name, dir)
			}
			if err != nil {
				return failf(ExitUsage, "%w", err)
			}

			slices.Sort(written)

			fmt.Fprintf(out, "%s %s\n", ui.Success("created"), dir)
			for _, f := range written {
				fmt.Fprintf(out, "  %s\n", ui.Dim(f))
			}
			// A copy keeps the original's ports, so the two cannot run at the
			// same time until one of them is changed. Better said now than
			// discovered as "port is already allocated".
			if from != "" {
				fmt.Fprintf(out, "\n%s %s uses the same ports as %s — change them in spinup.yaml and\n"+
					".env.example before running both.\n", ui.Warn("note:"), name, from)
			}

			fmt.Fprintf(out, "\nNext:\n  %s\n  %s\n",
				ui.Dim("spin up "+name), ui.Dim("$EDITOR "+filepath.Join(dir, "compose.yaml")))
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "copy an existing stack instead of the template")

	return cmd
}

// writeScaffold writes a new stack's files, and cleans up after itself if it
// cannot finish: half a stack is worse than none, because `spin list` would
// then report a broken one forever.
func writeScaffold(name, dir string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	var written []string
	for path, data := range catalog.Scaffold(name) {
		if err := os.WriteFile(filepath.Join(dir, path), data, 0o644); err != nil {
			os.RemoveAll(dir) //nolint:errcheck // the write error is the one worth reporting
			return nil, err
		}
		written = append(written, path)
	}
	return written, nil
}

// copyStack materialises an existing stack under a new name, rewriting the name
// in its spinup.yaml — the catalog requires the two to agree, so a plain copy
// would produce a stack that does not load.
func copyStack(cat *catalog.Catalog, from, name, dir string) ([]string, error) {
	if !cat.Has(from) {
		return nil, fmt.Errorf("%q: %w", from, catalog.ErrNotFound)
	}

	written, err := cat.Materialize(from, dir)
	if err != nil {
		os.RemoveAll(dir) //nolint:errcheck // see writeScaffold
		return nil, err
	}

	meta := filepath.Join(dir, "spinup.yaml")
	data, err := os.ReadFile(meta)
	if err != nil {
		os.RemoveAll(dir) //nolint:errcheck // see writeScaffold
		return nil, err
	}

	updated := strings.Replace(string(data), "name: "+from+"\n", "name: "+name+"\n", 1)
	if updated == string(data) {
		// The catalog requires the name and the directory to agree, so a copy
		// whose name was not rewritten would not load. Saying so here beats
		// leaving the user with a stack that fails on its next command.
		os.RemoveAll(dir) //nolint:errcheck // see writeScaffold
		return nil, fmt.Errorf("%s does not spell its name as `name: %s`, so it cannot be copied automatically",
			filepath.Join(from, "spinup.yaml"), from)
	}
	if err := os.WriteFile(meta, []byte(updated), 0o644); err != nil {
		os.RemoveAll(dir) //nolint:errcheck // see writeScaffold
		return nil, err
	}
	return written, nil
}
