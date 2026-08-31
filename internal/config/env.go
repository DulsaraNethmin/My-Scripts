package config

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
)

// Env is a stack's environment: the ports, credentials and database names that
// its compose.yaml reads.
type Env map[string]string

// Get returns a value, or fallback when it is unset or empty.
func (e Env) Get(key, fallback string) string {
	if v, ok := e[key]; ok && v != "" {
		return v
	}
	return fallback
}

// Int returns a value parsed as an int, or fallback when it is unset or not a
// number.
func (e Env) Int(key string, fallback int) int {
	v, err := strconv.Atoi(e.Get(key, ""))
	if err != nil {
		return fallback
	}
	return v
}

// Clone returns a copy, so callers can override values without disturbing the
// resolved environment.
func (e Env) Clone() Env {
	out := make(Env, len(e))
	for k, v := range e {
		out[k] = v
	}
	return out
}

// MergeEnv layers environments, with later ones winning.
func MergeEnv(layers ...Env) Env {
	out := Env{}
	for _, l := range layers {
		for k, v := range l {
			out[k] = v
		}
	}
	return out
}

// ParseEnv reads the KEY=value format Compose uses for --env-file: comments
// and blank lines are skipped, `export` prefixes are allowed, and values may
// be quoted.
//
// Compose is the authority here — spinup passes the same file to it with
// --env-file, so this parser exists to *report* what compose will do (in
// `spin url`, `env` and the post-up card), not to decide it.
func ParseEnv(data []byte) (Env, error) {
	env := Env{}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		text = strings.TrimPrefix(text, "export ")

		key, value, ok := strings.Cut(text, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: %q is not KEY=value", line, text)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("line %d: empty variable name", line)
		}
		env[key] = unquote(strings.TrimSpace(value))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return env, nil
}

// unquote strips one layer of quotes, or an unquoted trailing comment.
func unquote(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		inner := v[1 : len(v)-1]
		if v[0] == '"' {
			// Only the escapes compose itself expands in double quotes.
			r := strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\"`, `"`, `\\`, `\`)
			return r.Replace(inner)
		}
		return inner
	}

	// An unquoted value ends at the first ` #`: `PORT=5432 # the default`.
	if i := strings.Index(v, " #"); i >= 0 {
		return strings.TrimSpace(v[:i])
	}
	return v
}

// ParseEnvFile reads an env file. A missing file is not an error — a stack
// that has never been configured just uses its defaults.
func ParseEnvFile(path string) (Env, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Env{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	env, err := ParseEnv(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return env, nil
}

// ResolveEnv builds a stack's environment from every layer that contributes to
// it, lowest precedence first:
//
//  1. the port defaults in spinup.yaml
//  2. the stack's .env.example — the documented defaults
//  3. the user's env file, ~/.spinup/env/<stack>.env
//  4. the process environment, but only for variables the stack already
//     defines
//
// Step 4 mirrors `docker compose`, which lets the shell environment win over
// --env-file: `POSTGRES_PORT=5555 spin up postgres` has to report the port
// compose will actually bind. Limiting it to known keys is what keeps an
// unrelated PATH or USER out of the stack's environment.
func ResolveEnv(stack *catalog.Stack, example []byte, userFile string) (Env, error) {
	defaults := Env{}
	for _, p := range stack.Ports {
		defaults[p.Name] = strconv.Itoa(p.Default)
	}

	documented, err := ParseEnv(example)
	if err != nil {
		return nil, fmt.Errorf("%s/%s: %w", stack.Name, catalog.EnvExample, err)
	}

	user, err := ParseEnvFile(userFile)
	if err != nil {
		return nil, err
	}

	env := MergeEnv(defaults, documented, user)

	for key := range env {
		if v, ok := os.LookupEnv(key); ok {
			env[key] = v
		}
	}
	return env, nil
}
