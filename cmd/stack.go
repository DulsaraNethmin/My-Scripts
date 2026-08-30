package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/compose"
	"github.com/DulsaraNethmin/spinup/internal/config"
	"github.com/DulsaraNethmin/spinup/internal/docker"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

// dockerTimeout bounds the readiness check, not the commands themselves — an
// image pull can take as long as it takes.
const dockerTimeout = 30 * time.Second

// prepared is a stack ready to hand to compose: its metadata, its materialised
// files, its environment and the compose project that runs it.
type prepared struct {
	stack   *catalog.Stack
	project compose.Project
	env     config.Env
	paths   config.Paths
}

// profileFlags are the profile-selecting flags the lifecycle commands share.
type profileFlags struct {
	gui   bool
	noGUI bool
	gpu   bool
}

func (f *profileFlags) register(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.gui, "gui", false, "start the stack's web interface")
	cmd.Flags().BoolVar(&f.noGUI, "no-gui", false, "do not start the stack's web interface")
	cmd.Flags().BoolVar(&f.gpu, "gpu", false, "use the stack's GPU variant, if it has one")
	cmd.MarkFlagsMutuallyExclusive("gui", "no-gui")
}

// prepare gets a stack ready to run: it materialises the stack into
// ~/.spinup/stacks, seeds its env file, resolves its environment and works out
// which compose profiles to enable.
//
// Materialising on every command rather than only the first is deliberate — a
// user who deletes ~/.spinup gets it back, and one who edits a file keeps their
// edit, because nothing is ever overwritten.
func prepare(ctx context.Context, name string, flags profileFlags) (*prepared, error) {
	cat, ok := catalog.FromContext(ctx)
	if !ok {
		return nil, errors.New("no catalog: this is a bug in spinup")
	}

	stack, err := cat.Load(name)
	if err != nil {
		if errors.Is(err, catalog.ErrNotFound) {
			return nil, failf(ExitNotFound, "no stack called %q — run `spinup list` to see them", name)
		}
		return nil, err
	}

	paths, err := config.DefaultPaths()
	if err != nil {
		return nil, err
	}
	if err := paths.Ensure(); err != nil {
		return nil, err
	}

	cfg, err := config.LoadConfig(paths.ConfigFile())
	if err != nil {
		return nil, err
	}

	dir := paths.StackDir(name)
	if _, err := cat.Materialize(name, dir); err != nil {
		return nil, err
	}

	envFile := paths.EnvFile(name)
	if _, err := cat.SeedEnv(name, envFile); err != nil {
		return nil, err
	}

	example, err := cat.ReadFile(name, catalog.EnvExample)
	if err != nil {
		return nil, err
	}
	env, err := config.ResolveEnv(stack, example, envFile)
	if err != nil {
		return nil, err
	}

	return &prepared{
		stack: stack,
		env:   env,
		paths: paths,
		project: compose.Project{
			Stack:    name,
			Dir:      dir,
			EnvFile:  envFile,
			Profiles: profilesFor(stack, cfg, flags),
		},
	}, nil
}

// profilesFor decides which compose profiles to enable.
//
// The starting point is the stack's own default_profiles, which exists for
// stacks like pytorch where every service sits behind a profile and nothing
// would start at all. --gpu swaps those defaults for the stack's GPU profile,
// since the two variants share ports and cannot both run.
func profilesFor(s *catalog.Stack, cfg config.Config, flags profileFlags) []string {
	profiles := slices.Clone(s.DefaultProfiles)

	if flags.gpu && s.HasGPU() {
		profiles = []string{s.GPU.Profile}
	}

	// A GUI in its own container is behind the gui profile; one served by the
	// primary service is already running and has no profile to enable.
	wantGUI := cfg.GUI
	if flags.gui {
		wantGUI = true
	}
	if flags.noGUI {
		wantGUI = false
	}
	if wantGUI && s.HasGUI() && s.HasProfile("gui") && !slices.Contains(profiles, "gui") {
		profiles = append(profiles, "gui")
	}

	return profiles
}

// requireDocker fails with exit code 2 unless docker, its daemon and Compose
// v2 are all there. Every command that runs compose starts here, so the user
// gets one clear message instead of whatever compose would have said.
func requireDocker(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, dockerTimeout)
	defer cancel()

	if err := docker.New().Available(ctx); err != nil {
		return failf(ExitDocker, "%s (run `spinup doctor` for details)", err)
	}
	return nil
}

// runCompose maps a compose failure to exit code 4, so a script can tell "your
// stack did not start" from "you asked for a stack that does not exist".
func runCompose(err error) error {
	var cerr *compose.Error
	if errors.As(err, &cerr) {
		return failf(ExitCompose, "%s", cerr)
	}
	return err
}

// splitDashArgs separates the stack names from anything after a --, which is
// passed straight through to docker compose.
func splitDashArgs(cmd *cobra.Command, args []string) (stacks, passthrough []string) {
	at := cmd.ArgsLenAtDash()
	if at < 0 {
		return args, nil
	}
	return args[:at], args[at:]
}

// heading announces which stack is being worked on, for the multi-stack case.
func heading(out io.Writer, name string) {
	fmt.Fprintf(out, "\n%s %s\n", ui.Dim("=>"), ui.Bold(name))
}
