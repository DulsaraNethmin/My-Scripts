package docker_test

import (
	"context"
	"net"
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/docker"
)

const psArgs = `ps --format {{.Names}}\t{{.Ports}}\t{{.Label "com.docker.compose.project"}}`

func TestPortConflictsNamesTheHolder(t *testing.T) {
	dc := docker.NewWith(fake{out: map[string]string{
		psArgs: "shipper-mailpit\t0.0.0.0:1025->1025/tcp, [::]:1025->1025/tcp, 0.0.0.0:8025->8025/tcp\tshipper",
	}})

	got := dc.PortConflicts(context.Background(), []int{1025}, "spinup-mailpit")
	if len(got) != 1 {
		t.Fatalf("PortConflicts gave %v, want one conflict", got)
	}
	if got[0].Port != 1025 || got[0].Holder != "shipper-mailpit" {
		t.Errorf("got %+v, want port 1025 held by shipper-mailpit", got[0])
	}
}

// `up` is idempotent, so a stack that is already running must not be refused
// over the ports it is itself holding.
func TestPortConflictsIgnoresItsOwnProject(t *testing.T) {
	dc := docker.NewWith(fake{out: map[string]string{
		psArgs: "spinup-mailpit-mailpit-1\t0.0.0.0:1025->1025/tcp\tspinup-mailpit",
	}})

	if got := dc.PortConflicts(context.Background(), []int{1025}, "spinup-mailpit"); len(got) != 0 {
		t.Errorf("a stack conflicted with itself: %+v", got)
	}
	// The same container under another project is a real conflict.
	if got := dc.PortConflicts(context.Background(), []int{1025}, "spinup-other"); len(got) != 1 {
		t.Errorf("PortConflicts gave %+v, want one conflict", got)
	}
}

// An exposed-but-not-published port has no host side and must not be read as
// one: "8025/tcp" means the container listens, not that the host is bound.
func TestPortConflictsIgnoresUnpublishedPorts(t *testing.T) {
	dc := docker.NewWith(fake{out: map[string]string{
		psArgs: "lonely\t8025/tcp\t",
	}})

	free := freePort(t)
	if got := dc.PortConflicts(context.Background(), []int{free}, ""); len(got) != 0 {
		t.Errorf("an exposed port was read as published: %+v", got)
	}
}

// Docker not answering is not a reason to refuse a start: the bind probe still
// decides, and the port is simply unattributed.
func TestPortConflictsFallsBackToTheBindProbe(t *testing.T) {
	dc := docker.NewWith(fake{err: map[string]error{psArgs: errNoDocker{}}})

	// The wildcard, because that is what the probe binds and what docker
	// publishes to. Holding only 127.0.0.1 does not stop a wildcard bind on
	// macOS, so a loopback listener here would prove nothing.
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close() //nolint:errcheck // test cleanup
	taken := l.Addr().(*net.TCPAddr).Port

	got := dc.PortConflicts(context.Background(), []int{taken}, "")
	if len(got) != 1 {
		t.Fatalf("PortConflicts gave %+v, want the bound port back", got)
	}
	if got[0].Holder != "" {
		t.Errorf("holder is %q, want empty when docker cannot say", got[0].Holder)
	}

	if got := dc.PortConflicts(context.Background(), []int{freePort(t)}, ""); len(got) != 0 {
		t.Errorf("a free port was reported as taken: %+v", got)
	}
}

type errNoDocker struct{}

func (errNoDocker) Error() string { return "docker: not found" }

// freePort returns a port nothing is listening on. Racy in principle; the
// window is a few microseconds and nothing else here binds ports.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return port
}
