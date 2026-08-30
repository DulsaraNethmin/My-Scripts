//go:build integration

package config_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/config"
)

// spinup passes the same env file to docker compose that it parses here, so
// the two have to agree: if they don't, `spinup url` prints a port compose
// never bound. This asks compose itself what it resolved and compares.
func TestResolvedPortsMatchCompose(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker is not installed")
	}

	dirs, err := filepath.Glob(filepath.Join("..", "..", "stacks", "*"))
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no stacks found: %v", err)
	}

	for _, dir := range dirs {
		name := filepath.Base(dir)

		t.Run(name, func(t *testing.T) {
			meta, err := os.ReadFile(filepath.Join(dir, "spinup.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			stack, err := catalog.ParseStack(name, meta)
			if err != nil {
				t.Fatal(err)
			}

			example, err := os.ReadFile(filepath.Join(dir, catalog.EnvExample))
			if err != nil {
				t.Fatal(err)
			}
			env, err := config.ResolveEnv(stack, example, "")
			if err != nil {
				t.Fatalf("ResolveEnv: %v", err)
			}

			args := []string{
				"compose",
				"-f", filepath.Join(dir, "compose.yaml"),
				"--env-file", filepath.Join(dir, catalog.EnvExample),
			}
			for _, p := range stack.Profiles {
				args = append(args, "--profile", p)
			}
			args = append(args, "config", "--format", "json")

			out, err := exec.Command("docker", args...).Output()
			if err != nil {
				t.Fatalf("docker compose config: %v", err)
			}

			var parsed struct {
				Services map[string]struct {
					Ports []struct {
						Published string `json:"published"`
					} `json:"ports"`
				} `json:"services"`
			}
			if err := json.Unmarshal(out, &parsed); err != nil {
				t.Fatalf("decoding compose config: %v", err)
			}

			published := map[string]bool{}
			for _, svc := range parsed.Services {
				for _, p := range svc.Ports {
					published[p.Published] = true
				}
			}

			for _, p := range stack.Ports {
				got := env.Get(p.Name, "")
				if got == "" {
					t.Errorf("%s resolved to nothing", p.Name)
					continue
				}
				if !published[got] {
					t.Errorf("%s resolved to %s, but compose publishes %v",
						p.Name, got, keys(published))
				}
			}
		})
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
