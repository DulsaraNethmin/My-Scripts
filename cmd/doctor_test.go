package cmd

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/config"
	"github.com/DulsaraNethmin/spinup/internal/docker"
)

// fakeDocker answers docker invocations from a table keyed by joined args.
type fakeDocker struct {
	out map[string]string
	err map[string]error
}

func (f fakeDocker) Output(_ context.Context, _ string, args ...string) ([]byte, error) {
	key := strings.Join(args, " ")
	if err, ok := f.err[key]; ok {
		return nil, err
	}
	return []byte(f.out[key]), nil
}

func healthyDocker() fakeDocker {
	return fakeDocker{out: map[string]string{
		"version --format {{.Client.Version}}": "28.5.1",
		"version --format {{.Server.Version}}": "28.5.1",
		"compose version --short":              "2.40.2",
		"compose up --help":                    "Usage: docker compose up\n  --wait\n  --wait-timeout int\n",
		"info --format {{json .Runtimes}}":     `{"io.containerd.runc.v2":{},"runc":{}}`,
	}}
}

// freePort returns a port nothing is listening on, by taking one and letting
// it go. Good enough for a check whose whole answer is "is anything there".
func freeDoctorPort(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return strconv.Itoa(port)
}

func runDoctor(t *testing.T, f fakeDocker) (string, error) {
	t.Helper()

	var out bytes.Buffer
	c := newDoctorCmd(docker.NewWith(f))
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetArgs([]string{}) // nil would make cobra fall back to os.Args, i.e. the go test flags

	stacks := fstest.MapFS{
		"postgres/spinup.yaml": &fstest.MapFile{Data: []byte(
			"name: postgres\ndescription: PostgreSQL 16\ncategory: database\nprimary: postgres\n" +
				"url: postgres://localhost:${POSTGRES_PORT}\nports:\n  - name: POSTGRES_PORT\n    default: 5432\n")},
	}
	ctx := catalog.NewContext(context.Background(), catalog.New(stacks))

	// The stack's port is resolved from the environment, so the port check has
	// something predictable to probe rather than whatever this machine happens
	// to be running.
	if os.Getenv("POSTGRES_PORT") == "" {
		t.Setenv("POSTGRES_PORT", freeDoctorPort(t))
	}

	// Run first: a return statement evaluates its operands left to right, so
	// out.String() would otherwise be read before the command has written to it.
	err := c.ExecuteContext(ctx)
	return out.String(), err
}

func TestDoctorHealthy(t *testing.T) {
	t.Setenv(config.HomeEnv, t.TempDir())

	out, err := runDoctor(t, healthyDocker())
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	for _, want := range []string{"docker", "daemon", "compose", "catalog", "1 stacks", "everything checks out"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
}

// Exit 2 is what a script checks to find out that Docker, not spinup, is the
// problem.
func TestDoctorExitsTwoWithoutADaemon(t *testing.T) {
	t.Setenv(config.HomeEnv, t.TempDir())

	f := healthyDocker()
	f.err = map[string]error{"version --format {{.Server.Version}}": os.ErrNotExist}

	out, err := runDoctor(t, f)
	if err == nil {
		t.Fatalf("want a failure:\n%s", out)
	}
	if got := codeFor(err); got != ExitDocker {
		t.Errorf("exit code = %d, want %d", got, ExitDocker)
	}
	if !strings.Contains(out, "not running") {
		t.Errorf("output does not say the daemon is down:\n%s", out)
	}
}

// A broken config.yaml is the user's mistake, not Docker's: exit 1, and the
// message has to name the file and the keys that exist.
func TestDoctorReportsABrokenConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv(config.HomeEnv, home)
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte("guy: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runDoctor(t, healthyDocker())
	if err == nil {
		t.Fatalf("want a failure:\n%s", out)
	}
	if got := codeFor(err); got != ExitUsage {
		t.Errorf("exit code = %d, want %d", got, ExitUsage)
	}
	if !strings.Contains(out, "known keys: gui") {
		t.Errorf("output does not list the valid keys:\n%s", out)
	}
	// Details must stay on one line or the column layout falls apart.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "field guy not found") && !strings.Contains(line, "fail") {
			t.Errorf("the config error wrapped onto its own line:\n%s", out)
		}
	}
}

// The port check is what answers "why will this not start": something else is
// already on the port the stack wants.
func TestDoctorReportsAPortInUse(t *testing.T) {
	t.Setenv(config.HomeEnv, t.TempDir())

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close() //nolint:errcheck // held only for the length of the test

	port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	t.Setenv("POSTGRES_PORT", port)

	out, err := runDoctor(t, healthyDocker())
	if err != nil {
		// A port in use is a warning, not a failure: the stack may not be one
		// the user is about to start.
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	if !strings.Contains(out, port) || !strings.Contains(out, "in use") {
		t.Errorf("doctor does not report %s as in use:\n%s", port, out)
	}
	if !strings.Contains(out, "1 warning") {
		t.Errorf("the summary does not acknowledge the warning:\n%s", out)
	}
}

// A machine with an NVIDIA driver but no nvidia runtime in docker is a setup
// someone meant to finish; a machine with neither is just a machine.
func TestDoctorGPU(t *testing.T) {
	t.Setenv(config.HomeEnv, t.TempDir())

	f := healthyDocker()
	f.out["info --format {{json .Runtimes}}"] = `{"nvidia":{"path":"nvidia-container-runtime"},"runc":{}}`

	out, err := runDoctor(t, f)
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	if !strings.Contains(out, "nvidia runtime available") {
		t.Errorf("doctor does not report the nvidia runtime:\n%s", out)
	}
}

// Every `spin up` passes --wait-timeout, so a compose plugin without it is
// worth saying out loud before the first command fails.
func TestDoctorReportsComposeWithoutWaitTimeout(t *testing.T) {
	t.Setenv(config.HomeEnv, t.TempDir())

	f := healthyDocker()
	f.out["compose up --help"] = "Usage: docker compose up\n  --wait\n"

	out, err := runDoctor(t, f)
	if err != nil {
		t.Fatalf("doctor: %v\n%s", err, out)
	}
	if !strings.Contains(out, "--wait-timeout") {
		t.Errorf("doctor does not mention the missing flag:\n%s", out)
	}
}
