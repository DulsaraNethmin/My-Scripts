package catalog

import (
	"fmt"
	"strings"
)

// Scaffold returns the four files of a new stack, ready to write into
// ~/.spinup/stacks/<name>/. It is what `spin new` starts from.
//
// The result is a stack that actually runs — an nginx serving its own default
// page — rather than a skeleton of comments. Something that comes up healthy on
// the first `spin up` is a better starting point than a template that has to
// be finished before it can be tried, and every convention a stack has to
// follow (env-driven ports, a pinned tag, a healthcheck on 127.0.0.1, a named
// volume) is there to be copied rather than described.
func Scaffold(name string) map[string][]byte {
	// PORT is the env var for the host port: NAME_PORT, upper case, dashes to
	// underscores, which is the same shape every catalog stack uses.
	portVar := strings.ToUpper(strings.ReplaceAll(name, "-", "_")) + "_PORT"

	files := map[string]string{
		"spinup.yaml": fmt.Sprintf(`name: %[1]s
description: A stack of my own
category: tooling
primary: app
url: http://localhost:${%[2]s}
ports:
  - name: %[2]s
    default: 8099
`, name, portVar),

		"compose.yaml": fmt.Sprintf(`# %[1]s — edit this into whatever you need.
#
# The rules the built-in stacks follow, and why:
#   * pin a real tag, never :latest, so the stack is the same next month
#   * drive every host port and credential from the environment, with an
#     inline default, so ~/.spinup/env/%[1]s.env can override them
#   * healthcheck the primary service against 127.0.0.1 — in several images
#     localhost resolves to ::1 first while the service binds IPv4 only
#   * keep data in a named volume, so `+"`spin down`"+` never destroys it

services:
  app:
    image: nginx:1.27-alpine
    restart: unless-stopped
    ports:
      - "${%[2]s:-8099}:80"
    volumes:
      - data:/usr/share/nginx/html/data
    healthcheck:
      test: ["CMD-SHELL", "wget -q --spider http://127.0.0.1:80/ || exit 1"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 5s

volumes:
  data:
`, name, portVar),

		EnvExample: fmt.Sprintf(`# %[1]s — copied to ~/.spinup/env/%[1]s.env on the first `+"`spin up`"+`.
# Edit that copy, not this one: `+"`spin env %[1]s --edit`"+`.
%[2]s=8099
`, name, portVar),

		"README.md": fmt.Sprintf(`# %[1]s

A stack of my own.

`+"```"+`
spin up %[1]s
spin info %[1]s
`+"```"+`

## What it runs

nginx, until you edit `+"`compose.yaml`"+` into something else. The stack lives in
`+"`~/.spinup/stacks/%[1]s/`"+` and nothing but you writes there.

## Ports

| Service | Env var | Default |
| --- | --- | --- |
| app | `+"`%[2]s`"+` | 8099 |

## Notes

- `+"`spin up %[1]s`"+` waits for the healthcheck, so add one to every service
  worth waiting for.
- `+"`spin down`"+` keeps the volume; `+"`spin destroy`"+` is the only thing that
  deletes it.
`, name, portVar),
	}

	out := make(map[string][]byte, len(files))
	for path, content := range files {
		out[path] = []byte(content)
	}
	return out
}
