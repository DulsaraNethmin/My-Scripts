package docker

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
)

// Conflict is a host port that cannot be bound, and what holds it.
type Conflict struct {
	Port int

	// Holder is the container holding the port, empty when something outside
	// Docker has it. Naming the container is the whole point of the check —
	// "1025 is taken" sends the user looking, "shipper-mailpit is using 1025"
	// does not.
	Holder string
}

// portHolder is one running container's claim on a host port.
type portHolder struct {
	name    string
	project string
}

// PortConflicts reports which of ports cannot be bound on this host.
//
// ignoreProject is the compose project whose own containers do not count:
// `up` is idempotent, and a stack that is already running holds exactly the
// ports it is about to be asked for.
//
// The check errs towards saying nothing. A port it cannot decide about is
// reported as free, because a wrong refusal stops a start that would have
// worked, while a missed conflict only means the user sees compose's own error
// — which is what they got before this existed.
func (c *Client) PortConflicts(ctx context.Context, ports []int, ignoreProject string) []Conflict {
	holders := c.portHolders(ctx)

	var out []Conflict
	for _, port := range ports {
		if h, ok := holders[port]; ok {
			if h.project != "" && h.project == ignoreProject {
				continue
			}
			out = append(out, Conflict{Port: port, Holder: h.name})
			continue
		}
		if !bindable(port) {
			out = append(out, Conflict{Port: port})
		}
	}
	return out
}

// portHolders maps each published host port to the container publishing it.
// A docker that will not answer gives an empty map, not an error: the bind
// probe still works, and half an answer beats refusing to check.
func (c *Client) portHolders(ctx context.Context) map[int]portHolder {
	out, err := c.run(ctx, "ps", "--format",
		`{{.Names}}\t{{.Ports}}\t{{.Label "com.docker.compose.project"}}`)
	if err != nil {
		return nil
	}

	holders := make(map[int]portHolder)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(strings.TrimSpace(line), "\t")
		if len(fields) < 2 || fields[0] == "" {
			continue
		}
		h := portHolder{name: fields[0]}
		if len(fields) > 2 {
			h.project = fields[2]
		}
		for _, port := range publishedPorts(fields[1]) {
			// First writer wins: a port is published by one container, and the
			// dual 0.0.0.0/[::] entries docker prints are the same claim.
			if _, seen := holders[port]; !seen {
				holders[port] = h
			}
		}
	}
	return holders
}

// publishedPorts pulls the host ports out of docker's Ports column, which
// reads like "0.0.0.0:1025->1025/tcp, [::]:1025->1025/tcp, 8025/tcp". Only the
// mappings have a host port; a bare "8025/tcp" is exposed, not published.
func publishedPorts(column string) []int {
	var ports []int
	for _, mapping := range strings.Split(column, ",") {
		mapping = strings.TrimSpace(mapping)
		host, _, ok := strings.Cut(mapping, "->")
		if !ok {
			continue
		}
		// The host side is [::]:1025, 0.0.0.0:1025 or 1025.
		if i := strings.LastIndex(host, ":"); i >= 0 {
			host = host[i+1:]
		}
		if n, err := strconv.Atoi(host); err == nil && n > 0 && n <= 65535 {
			ports = append(ports, n)
		}
	}
	return ports
}

// bindable reports whether the port can be listened on. A permission error is
// not a conflict: binding below 1024 needs privileges spinup does not have and
// does not need, and treating that as "in use" would refuse to start every
// stack that publishes port 80.
func bindable(port int) bool {
	l, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(port)))
	if err == nil {
		_ = l.Close()
		return true
	}
	return errors.Is(err, os.ErrPermission)
}
