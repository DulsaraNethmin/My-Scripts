// Package compose wraps the `docker compose` CLI.
//
// spinup shells out rather than using the Docker SDK, so users get exactly the
// behaviour they would get by running compose by hand, and every stack stays a
// plain compose.yaml that works without spinup installed.
//
// Filled in by task 2.4: command construction (project name spinup-<stack>,
// --env-file, --profile), streaming output and exit-code mapping.
package compose
