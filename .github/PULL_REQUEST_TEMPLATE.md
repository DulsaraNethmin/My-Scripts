## What and why

<!-- The interesting half of most changes here is the constraint that forced
     them. Say what the alternative was and why it did not work. -->

## Checks

- [ ] `make check` passes
- [ ] `CHANGELOG.md` has a line under `[Unreleased]`

## If this adds or changes a stack

- [ ] Ports claimed in `docs/PORTS.md`
- [ ] Image pinned to a tag that exists (`docker manifest inspect`)
- [ ] Healthcheck probes `127.0.0.1` and uses tooling the image actually has
- [ ] A GUI in its own container is behind the `gui` profile; one served by the
      primary service declares no profile
- [ ] `spinup up <name>` reaches healthy, the service does the thing it is for,
      and `spinup destroy <name>` leaves no containers or volumes
- [ ] Added to the `smoke` matrix in `.github/workflows/ci.yml`
- [ ] `README.md` in the stack folder: ports, credentials, seeding, gotchas
