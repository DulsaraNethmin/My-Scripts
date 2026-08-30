package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/config"
	"github.com/DulsaraNethmin/spinup/internal/docker"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func newDoctorCmd(dc *docker.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check that Docker and spinup are ready",
		Long: "doctor checks the things that stop a stack from starting: the docker CLI,\n" +
			"a running daemon, Compose v2, and spinup's own catalog and configuration.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			ctx := cmd.Context()

			dockerChecks := dc.Diagnose(ctx)
			section(out, "docker", dockerChecks)

			spinupChecks := diagnoseSpinup(ctx)
			section(out, "spinup", spinupChecks)

			switch {
			case !docker.OK(dockerChecks):
				// Exit 2 is "docker unavailable", which is the useful signal
				// for anything scripting spinup.
				return failf(ExitDocker, "docker is not ready")
			case !docker.OK(spinupChecks):
				return failf(ExitUsage, "spinup is not ready")
			}

			fmt.Fprintf(out, "\n%s\n", ui.Success("everything checks out"))
			return nil
		},
	}
}

// diagnoseSpinup checks the half of the setup that is spinup's own: where its
// state lives, whether the catalog parses, and whether config.yaml is valid.
func diagnoseSpinup(ctx context.Context) []docker.Check {
	var checks []docker.Check

	paths, err := config.DefaultPaths()
	if err != nil {
		checks = append(checks, docker.Check{
			Name: "home", Detail: err.Error(), Status: docker.StatusFail,
			Hint: "set " + config.HomeEnv + " to a writable directory",
		})
		return checks
	}
	checks = append(checks, docker.Check{Name: "home", Detail: paths.Root, Status: docker.StatusOK})

	switch _, err := config.LoadConfig(paths.ConfigFile()); {
	case err != nil:
		checks = append(checks, docker.Check{
			Name: "config", Detail: err.Error(), Status: docker.StatusFail,
			Hint: "fix or delete " + paths.ConfigFile(),
		})
	default:
		detail := paths.ConfigFile()
		if _, err := os.Stat(paths.ConfigFile()); err != nil {
			detail = "defaults (no config.yaml yet)"
		}
		checks = append(checks, docker.Check{Name: "config", Detail: detail, Status: docker.StatusOK})
	}

	cat, ok := catalog.FromContext(ctx)
	if !ok {
		return checks
	}

	stacks, err := cat.All()
	check := docker.Check{Name: "catalog", Detail: fmt.Sprintf("%d stacks", len(stacks)), Status: docker.StatusOK}
	if err != nil {
		// A broken stack in ~/.spinup/stacks is the user's own; the built-in
		// ones still work, so this is a warning rather than a failure.
		check.Status = docker.StatusWarn
		check.Detail = fmt.Sprintf("%d stacks, and one that does not parse", len(stacks))
		check.Hint = err.Error()
	}
	return append(checks, check)
}

func section(out io.Writer, title string, checks []docker.Check) {
	fmt.Fprintf(out, "\n%s\n", ui.Bold(title))
	for _, c := range checks {
		// A check's detail comes from docker or a yaml parser and can arrive
		// with newlines in it, which would wreck the column layout.
		detail := strings.Join(strings.Fields(c.Detail), " ")
		fmt.Fprintf(out, "  %s  %-9s %s\n", status(c.Status), c.Name, detail)
		if c.Hint != "" {
			fmt.Fprintf(out, "        %s %s\n", ui.Dim("->"), c.Hint)
		}
	}
}

func status(s docker.Status) string {
	switch s {
	case docker.StatusOK:
		return ui.Success("ok  ")
	case docker.StatusWarn:
		return ui.Warn("warn")
	default:
		return ui.Error("fail")
	}
}
