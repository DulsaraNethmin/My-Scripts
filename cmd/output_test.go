package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/compose"
)

// --json is an interface other programs read, so what matters is that it
// parses, that the ports are the resolved ones, and that the connection string
// is the same one `spin url` prints.
func TestListJSON(t *testing.T) {
	out, _, err := runCmd(t, newListCmd(), "--json")
	if err != nil {
		t.Fatalf("list --json: %v", err)
	}

	var stacks []stackJSON
	if err := json.Unmarshal([]byte(out), &stacks); err != nil {
		t.Fatalf("list --json is not JSON: %v\n%s", err, out)
	}
	if len(stacks) != 1 {
		t.Fatalf("list --json returned %d stacks, want 1", len(stacks))
	}

	s := stacks[0]
	if s.Name != "postgres" || s.Category != "database" {
		t.Errorf("stack = %+v", s)
	}
	if s.Ports["POSTGRES_PORT"] != 5432 || s.Ports["PGADMIN_PORT"] != 8080 {
		t.Errorf("ports = %v, want the resolved host ports", s.Ports)
	}
	if s.URL != "postgres://spinup@localhost:5432/spinup" {
		t.Errorf("url = %q — the credentials come from .env.example and have to be resolved", s.URL)
	}
	if s.GUI != "http://localhost:8080" {
		t.Errorf("gui = %q", s.GUI)
	}
}

// The table and --json must not both be printed, and -q means something else
// entirely, so asking for both is a usage error rather than a guess.
func TestListJSONAndQuietAreExclusive(t *testing.T) {
	if _, _, err := runCmd(t, newListCmd(), "--json", "--quiet"); err == nil {
		t.Fatal("list --json --quiet was accepted")
	}
}

func TestContainerToJSON(t *testing.T) {
	got := containerToJSON("redis", compose.Container{
		Name:    "spinup-redis-redis-1",
		Service: "redis",
		Image:   "redis:7-alpine",
		State:   "running",
		Status:  "Up 6 seconds (healthy)",
		Health:  "healthy",
		Publishers: []compose.Publisher{
			// compose reports one publisher per address family; the same port
			// twice is one port.
			{PublishedPort: 6379, TargetPort: 6379, Protocol: "tcp"},
			{PublishedPort: 6379, TargetPort: 6379, Protocol: "tcp"},
			{PublishedPort: 0, TargetPort: 9999}, // unpublished: not a host port
		},
	})

	if !got.Running || !got.Healthy {
		t.Errorf("running = %v, healthy = %v; want both true", got.Running, got.Healthy)
	}
	if len(got.Ports) != 1 || got.Ports[0].Published != 6379 {
		t.Errorf("ports = %+v, want one published 6379", got.Ports)
	}

	// An encoded empty result has to be [], not null: a script that iterates
	// the answer should not have to special-case "nothing is running".
	data, err := json.Marshal(containerToJSON("redis", compose.Container{}).Ports)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "[]" {
		t.Errorf("empty ports encode as %s, want []", data)
	}
}

func TestRunningServices(t *testing.T) {
	got := runningServices([]compose.Container{
		{Service: "redis", State: "running", Publishers: []compose.Publisher{
			{PublishedPort: 16399, TargetPort: 6379},
			{PublishedPort: 16399, TargetPort: 6379},
		}},
		{Service: "redisinsight", State: "running", Publishers: []compose.Publisher{
			{PublishedPort: 8083, TargetPort: 5540},
		}},
		{Service: "stopped-thing", State: "exited"},
	})

	if want := "redis 16399, redisinsight 8083"; got != want {
		t.Errorf("runningServices = %q, want %q", got, want)
	}

	// A service with no published port is still worth naming: it came up.
	if got := runningServices([]compose.Container{{Service: "worker", State: "running"}}); got != "worker" {
		t.Errorf("runningServices = %q, want %q", got, "worker")
	}
}

// --port is the one-run escape hatch for a port that is already taken. A name
// the stack does not declare must be refused: compose would accept it silently
// and bind the port the user was trying to change.
func TestPortOverrides(t *testing.T) {
	ctx, _ := prepareCtx(t)

	p, err := prepare(ctx, "postgres", profileFlags{})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	env, err := portOverrides(p, []string{"POSTGRES_PORT=15432"})
	if err != nil {
		t.Fatalf("portOverrides: %v", err)
	}
	if len(env) != 1 || env[0] != "POSTGRES_PORT=15432" {
		t.Errorf("env = %v, want [POSTGRES_PORT=15432]", env)
	}

	// The resolved environment has to move too, or the summary and the
	// connection string would advertise the old port.
	if p.env["POSTGRES_PORT"] != "15432" {
		t.Errorf("the stack's env still says %q", p.env["POSTGRES_PORT"])
	}
	if url := catalogExpand(p.stack.URL, p); !strings.Contains(url, "15432") {
		t.Errorf("url = %q, want the overridden port", url)
	}

	for name, flag := range map[string]string{
		"an unknown port": "NOPE=1234",
		"no value":        "POSTGRES_PORT=",
		"no equals":       "POSTGRES_PORT",
		"not a number":    "POSTGRES_PORT=abc",
		"out of range":    "POSTGRES_PORT=70000",
	} {
		if _, err := portOverrides(p, []string{flag}); err == nil {
			t.Errorf("%s: --port %s was accepted", name, flag)
		}
	}

	if env, err := portOverrides(p, nil); err != nil || env != nil {
		t.Errorf("portOverrides with no flags = %v, %v; want nothing", env, err)
	}
}
