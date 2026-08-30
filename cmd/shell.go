package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/compose"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

// loginShell picks bash when the image has it and falls back to sh. Alpine
// images (redis, nginx) have only sh; the database images mostly have bash, and
// it is the difference between arrow keys working and printing ^[[A.
var loginShell = []string{"sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi"}

func newShellCmd() *cobra.Command {
	var (
		flags profileFlags
		shell string
		user  string
	)

	cmd := &cobra.Command{
		Use:     "shell <stack> [service]",
		Aliases: []string{"sh"},
		Short:   "Open a shell in a stack's container",
		Long: "shell opens an interactive shell inside a running stack — its primary\n" +
			"service unless you name another one.\n\n" +
			"It is `docker compose exec` with the project, file and env file already\n" +
			"right, and your terminal handed straight to the container.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := requireDocker(ctx); err != nil {
				return err
			}

			p, err := prepare(ctx, args[0], flags)
			if err != nil {
				return err
			}

			service := p.stack.Primary
			if len(args) == 2 {
				service = args[1]
			}
			if service == "" {
				return failf(ExitUsage, "%s declares no primary service — name one: `spinup shell %s <service>`",
					p.stack.Name, p.stack.Name)
			}

			if err := requireRunning(ctx, p, service); err != nil {
				return err
			}

			command := loginShell
			if shell != "" {
				command = []string{shell}
			}

			runner := compose.New(cmd.OutOrStdout(), cmd.ErrOrStderr())
			err = runner.Exec(ctx, p.project, compose.ExecOptions{
				Service: service,
				Command: command,
				User:    user,
				NoTTY:   !stdinIsTerminal(),
			})
			if err != nil {
				return runCompose(err)
			}
			return nil
		},
	}

	flags.register(cmd)
	cmd.Flags().StringVar(&shell, "shell", "", "shell to run (default: bash if the image has it, else sh)")
	cmd.Flags().StringVar(&user, "user", "", "run as this user inside the container")

	return cmd
}

// stdinIsTerminal reports whether spinup was given a terminal. Compose asks for
// a TTY by default and fails when there is none, so a piped or scripted
// invocation has to say so with -T.
func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// requireRunning turns "compose exec failed" into the sentence the user
// actually needs: the stack is not up, and here is the command that ups it.
func requireRunning(ctx context.Context, p *prepared, service string) error {
	runner := compose.New(nil, nil)

	containers, err := runner.PS(ctx, p.project)
	if err != nil {
		// If compose cannot be asked, let the real command produce the error.
		return nil //nolint:nilerr // a failure to check is not a failure to run
	}

	var running []string
	for _, c := range containers {
		if !c.Running() {
			continue
		}
		if c.Service == service {
			return nil
		}
		running = append(running, c.Service)
	}

	if len(running) == 0 {
		return failf(ExitCompose, "%s is not running — start it with `spinup up %s`",
			p.stack.Name, p.stack.Name)
	}
	return failf(ExitCompose, "%s has no running service called %s (running: %s)",
		p.stack.Name, service, strings.Join(running, ", "))
}

// clientCommand expands a stack's cli template into argv.
//
// The template is split into fields *before* its ${VARS} are expanded, so a
// password containing a space stays one argument and one containing a quote or
// a semicolon is not a shell injection: no shell is involved at any point.
func clientCommand(template string, env map[string]string) []string {
	fields := strings.Fields(template)

	argv := make([]string, 0, len(fields))
	for _, f := range fields {
		argv = append(argv, catalog.Expand(f, env))
	}
	return argv
}

func newCLICmd() *cobra.Command {
	var flags profileFlags

	cmd := &cobra.Command{
		Use:   "cli <stack> [-- client args]",
		Short: "Open the stack's native client",
		Long: "cli runs the stack's own client inside its container — psql for postgres,\n" +
			"mysql for mysql, mongosh for mongodb, redis-cli for redis — already\n" +
			"pointed at the database and authenticated with the stack's credentials.\n\n" +
			"Anything after -- is passed to the client.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := requireDocker(ctx); err != nil {
				return err
			}

			names, extra := splitDashArgs(cmd, args)
			if len(names) != 1 {
				return failf(ExitUsage, "cli takes one stack: `spinup cli postgres`")
			}

			p, err := prepare(ctx, names[0], flags)
			if err != nil {
				return err
			}

			if p.stack.CLI == "" {
				return failf(ExitUsage, "%s has no native client — try `spinup shell %s`",
					p.stack.Name, p.stack.Name)
			}
			if p.stack.Primary == "" {
				return failf(ExitUsage, "%s declares no primary service to run its client in", p.stack.Name)
			}

			if err := requireRunning(ctx, p, p.stack.Primary); err != nil {
				return err
			}

			command := append(clientCommand(p.stack.CLI, p.env), extra...)

			// Only the client's name: the expanded command carries the
			// stack's password, and echoing it would put it in the scrollback
			// of everyone who ever runs `spinup cli` on a shared screen.
			fmt.Fprintf(cmd.ErrOrStderr(), "%s %s in %s\n",
				ui.Dim("=>"), command[0], p.project.Name())

			runner := compose.New(cmd.OutOrStdout(), cmd.ErrOrStderr())
			err = runner.Exec(ctx, p.project, compose.ExecOptions{
				Service: p.stack.Primary,
				Command: command,
				NoTTY:   !stdinIsTerminal(),
			})
			if err != nil {
				return runCompose(err)
			}
			return nil
		},
	}

	flags.register(cmd)

	return cmd
}
