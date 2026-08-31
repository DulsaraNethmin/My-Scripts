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
	case !c.SupportsWaitTimeout(ctx):
		// Every `spinup up` passes --wait-timeout, so a plugin without it
		// fails on the first command rather than here.
		checks = append(checks, Check{
			Name: "compose", Detail: "v" + compose + ", without --wait-timeout", Status: StatusWarn,
			Hint: "upgrade the compose plugin; `spinup up` waits for healthy with that flag",
		})
	default:
		checks = append(checks, Check{Name: "compose", Detail: "v" + compose, Status: StatusOK})
	}

	checks = append(checks, c.gpuCheck(ctx))

	return checks
}

// gpuCheck reports the NVIDIA container runtime, which only the pytorch stack's
// gpu profile needs. Not having one is not a problem — most machines do not —
// so the case worth flagging is the machine with a driver and no runtime: a GPU
// docker cannot reach is a setup someone meant to finish.
func (c *Client) gpuCheck(ctx context.Context) Check {
	has, err := c.HasNVIDIARuntime(ctx)
	switch {
	case err != nil:
		return Check{Name: "gpu", Detail: "could not be determined", Status: StatusWarn, Hint: err.Error()}
	case has:
		return Check{Name: "gpu", Detail: "nvidia runtime available", Status: StatusOK}
	case HasNVIDIADriver():
		return Check{
			Name: "gpu", Detail: "an NVIDIA driver is installed but docker has no nvidia runtime",
			Status: StatusWarn,
			Hint:   "install nvidia-container-toolkit and restart docker, or skip `spinup up pytorch --gpu`",
		}
	default:
		return Check{Name: "gpu", Detail: "no nvidia runtime (only `spinup up pytorch --gpu` needs one)", Status: StatusOK}
	}
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
