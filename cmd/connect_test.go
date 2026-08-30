package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// runCmd executes one command against the fixture catalog and a temporary
// ~/.spinup, which is all url, open and info need — none of them touches
// Docker.
func runCmd(t *testing.T, c *cobra.Command, args ...string) (string, string, error) {
	t.Helper()

	ctx, _ := prepareCtx(t)

	var out, errOut bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errOut)
	c.SetArgs(args)

	err := c.ExecuteContext(ctx)
	return out.String(), errOut.String(), err
}

// url is meant to be substituted into a command, so it prints the connection
// string and nothing else.
func TestURL(t *testing.T) {
	out, _, err := runCmd(t, newURLCmd(), "postgres")
	if err != nil {
		t.Fatalf("url: %v", err)
	}
	if got, want := strings.TrimSpace(out), "postgres://spinup@localhost:5432/spinup"; got != want {
		t.Errorf("url = %q, want %q", got, want)
	}
}

func TestURLGUI(t *testing.T) {
	out, _, err := runCmd(t, newURLCmd(), "postgres", "--gui")
	if err != nil {
		t.Fatalf("url --gui: %v", err)
	}
	if got, want := strings.TrimSpace(out), "http://localhost:8080"; got != want {
		t.Errorf("url --gui = %q, want %q", got, want)
	}
}

func TestURLOfAnUnknownStack(t *testing.T) {
	_, _, err := runCmd(t, newURLCmd(), "nosuchstack")
	if err == nil {
		t.Fatal("url of an unknown stack: want an error")
	}
	if got := codeFor(err); got != ExitNotFound {
		t.Errorf("exit code %d, want %d", got, ExitNotFound)
	}
}

// --print is what a headless machine or a script uses: the address, the login,
// and no browser.
func TestOpenPrint(t *testing.T) {
	out, _, err := runCmd(t, newOpenCmd(), "postgres", "--print")
	if err != nil {
		t.Fatalf("open --print: %v", err)
	}
	for _, want := range []string{"http://localhost:8080", "admin@example.com / spinup"} {
		if !strings.Contains(out, want) {
			t.Errorf("open --print does not show %q:\n%s", want, out)
		}
	}
}

func TestInfo(t *testing.T) {
	out, _, err := runCmd(t, newInfoCmd(), "postgres")
	if err != nil {
		t.Fatalf("info: %v", err)
	}

	for _, want := range []string{
		"postgres",
		"PostgreSQL 16 with pgAdmin",
		"postgres://spinup@localhost:5432/spinup", // the connection string, expanded
		"http://localhost:8080",                   // the GUI
		"postgres 5432, pgadmin 8080",             // every port on one line
		"# postgres",                              // the README, after the card
	} {
		if !strings.Contains(out, want) {
			t.Errorf("info does not show %q:\n%s", want, out)
		}
	}
}

func TestInfoReadmeOnly(t *testing.T) {
	out, _, err := runCmd(t, newInfoCmd(), "postgres", "--readme")
	if err != nil {
		t.Fatalf("info --readme: %v", err)
	}
	if got := strings.TrimSpace(out); got != "# postgres" {
		t.Errorf("info --readme = %q, want just the README", got)
	}
}

func TestInfoWithoutReadme(t *testing.T) {
	out, _, err := runCmd(t, newInfoCmd(), "postgres", "--no-readme")
	if err != nil {
		t.Fatalf("info --no-readme: %v", err)
	}
	if strings.Contains(out, "# postgres") {
		t.Errorf("info --no-readme printed the README:\n%s", out)
	}
	if !strings.Contains(out, "5432") {
		t.Errorf("info --no-readme dropped the card too:\n%s", out)
	}
}

// The cli template is split before it is expanded, so a password with a space
// stays one argument and one with a semicolon is not a shell injection: no
// shell ever sees it.
func TestClientCommand(t *testing.T) {
	env := map[string]string{
		"USER": "spinup",
		"DB":   "app db",
		"PASS": "p@ss word; rm -rf /",
	}

	got := clientCommand("psql -U ${USER} -d ${DB} --password=${PASS}", env)
	want := []string{"psql", "-U", "spinup", "-d", "app db", "--password=p@ss word; rm -rf /"}

	if len(got) != len(want) {
		t.Fatalf("clientCommand = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("argument %d = %q, want %q", i, got[i], want[i])
		}
	}

	if got := clientCommand("", nil); len(got) != 0 {
		t.Errorf("clientCommand of an empty template = %q, want nothing", got)
	}
}

func TestBrowserCommand(t *testing.T) {
	name, args := browserCommand("http://localhost:8080")
	if name == "" {
		t.Fatal("browserCommand returned no command")
	}

	// Whatever the platform, the address has to be in there — a launcher that
	// drops it opens a blank browser.
	if !strings.Contains(strings.Join(args, " "), "http://localhost:8080") {
		t.Errorf("%s %q does not carry the URL", name, args)
	}
}
