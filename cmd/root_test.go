package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
)

func TestCodeFor(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want int
	}{
		"plain error":     {errors.New("boom"), ExitUsage},
		"coded":           {failf(ExitCompose, "compose said no"), ExitCompose},
		"docker missing":  {failf(ExitDocker, "no daemon"), ExitDocker},
		"wrapped coded":   {fmt.Errorf("up: %w", failf(ExitCompose, "boom")), ExitCompose},
		"stack not found": {fmt.Errorf("%q: %w", "nope", catalog.ErrNotFound), ExitNotFound},
	} {
		if got := codeFor(tc.err); got != tc.want {
			t.Errorf("%s: codeFor(%v) = %d, want %d", name, tc.err, got, tc.want)
		}
	}
}

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	root := newRootCmd(Build{Version: "1.2.3", Commit: "abc1234", Date: "2026-01-01T00:00:00Z"})
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)

	err := root.Execute()
	return out.String(), err
}

func TestVersion(t *testing.T) {
	out, err := run(t, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	for _, want := range []string{"1.2.3", "abc1234", "2026-01-01T00:00:00Z", "go1."} {
		if !strings.Contains(out, want) {
			t.Errorf("version output is missing %q:\n%s", want, out)
		}
	}
}

func TestVersionShort(t *testing.T) {
	// --short is what a script parses, so it must be the bare version and
	// nothing else.
	out, err := run(t, "version", "--short")
	if err != nil {
		t.Fatalf("version --short: %v", err)
	}
	if out != "1.2.3\n" {
		t.Errorf("version --short = %q, want %q", out, "1.2.3\n")
	}
}

func TestUnknownCommandIsUsageError(t *testing.T) {
	_, err := run(t, "nosuchcommand")
	if err == nil {
		t.Fatal("unknown command: want an error")
	}
	if got := codeFor(err); got != ExitUsage {
		t.Errorf("unknown command exits %d, want %d", got, ExitUsage)
	}
}

func TestUnknownFlagIsUsageError(t *testing.T) {
	_, err := run(t, "--nosuchflag")
	if err == nil {
		t.Fatal("unknown flag: want an error")
	}
	if got := codeFor(err); got != ExitUsage {
		t.Errorf("unknown flag exits %d, want %d", got, ExitUsage)
	}
}
