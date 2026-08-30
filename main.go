// Command spinup starts local development services — databases, queues, GUIs
// and dev tooling — with one command, using Docker Compose underneath.
package main

import (
	"embed"
	"io/fs"
	"os"

	"github.com/DulsaraNethmin/spinup/cmd"
)

// Set at build time with -ldflags; see the Makefile and .goreleaser.yaml.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// The stack catalog is compiled into the binary, so a released spinup needs no
// clone and no network to bring a stack up.
//
// The all: prefix is load-bearing: without it go:embed silently skips files
// whose names begin with a dot, and every stack ships a .env.example.
//
//go:embed all:stacks
var stacksFS embed.FS

func main() {
	os.Exit(cmd.Execute(cmd.Build{
		Version: version,
		Commit:  commit,
		Date:    date,
	}, embeddedStacks()))
}

// embeddedStacks re-roots the embedded catalog at stacks/, so a stack's files
// are addressed as "<name>/compose.yaml" rather than "stacks/<name>/...".
func embeddedStacks() fs.FS {
	sub, err := fs.Sub(stacksFS, "stacks")
	if err != nil {
		// Unreachable: the path is a literal and the directory is embedded at
		// build time, so a failure here is a broken binary, not bad input.
		panic("spinup: embedded catalog is missing stacks/: " + err.Error())
	}
	return sub
}
