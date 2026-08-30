package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func newURLCmd() *cobra.Command {
	var gui bool

	cmd := &cobra.Command{
		Use:   "url <stack>",
		Short: "Print a stack's connection string",
		Long: "url prints the connection string a client needs, with the stack's own\n" +
			"ports and credentials filled in — the thing people otherwise go digging\n" +
			"through a compose file for.\n\n" +
			"--gui prints the address of the stack's web interface instead. Neither\n" +
			"needs Docker: the answer comes from the stack's environment, so it is the\n" +
			"same before and after the stack is running.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// No profile flags: which profiles are active changes what runs,
			// not what a client would connect to, and `url --gui` means "the
			// GUI's address", not "start the GUI".
			p, err := prepare(cmd.Context(), args[0], profileFlags{})
			if err != nil {
				return err
			}

			value, err := stackURL(p, gui)
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), value)
			return nil
		},
	}

	cmd.Flags().BoolVar(&gui, "gui", false, "print the web interface's address instead")

	return cmd
}

// stackURL resolves the connection string or the GUI address, with the same
// wording for both kinds of "this stack does not have one".
func stackURL(p *prepared, gui bool) (string, error) {
	s := p.stack

	if gui {
		if !s.HasGUI() {
			return "", failf(ExitUsage, "%s has no web interface", s.Name)
		}
		url := catalogExpand(s.GUI.URL, p)
		if url == "" {
			return "", failf(ExitUsage, "%s does not say where its web interface is", s.Name)
		}
		return url, nil
	}

	url := catalogExpand(s.URL, p)
	if url == "" {
		return "", failf(ExitUsage, "%s has no connection string — try `spinup info %s`", s.Name, s.Name)
	}
	return url, nil
}

func newOpenCmd() *cobra.Command {
	var printOnly bool

	cmd := &cobra.Command{
		Use:   "open <stack>",
		Short: "Open a stack's web interface in the browser",
		Long: "open launches the stack's web interface — pgAdmin, phpMyAdmin, Jupyter,\n" +
			"whatever it ships — in your browser, with the login printed beside it.\n\n" +
			"The stack has to be running, and its GUI is behind the `gui` profile:\n" +
			"`spinup up <stack> --gui` if you started it without one.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			p, err := prepare(cmd.Context(), args[0], profileFlags{})
			if err != nil {
				return err
			}

			url, err := stackURL(p, true)
			if err != nil {
				return err
			}

			fmt.Fprintln(out, url)
			if login := catalogExpand(p.stack.GUI.Login, p); login != "" && login != "none" {
				fmt.Fprintf(out, "%s %s\n", ui.Dim("login"), login)
			}
			if printOnly {
				return nil
			}

			if err := openInBrowser(cmd, url); err != nil {
				// Not being able to launch a browser is not a failed command —
				// the address is on screen and can be pasted.
				fmt.Fprintf(cmd.ErrOrStderr(), "%s could not open a browser: %v\n", ui.Warn("warning:"), err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&printOnly, "print", false, "print the address without opening a browser")

	return cmd
}

// browserCommand is the platform's "open this in whatever handles it".
func browserCommand(url string) (string, []string) {
	switch runtime.GOOS {
	case "darwin":
		return "open", []string{url}
	case "windows":
		// The empty string is start's window-title argument: without it, a URL
		// in quotes is taken as the title and nothing opens.
		return "cmd", []string{"/c", "start", "", url}
	default:
		return "xdg-open", []string{url}
	}
}

func openInBrowser(cmd *cobra.Command, url string) error {
	name, args := browserCommand(url)

	launch := exec.CommandContext(cmd.Context(), name, args...)
	launch.Stdout = cmd.ErrOrStderr()
	launch.Stderr = cmd.ErrOrStderr()

	return launch.Run()
}
