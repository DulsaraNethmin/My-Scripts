// Package cmd holds spinup's command-line surface: one file per command.
//
// Commands own all user-facing behaviour — printing, prompting and the process
// exit code. The internal/ packages they call stay silent and never exit, so
// they stay usable from tests and from each other.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/config"
	"github.com/DulsaraNethmin/spinup/internal/docker"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

// Exit codes, as documented in docs/PLAN.md §3. Scripts depend on these, so
// they are part of spinup's interface.
const (
	ExitOK       = 0 // success
	ExitUsage    = 1 // bad invocation, or a failure with no more specific code
	ExitDocker   = 2 // docker or compose v2 unavailable
	ExitNotFound = 3 // no such stack
	ExitCompose  = 4 // docker compose ran and failed
)

// Build is the version information main injects via -ldflags.
type Build struct {
	Version string
	Commit  string
	Date    string
}

// exitError attaches an exit code to a failure. Commands return one when the
// failure maps to something more specific than "it didn't work".
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

// failf builds an error that exits with the given code.
func failf(code int, format string, a ...any) error {
	return &exitError{code: code, err: fmt.Errorf(format, a...)}
}

// Execute runs the CLI and returns the process exit code. It prints its own
// errors, so main only has to hand the code to os.Exit.
func Execute(b Build, stacks fs.FS) int {
	// Ctrl-C cancels the running command. Docker Compose is a child process and
	// gets the signal from the terminal itself, so this is about spinup winding
	// down cleanly rather than about stopping containers.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Commands read the catalog off the context, so tests can swap in a stack
	// tree without touching the embedded one.
	ctx = catalog.NewContext(ctx, userCatalog(stacks))

	if err := newRootCmd(b).ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, ui.Error("error:"), err)
		return codeFor(err)
	}
	return ExitOK
}

// codeFor maps an error to an exit code, defaulting to ExitUsage for anything
// that did not ask for something more specific.
func codeFor(err error) int {
	var ex *exitError
	if errors.As(err, &ex) {
		return ex.code
	}
	if errors.Is(err, catalog.ErrNotFound) {
		return ExitNotFound
	}
	return ExitUsage
}

// userCatalog layers ~/.spinup/stacks over the stacks embedded in the binary,
// so a stack the user has edited wins over the shipped copy. Not being able to
// find a home directory is not fatal — it only means there are no user stacks.
func userCatalog(embedded fs.FS) *catalog.Catalog {
	cat := catalog.New(embedded)

	paths, err := config.DefaultPaths()
	if err != nil {
		return cat
	}
	return cat.WithUserStacks(os.DirFS(paths.StacksDir()))
}

func newRootCmd(b Build) *cobra.Command {
	var noColor bool

	root := &cobra.Command{
		Use:   "spinup",
		Short: "Start local development services with one command",
		Long: "spinup starts local development services — databases, queues, GUIs and\n" +
			"dev tooling — with one command, using Docker Compose underneath.\n\n" +
			"Every stack is a plain compose.yaml, embedded in this binary and\n" +
			"materialised into ~/.spinup/stacks so you can read and tweak it.",
		Version:       b.Version,
		SilenceUsage:  true, // usage after a failed run is noise; see the flag error below
		SilenceErrors: true, // Execute prints the error, so cobra should not
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			if noColor {
				ui.SetColor(false)
			}
		},
	}

	root.SetVersionTemplate("{{.Version}}\n")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false,
		"disable colour output (NO_COLOR is honoured too)")

	// A malformed invocation is the one case where usage actually helps.
	root.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		_ = c.Usage()
		return failf(ExitUsage, "%w", err)
	})

	root.AddCommand(
		newUpCmd(),
		newDownCmd(),
		newRestartCmd(),
		newDestroyCmd(),
		newListCmd(),
		newPSCmd(),
		newLogsCmd(),
		newEnvCmd(),
		newShellCmd(),
		newCLICmd(),
		newOpenCmd(),
		newURLCmd(),
		newInfoCmd(),
		newNewCmd(),
		newResetCmd(),
		newDoctorCmd(docker.New()),
		newUpdateCmd(b),
		newVersionCmd(b),
	)

	return root
}
