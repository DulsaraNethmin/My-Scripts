package docker_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/docker"
)

// fake answers docker invocations from a table keyed by the joined arguments.
type fake struct {
	out map[string]string
	err map[string]error
}

func (f fake) Output(_ context.Context, _ string, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	if err, ok := f.err[key]; ok {
		return nil, err
	}
	return []byte(f.out[key] + "\n"), nil
}

const (
	clientArgs  = "version --format {{.Client.Version}}"
	serverArgs  = "version --format {{.Server.Version}}"
	composeArgs = "compose version --short"
)

func healthy() fake {
	return fake{out: map[string]string{
		clientArgs:  "28.5.1",
		serverArgs:  "28.5.1",
		composeArgs: "2.40.2-desktop.1",
	}}
}

func TestVersions(t *testing.T) {
	c := docker.NewWith(healthy())
	ctx := context.Background()

	if v, err := c.ClientVersion(ctx); err != nil || v != "28.5.1" {
		t.Errorf("ClientVersion = %q, %v", v, err)
	}
	if v, err := c.ServerVersion(ctx); err != nil || v != "28.5.1" {
		t.Errorf("ServerVersion = %q, %v", v, err)
	}
	if v, err := c.ComposeVersion(ctx); err != nil || v != "2.40.2-desktop.1" {
		t.Errorf("ComposeVersion = %q, %v", v, err)
	}
}

// A daemon that is not running is the most common failure by far, and it has
// to be reported as itself rather than as whatever compose says next.
func TestDaemonNotRunning(t *testing.T) {
	f := healthy()
	f.err = map[string]error{serverArgs: errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock")}

	_, err := docker.NewWith(f).ServerVersion(context.Background())
	if !errors.Is(err, docker.ErrDaemonNotRunning) {
		t.Errorf("ServerVersion = %v, want ErrDaemonNotRunning", err)
	}
	// The underlying message is what tells the user which socket failed.
	if err == nil || !strings.Contains(err.Error(), "docker.sock") {
		t.Errorf("the daemon error lost docker's own message: %v", err)
	}
}

// The whole repo used to run on compose v1. Accepting it now would fail later,
// in the middle of an up, on profiles or a healthcheck condition.
func TestComposeV1IsRejected(t *testing.T) {
	f := healthy()
	f.out[composeArgs] = "1.29.2"

	v, err := docker.NewWith(f).ComposeVersion(context.Background())
	if !errors.Is(err, docker.ErrComposeMissing) {
		t.Fatalf("ComposeVersion = %q, %v; want ErrComposeMissing", v, err)
	}
	if v != "1.29.2" {
		t.Errorf("the version found should still be reported, got %q", v)
	}
	if !strings.Contains(err.Error(), "docker-compose") {
		t.Errorf("the error should name the v1 binary: %v", err)
	}
}

func TestComposeMissing(t *testing.T) {
	f := healthy()
	f.err = map[string]error{composeArgs: errors.New("docker: 'compose' is not a docker command")}

	if _, err := docker.NewWith(f).ComposeVersion(context.Background()); !errors.Is(err, docker.ErrComposeMissing) {
		t.Errorf("ComposeVersion = %v, want ErrComposeMissing", err)
	}
}

func TestComposeVersionSuffixes(t *testing.T) {
	// Docker Desktop, Linux packages and release candidates all format the
	// version differently; only the major number matters.
	for _, v := range []string{"2.40.2-desktop.1", "v2.29.0", "2.0.0", "3.0.0-rc.1"} {
		f := healthy()
		f.out[composeArgs] = v
		if _, err := docker.NewWith(f).ComposeVersion(context.Background()); err != nil {
			t.Errorf("ComposeVersion(%q) = %v", v, err)
		}
	}

	f := healthy()
	f.out[composeArgs] = "not-a-version"
	if _, err := docker.NewWith(f).ComposeVersion(context.Background()); !errors.Is(err, docker.ErrComposeMissing) {
		t.Error("an unparseable version should be reported as compose missing")
	}
}

func TestDiagnoseHealthy(t *testing.T) {
	checks := docker.NewWith(healthy()).Diagnose(context.Background())

	if !docker.OK(checks) {
		t.Errorf("healthy docker reported a failure: %+v", checks)
	}
	if len(checks) != 3 {
		t.Errorf("checks = %+v, want docker, daemon and compose", checks)
	}
	for _, c := range checks {
		if c.Detail == "" {
			t.Errorf("%s has no detail", c.Name)
		}
	}
}

// With no daemon there is nothing to ask about compose, and a second failure
// line would just be noise.
func TestDiagnoseStopsAtTheDaemon(t *testing.T) {
	f := healthy()
	f.err = map[string]error{serverArgs: errors.New("Cannot connect to the Docker daemon")}

	checks := docker.NewWith(f).Diagnose(context.Background())
	if docker.OK(checks) {
		t.Error("OK with no daemon")
	}
	if len(checks) != 2 || checks[1].Name != "daemon" {
		t.Fatalf("checks = %+v, want docker then daemon", checks)
	}
	if checks[1].Hint == "" {
		t.Error("the daemon failure has no hint about what to do")
	}
}

func TestDiagnoseComposeV1(t *testing.T) {
	f := healthy()
	f.out[composeArgs] = "1.29.2"

	checks := docker.NewWith(f).Diagnose(context.Background())
	if docker.OK(checks) {
		t.Error("OK with compose v1")
	}
	last := checks[len(checks)-1]
	if last.Name != "compose" || !strings.Contains(last.Detail, "1.29.2") {
		t.Errorf("last check = %+v, want the v1 version reported", last)
	}
}
