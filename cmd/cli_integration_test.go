//go:build integration

package cmd

import (
	"bytes"
	"context"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/compose"
	"github.com/DulsaraNethmin/spinup/internal/config"
	"github.com/DulsaraNethmin/spinup/internal/docker"
	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func freePort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close() //nolint:errcheck // test cleanup

	return l.Addr().(*net.TCPAddr).Port
}

// installTestStack copies the redis stack into the user's stacks directory
// under a name of its own. That keeps the test off the compose project a
// developer may have running (spinup-redis), and exercises the user-stack
// overlay while it is at it.
func installTestStack(t *testing.T, home, name string) {
	t.Helper()

	src := os.DirFS(filepath.Join("..", "stacks", "redis"))
	dst := filepath.Join(home, "stacks", name)

	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		target := filepath.Join(dst, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		if p == "spinup.yaml" {
			data = bytes.Replace(data, []byte("name: redis\n"), []byte("name: "+name+"\n"), 1)
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("installing the test stack: %v", err)
	}
}

// The whole CLI, end to end, against a real daemon: up, ps, env, down and
// destroy, including the invariant that down keeps data and destroy does not.
func TestCLILifecycle(t *testing.T) {
	ctx := context.Background()
	if err := docker.New().Available(ctx); err != nil {
		t.Skipf("docker is not available: %v", err)
	}

	ui.SetColor(false)
	t.Cleanup(func() { ui.SetColor(true) })

	const stack = "redis-clitest"

	home := t.TempDir()
	t.Setenv(config.HomeEnv, home)
	t.Setenv("REDIS_PORT", strconv.Itoa(freePort(t)))
	t.Setenv("REDISINSIGHT_PORT", strconv.Itoa(freePort(t)))

	installTestStack(t, home, stack)

	cat := catalog.New(os.DirFS(filepath.Join("..", "stacks"))).
		WithUserStacks(os.DirFS(filepath.Join(home, "stacks")))
	ctx = catalog.NewContext(ctx, cat)

	run := func(args ...string) (string, error) {
		t.Helper()

		var out bytes.Buffer
		root := newRootCmd(Build{Version: "test"})
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetIn(strings.NewReader(""))
		root.SetArgs(args)

		err := root.ExecuteContext(ctx)
		return out.String(), err
	}

	t.Cleanup(func() {
		if _, err := run("destroy", "-y", stack); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	out, err := run("up", stack, "--no-gui")
	if err != nil {
		t.Fatalf("up: %v\n%s", err, out)
	}
	if !strings.Contains(out, "redis://") {
		t.Errorf("up did not print a connection string:\n%s", out)
	}

	out, err = run("ps", stack)
	if err != nil {
		t.Fatalf("ps: %v\n%s", err, out)
	}
	if !strings.Contains(out, "healthy") {
		t.Errorf("ps does not report the stack healthy:\n%s", out)
	}

	out, err = run("env", stack)
	if err != nil {
		t.Fatalf("env: %v\n%s", err, out)
	}
	if !strings.Contains(out, "REDIS_PASSWORD=") {
		t.Errorf("env did not print the stack's credentials:\n%s", out)
	}

	// list without --quiet needs the status column, which needs Docker.
	out, err = run("list")
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "running") {
		t.Errorf("list does not show the stack as running:\n%s", out)
	}

	// cli and shell exec into the running container. Their output goes to the
	// process's own stdout — an interactive client needs the real terminal, not
	// a buffer — so what is asserted here is that they reached the container
	// and came back cleanly.
	if out, err = run("cli", stack, "--", "ping"); err != nil {
		t.Fatalf("cli: %v\n%s", err, out)
	}
	if out, err = run("shell", stack, "--shell", "true"); err != nil {
		t.Fatalf("shell: %v\n%s", err, out)
	}

	project := compose.ProjectPrefix + stack

	if out, err = run("down", stack); err != nil {
		t.Fatalf("down: %v\n%s", err, out)
	}
	if n := volumeCount(t, project); n == 0 {
		t.Error("down deleted the stack's data")
	}

	// With the stack stopped, cli says so rather than passing compose's
	// "service is not running" through.
	if _, err := run("cli", stack); err == nil {
		t.Error("cli on a stopped stack succeeded")
	} else if !strings.Contains(err.Error(), "not running") {
		t.Errorf("cli on a stopped stack says: %v", err)
	}

	if out, err = run("destroy", "-y", stack); err != nil {
		t.Fatalf("destroy: %v\n%s", err, out)
	}
	if n := volumeCount(t, project); n != 0 {
		t.Errorf("destroy left %d volume(s) behind", n)
	}
}

// destroy without -y and with nothing to read from must refuse rather than
// treat silence as consent — a CI job piping into spinup must not lose data.
func TestCLIDestroyRefusesWithoutAnAnswer(t *testing.T) {
	home := t.TempDir()
	t.Setenv(config.HomeEnv, home)

	if err := docker.New().Available(context.Background()); err != nil {
		t.Skipf("docker is not available: %v", err)
	}

	cat := catalog.New(os.DirFS(filepath.Join("..", "stacks")))
	ctx := catalog.NewContext(context.Background(), cat)

	var out bytes.Buffer
	root := newRootCmd(Build{Version: "test"})
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(""))
	root.SetArgs([]string{"destroy", "redis"})

	err := root.ExecuteContext(ctx)
	if err == nil {
		t.Fatalf("destroy went ahead without an answer:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "-y") {
		t.Errorf("the error does not mention -y: %v", err)
	}
}

func volumeCount(t *testing.T, project string) int {
	t.Helper()

	out, err := exec.Command("docker", "volume", "ls", "--quiet", "--filter", "name="+project+"_").Output()
	if err != nil {
		t.Fatalf("docker volume ls: %v", err)
	}

	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}
