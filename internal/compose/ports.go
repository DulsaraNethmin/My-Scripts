package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

// HostPort is a host port a project would bind, and the service that binds it.
type HostPort struct {
	Port    int
	Service string
}

// HostPorts asks compose which host ports this project would bind.
//
// Compose is the authority rather than the stack's spinup.yaml, because only
// compose knows what the selected profiles and the resolved environment add up
// to: a GUI port behind a profile the user did not ask for is not in the
// answer, and neither is a port a --port override has moved. Reading the
// declared ports instead would refuse to start a stack over a port it was
// never going to bind.
func (r *Runner) HostPorts(ctx context.Context, p Project) ([]HostPort, error) {
	out, err := r.Config(ctx, p, "--format", "json")
	if err != nil {
		return nil, err
	}
	return parseHostPorts(out)
}

// composePort is one entry of a service's ports, as `config --format json`
// writes it. published is a string because compose also allows a range
// ("8000-8010") and an empty value, meaning "pick one".
type composePort struct {
	Target    int    `json:"target"`
	Published string `json:"published"`
	Protocol  string `json:"protocol"`
}

func parseHostPorts(out []byte) ([]HostPort, error) {
	var cfg struct {
		Services map[string]struct {
			Ports []composePort `json:"ports"`
		} `json:"services"`
	}
	if err := json.Unmarshal(out, &cfg); err != nil {
		return nil, fmt.Errorf("reading the compose config: %w", err)
	}

	seen := make(map[int]bool)
	var ports []HostPort
	for service, svc := range cfg.Services {
		for _, p := range svc.Ports {
			// A range or an empty value is compose choosing the port itself, so
			// there is nothing to check: it will pick a free one.
			n, err := strconv.Atoi(p.Published)
			if err != nil || n <= 0 || n > 65535 {
				continue
			}
			if seen[n] {
				continue
			}
			seen[n] = true
			ports = append(ports, HostPort{Port: n, Service: service})
		}
	}

	// Services come out of a map, so without this the order — and any message
	// built from it — would change between runs.
	sort.Slice(ports, func(i, j int) bool { return ports[i].Port < ports[j].Port })
	return ports, nil
}
