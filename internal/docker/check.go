package docker

import (
	"context"
	"errors"
	"runtime"
)

// Status is the outcome of one check.
type Status string

const (
	StatusOK   Status = "ok"
	StatusWarn Status = "warn"
	StatusFail Status = "fail"
)

// Check is one line of `spinup doctor`: what was checked, what was found, and
// — when it went wrong — what to do about it.
type Check struct {
	Name   string
	Detail string
	Status Status
	Hint   string
}

// Diagnose runs the Docker checks in the order they should be displayed. It
// stops early when a failure makes the rest meaningless: there is no point
// asking a daemon that is not running for its Compose version.
func (c *Client) Diagnose(ctx context.Context) []Check {
	if err := c.Installed(); err != nil {
		return []Check{{
			Name:   "docker",
			Detail: "not found on PATH",
			Status: StatusFail,
			Hint:   installHint(),
		}}
	}

	checks := []Check{}

	client, err := c.ClientVersion(ctx)
	switch {
	case err != nil:
		checks = append(checks, Check{
			Name: "docker", Detail: err.Error(), Status: StatusFail, Hint: installHint(),
		})
		return checks
	default:
		checks = append(checks, Check{Name: "docker", Detail: client, Status: StatusOK})
	}

	server, err := c.ServerVersion(ctx)
	if err != nil {
		checks = append(checks, Check{
			Name: "daemon", Detail: "not running", Status: StatusFail, Hint: daemonHint(),
		})
		return checks // everything below needs a daemon
	}
	checks = append(checks, Check{Name: "daemon", Detail: "running " + server, Status: StatusOK})

	compose, err := c.ComposeVersion(ctx)
	switch {
	case err != nil && errors.Is(err, ErrComposeMissing) && compose != "":
		// Present, but v1 — the end-of-life python script.
		checks = append(checks, Check{
			Name: "compose", Detail: "v" + compose + " (v2 required)", Status: StatusFail,
			Hint: "install the compose plugin: docker compose, not docker-compose",
		})
	case err != nil:
		checks = append(checks, Check{
			Name: "compose", Detail: "not available", Status: StatusFail,
			Hint: "install the docker compose plugin (see setup/ in the spinup repo)",
		})
	default:
		checks = append(checks, Check{Name: "compose", Detail: "v" + compose, Status: StatusOK})
	}

	return checks
}

// OK reports whether every check passed. Warnings do not count as failures.
func OK(checks []Check) bool {
	for _, c := range checks {
		if c.Status == StatusFail {
			return false
		}
	}
	return true
}

func installHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "install Docker Desktop, or run setup/docker-macos.sh"
	case "windows":
		return "install Docker Desktop for Windows"
	default:
		return "run setup/docker-ubuntu.sh or setup/docker-fedora.sh"
	}
}

func daemonHint() string {
	switch runtime.GOOS {
	case "darwin", "windows":
		return "start Docker Desktop"
	default:
		return "sudo systemctl start docker"
	}
}
