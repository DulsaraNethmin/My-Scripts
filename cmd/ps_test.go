package cmd

import (
	"strings"
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/compose"
)

// compose reports one publisher per address family, so a port bound on both
// IPv4 and IPv6 arrives twice.
func TestPortsCellDeduplicates(t *testing.T) {
	c := compose.Container{Publishers: []compose.Publisher{
		{URL: "0.0.0.0", PublishedPort: 16380, TargetPort: 6379, Protocol: "tcp"},
		{URL: "::", PublishedPort: 16380, TargetPort: 6379, Protocol: "tcp"},
		{URL: "0.0.0.0", PublishedPort: 18083, TargetPort: 5540, Protocol: "tcp"},
	}}

	if got := portsCell(c); got != "16380->6379, 18083->5540" {
		t.Errorf("portsCell = %q", got)
	}
}

// An unpublished port has nothing to show.
func TestPortsCellSkipsUnpublished(t *testing.T) {
	c := compose.Container{Publishers: []compose.Publisher{{TargetPort: 6379}}}
	if got := portsCell(c); got != "" {
		t.Errorf("portsCell = %q, want empty", got)
	}
}

func TestStatusCell(t *testing.T) {
	for status, want := range map[string]string{
		"Up 35 seconds (healthy)":         "Up 35 seconds",
		"Up 2 minutes (unhealthy)":        "Up 2 minutes",
		"Up 3 hours":                      "Up 3 hours",
		"Exited (0) 4 minutes ago":        "Exited (0) 4 minutes ago", // an exit code is worth keeping
		"Up 4 seconds (health: starting)": "Up 4 seconds",
	} {
		if got := statusCellFor(compose.Container{Status: status}); got != want {
			t.Errorf("statusCellFor(%q) = %q, want %q", status, got, want)
		}
	}
}

func TestHealthCell(t *testing.T) {
	for name, tc := range map[string]struct {
		c    compose.Container
		want string
	}{
		"healthy":        {compose.Container{State: "running", Health: "healthy"}, "healthy"},
		"no healthcheck": {compose.Container{State: "running"}, "no check"},
		"unhealthy":      {compose.Container{State: "running", Health: "unhealthy"}, "unhealthy"},
		"stopped":        {compose.Container{State: "exited"}, "-"},
	} {
		if got := healthCell(tc.c); !strings.Contains(got, tc.want) {
			t.Errorf("%s: healthCell = %q, want it to contain %q", name, got, tc.want)
		}
	}
}
