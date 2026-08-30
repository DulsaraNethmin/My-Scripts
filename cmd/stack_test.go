package cmd

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/config"
)

func parse(t *testing.T, name, yaml string) *catalog.Stack {
	t.Helper()

	s, err := catalog.ParseStack(name, []byte(yaml))
	if err != nil {
		t.Fatalf("parsing the %s fixture: %v", name, err)
	}
	return s
}

// The three shapes in the real catalog: a GUI in its own container, a GUI that
// is the primary service, and a stack where everything is behind a profile.
func fixtures(t *testing.T) (postgres, nginx, pytorch *catalog.Stack) {
	t.Helper()

	postgres = parse(t, "postgres", `name: postgres
description: PostgreSQL 16 with pgAdmin
category: database
primary: postgres
gui:
  service: pgadmin
  url: http://localhost:${PGADMIN_PORT}
  login: admin
url: postgres://localhost:${POSTGRES_PORT}
ports:
  - name: POSTGRES_PORT
    default: 5432
profiles: [gui]
`)

	nginx = parse(t, "nginx-static", `name: nginx-static
description: nginx serving a static site
category: web
primary: nginx
gui:
  service: nginx
  url: http://localhost:${NGINX_PORT}
  login: none
url: http://localhost:${NGINX_PORT}
ports:
  - name: NGINX_PORT
    default: 8090
profiles: []
`)

	pytorch = parse(t, "pytorch", `name: pytorch
description: PyTorch with JupyterLab
category: ml
primary: jupyter
url: http://localhost:${JUPYTER_PORT}
ports:
  - name: JUPYTER_PORT
    default: 8888
profiles: [cpu, gpu]
default_profiles: [cpu]
gpu:
  profile: gpu
  service: jupyter-gpu
`)

	return postgres, nginx, pytorch
}

func TestProfilesForGUI(t *testing.T) {
	postgres, nginx, _ := fixtures(t)

	guiOn := config.Config{GUI: true}
	guiOff := config.Config{GUI: false}

	for name, tc := range map[string]struct {
		stack *catalog.Stack
		cfg   config.Config
		flags profileFlags
		want  []string
	}{
		"gui on by default":     {postgres, guiOn, profileFlags{}, []string{"gui"}},
		"--no-gui overrides":    {postgres, guiOn, profileFlags{noGUI: true}, nil},
		"config off":            {postgres, guiOff, profileFlags{}, nil},
		"--gui overrides off":   {postgres, guiOff, profileFlags{gui: true}, []string{"gui"}},
		"gui is the service":    {nginx, guiOn, profileFlags{}, nil},
		"--gui on a bare stack": {nginx, guiOn, profileFlags{gui: true}, nil},
	} {
		got := profilesFor(tc.stack, tc.cfg, tc.flags)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: profiles = %v, want %v", name, got, tc.want)
		}
	}
}

// pytorch is the reason default_profiles exists: both its services are behind
// profiles and they share ports, so with none selected nothing starts, and
// with both selected the second one cannot bind.
func TestProfilesForGPU(t *testing.T) {
	_, _, pytorch := fixtures(t)
	cfg := config.Config{GUI: true}

	if got := profilesFor(pytorch, cfg, profileFlags{}); !slices.Equal(got, []string{"cpu"}) {
		t.Errorf("default profiles = %v, want [cpu]", got)
	}

	got := profilesFor(pytorch, cfg, profileFlags{gpu: true})
	if !slices.Equal(got, []string{"gpu"}) {
		t.Errorf("--gpu profiles = %v, want [gpu] — cpu and gpu cannot both run", got)
	}
}

// --gpu on a stack with no GPU variant should do nothing rather than invent a
// profile that does not exist.
func TestProfilesForGPUOnAStackWithout(t *testing.T) {
	postgres, _, _ := fixtures(t)

	got := profilesFor(postgres, config.Config{GUI: true}, profileFlags{gpu: true})
	if !slices.Equal(got, []string{"gui"}) {
		t.Errorf("profiles = %v, want [gui]", got)
	}
}

func TestSplitDashArgs(t *testing.T) {
	cmd := &cobra.Command{Use: "up", Args: cobra.ArbitraryArgs, Run: func(*cobra.Command, []string) {}}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"postgres", "redis", "--", "--force-recreate"})

	var stacks, extra []string
	cmd.Run = func(c *cobra.Command, args []string) { stacks, extra = splitDashArgs(c, args) }
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(stacks, []string{"postgres", "redis"}) {
		t.Errorf("stacks = %v", stacks)
	}
	if !slices.Equal(extra, []string{"--force-recreate"}) {
		t.Errorf("passthrough = %v", extra)
	}
}

func TestSplitDashArgsWithoutADash(t *testing.T) {
	cmd := &cobra.Command{Use: "up", Args: cobra.ArbitraryArgs}
	cmd.SetArgs([]string{"postgres"})

	var stacks, extra []string
	cmd.Run = func(c *cobra.Command, args []string) { stacks, extra = splitDashArgs(c, args) }
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(stacks, []string{"postgres"}) || extra != nil {
		t.Errorf("stacks = %v, extra = %v", stacks, extra)
	}
}

// destroy is the only command that deletes data, so the default answer has to
// be no and an unanswerable prompt has to refuse rather than proceed.
func TestConfirmDestroy(t *testing.T) {
	for answer, want := range map[string]bool{
		"y\n":    true,
		"Y\n":    true,
		"yes\n":  true,
		"n\n":    false,
		"\n":     false,
		"maybe":  false,
		"delete": false,
	} {
		cmd := &cobra.Command{}
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetIn(strings.NewReader(answer))

		got, err := confirmDestroy(cmd, []string{"postgres"})
		if err != nil {
			t.Errorf("%q: %v", answer, err)
			continue
		}
		if got != want {
			t.Errorf("answer %q = %v, want %v", answer, got, want)
		}
	}
}

func TestConfirmDestroyWithNoInput(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetIn(strings.NewReader(""))

	ok, err := confirmDestroy(cmd, []string{"postgres"})
	if ok {
		t.Error("an empty stdin was taken as yes")
	}
	if err == nil {
		t.Fatal("want an error telling the user about -y")
	}
	if got := codeFor(err); got != ExitUsage {
		t.Errorf("exit code = %d, want %d", got, ExitUsage)
	}
}
