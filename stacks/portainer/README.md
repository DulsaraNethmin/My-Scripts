# portainer

[Portainer CE](https://www.portainer.io/) — a web UI for the Docker daemon this
stack is running on. Containers, images, volumes, networks, logs and an exec
shell, without leaving the browser.

```
spinup up portainer           # http://localhost:8089
spinup open portainer         # same thing, in your browser
spinup logs portainer
```

## Ports

| Service   | Host port | Container | Env var          |
|-----------|-----------|-----------|------------------|
| Web UI    | `8089`    | 9000      | `PORTAINER_PORT` |

Portainer also listens on 9443 (HTTPS with a self-signed certificate) and 8000
(the Edge agent tunnel). Neither is published: locally the first buys only a
browser warning and the second has nothing to connect to it.

## Credentials

| What     | Default        |
|----------|----------------|
| User     | `admin`        |
| Password | `spinupspinup` |

The admin account is created on the first start and lives in the data volume
from then on. Changing `PORTAINER_ADMIN_PASSWORD` afterwards does nothing —
change the password in the UI, or `spinup destroy portainer` and start over.

The default is twelve characters because that is the minimum Portainer's own
password form accepts; a shorter one set here would work but could not be
re-typed in the UI.

## The Docker socket

Portainer manages Docker by talking to `/var/run/docker.sock`, which this stack
mounts into the container. That is the entire point of the tool, and it is also
worth being clear-eyed about: a process with the socket can start any container
with any mount, which is root on the host. Run this locally, not on anything
exposed.

On Windows the host side of the socket is a named pipe instead:

```
PORTAINER_DOCKER_SOCK=//./pipe/docker_engine
```

## Storage

`portainer-data` holds the admin account, settings and saved stacks. `spinup
down portainer` keeps it; `spinup destroy portainer` deletes it and the next
start is a first start.

## Notes

- The `-H unix:///var/run/docker.sock` flag pre-creates the local environment
  ("primary"), so the UI opens on the container list rather than the "connect
  an environment" wizard.
- `portainer-init` is a one-shot BusyBox container that writes
  `PORTAINER_ADMIN_PASSWORD` into a volume before the server starts — Portainer
  reads the initial password only from a file. It exits immediately and shows
  as `Exited (0)` in `spinup ps`, which is what it should look like.
- Without that bootstrap Portainer waits for you to create the first user in
  the browser, and disables that screen five minutes after start. Getting it
  wrong means restarting the container.
- The GUI *is* the primary service here, so there is no `gui` profile and
  nothing to select with `--gui`.
- Portainer sees every container on the daemon, including the other spinup
  stacks. Stopping one from the UI works exactly like `spinup down`.
