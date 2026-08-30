//go:build integration

package compose_test

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/compose"
	"github.com/DulsaraNethmin/spinup/internal/docker"
)

// freePort asks the kernel for a port nothing is using, so the test cannot
// collide with the developer's own containers.
func freePort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close() //nolint:errcheck // test cleanup

	return l.Addr().(*net.TCPAddr).Port
}

// The full path a stack takes: materialise it out of the catalog, start it
// through the wrapper, and confirm compose reports it healthy. redis is the
// lightest stack in the catalog, which is why it is the one CI runs.
func TestUpRedis(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := docker.New().Available(ctx); err != nil {
		t.Skipf("docker is not available: %v", err)
	}

	// A project name of its own, so this can never touch a stack the developer
	// is running.
	const stack = "redis-itest"

	home := t.TempDir()
	dir := filepath.Join(home, "stacks", stack)
	envFile := filepath.Join(home, "env", stack+".env")

	cat := catalog.New(os.DirFS(filepath.Join("..", "..", "stacks")))
	if _, err := cat.Materialize("redis", dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if _, err := cat.SeedEnv("redis", envFile); err != nil {
		t.Fatalf("SeedEnv: %v", err)
	}

	port := freePort(t)
	r := compose.New(os.Stdout, os.Stderr)
	r.Env = []string{"REDIS_PORT=" + strconv.Itoa(port)}

	p := compose.Project{Stack: stack, Dir: dir, EnvFile: envFile}

	t.Cleanup(func() {
		// Destroy, not down: a test must not leave a volume behind.
		if err := r.Down(context.Background(), p, compose.DownOptions{Volumes: true, RemoveOrphans: true}); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	if err := r.Up(ctx, p, compose.UpOptions{Wait: true, Timeout: 3 * time.Minute}); err != nil {
		t.Fatalf("Up: %v", err)
	}

	containers, err := r.PS(ctx, p)
	if err != nil {
		t.Fatalf("PS: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("PS returned %d containers, want 1: %+v", len(containers), containers)
	}

	c := containers[0]
	if !c.Healthy() {
		t.Errorf("redis is not healthy: state=%q health=%q status=%q", c.State, c.Health, c.Status)
	}
	if !strings.HasPrefix(c.Name, compose.ProjectPrefix+stack) {
		t.Errorf("container %q is not in the %s project", c.Name, p.Name())
	}
	if len(c.Publishers) == 0 || c.Publishers[0].PublishedPort != port {
		t.Errorf("published ports = %+v, want %d", c.Publishers, port)
	}
}

// down keeps the data, destroy removes it. The old mysql-stop.sh in this repo
// had these the wrong way round and deleted its database on every stop.
func TestDownKeepsVolumesAndDestroyRemovesThem(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := docker.New().Available(ctx); err != nil {
		t.Skipf("docker is not available: %v", err)
	}

	const stack = "redis-vtest"

	home := t.TempDir()
	dir := filepath.Join(home, "stacks", stack)
	envFile := filepath.Join(home, "env", stack+".env")

	cat := catalog.New(os.DirFS(filepath.Join("..", "..", "stacks")))
	if _, err := cat.Materialize("redis", dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if _, err := cat.SeedEnv("redis", envFile); err != nil {
		t.Fatalf("SeedEnv: %v", err)
	}

	r := compose.New(os.Stdout, os.Stderr)
	r.Env = []string{"REDIS_PORT=" + strconv.Itoa(freePort(t))}
	p := compose.Project{Stack: stack, Dir: dir, EnvFile: envFile}

	t.Cleanup(func() {
		_ = r.Down(context.Background(), p, compose.DownOptions{Volumes: true})
	})

	if err := r.Up(ctx, p, compose.UpOptions{Wait: true, Timeout: 3 * time.Minute}); err != nil {
		t.Fatalf("Up: %v", err)
	}

	if err := r.Down(ctx, p, compose.DownOptions{}); err != nil {
		t.Fatalf("Down: %v", err)
	}
	if n := volumeCount(t, p.Name()); n == 0 {
		t.Error("plain down deleted the stack's volumes")
	}

	if err := r.Down(ctx, p, compose.DownOptions{Volumes: true}); err != nil {
		t.Fatalf("Down --volumes: %v", err)
	}
	if n := volumeCount(t, p.Name()); n != 0 {
		t.Errorf("down --volumes left %d volume(s) behind", n)
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
