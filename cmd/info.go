package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func newInfoCmd() *cobra.Command {
	var (
		readme   bool
		noReadme bool
	)

	cmd := &cobra.Command{
		Use:   "info <stack>",
		Short: "Show what a stack is, with its ports and credentials",
		Long: "info is the stack's page: what it runs, the ports and credentials it will\n" +
			"use, how to connect, and then its README — what the image is good for and\n" +
			"whatever is worth knowing before you start it.\n\n" +
			"Like `env` and `url`, it works with Docker stopped.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			p, err := prepare(ctx, args[0], profileFlags{})
			if err != nil {
				return err
			}

			cat, ok := catalog.FromContext(ctx)
			if !ok {
				return fmt.Errorf("no catalog: this is a bug in spinup")
			}

			if !readme {
				writeInfoHeader(out, p)
			}

			if noReadme {
				return nil
			}

			doc, err := cat.ReadFile(p.stack.Name, "README.md")
			if err != nil {
				// A stack without a README is legal — a user's own usually is
				// one — so the metadata above is the whole answer.
				if !readme {
					return nil
				}
				return failf(ExitUsage, "%s has no README", p.stack.Name)
			}

			if !readme {
				fmt.Fprintln(out)
			}
			fmt.Fprintln(out, strings.TrimRight(string(doc), "\n"))
			return nil
		},
	}

	cmd.Flags().BoolVar(&readme, "readme", false, "print only the stack's README")
	cmd.Flags().BoolVar(&noReadme, "no-readme", false, "print only the ports, credentials and addresses")
	cmd.MarkFlagsMutuallyExclusive("readme", "no-readme")

	return cmd
}

// writeInfoHeader prints the facts that come from spinup rather than from the
// stack's prose: what it is, where it will listen, and how to reach it.
func writeInfoHeader(out io.Writer, p *prepared) {
	s := p.stack

	fmt.Fprintf(out, "%s %s\n", ui.Bold(s.Name), ui.Dim("("+string(s.Category)+")"))
	if s.Description != "" {
		fmt.Fprintf(out, "%s\n", s.Description)
	}
	fmt.Fprintln(out)

	field := func(label, value string) {
		if value != "" {
			fmt.Fprintf(out, "  %-9s %s\n", ui.Dim(label), value)
		}
	}

	url := catalogExpand(s.URL, p)
	field("url", url)

	// In nginx-static, nginx-proxy-manager and pytorch the GUI *is* the primary
	// service, so its address is the connection string; printing both would say
	// the same thing twice.
	if s.HasGUI() {
		if gui := catalogExpand(s.GUI.URL, p); gui != "" && gui != url {
			if login := catalogExpand(s.GUI.Login, p); login != "" && login != "none" {
				gui = fmt.Sprintf("%s  (%s)", gui, login)
			}
			field("gui", gui)
		}
	}

	// One line for every port, rather than one line each: the names are as long
	// as the label column, and a stack with four of them would push the rest of
	// the card out of alignment.
	if len(s.Ports) > 0 {
		ports := make([]string, 0, len(s.Ports))
		for _, port := range s.Ports {
			value := p.env[port.Name]
			if value == "" {
				value = fmt.Sprint(port.Default)
			}
			ports = append(ports, fmt.Sprintf("%s %s", strings.ToLower(strings.TrimSuffix(port.Name, "_PORT")), value))
		}
		field("ports", strings.Join(ports, ", "))
	}

	field("primary", s.Primary)
	if s.CLI != "" {
		field("client", "spinup cli "+s.Name)
	}
	if len(s.Profiles) > 0 {
		field("profiles", strings.Join(s.Profiles, ", "))
	}
	field("origin", string(s.Origin))
	field("files", p.paths.StackDir(s.Name))
	field("env", p.paths.EnvFile(s.Name))
}
