package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/catalog"
	"github.com/DulsaraNethmin/spinup/internal/config"
)

func TestParseEnv(t *testing.T) {
	// The shape of a real stacks/<name>/.env.example, plus the things a user
	// will type into their own copy.
	const in = `# stacks/redis — copy to ~/.spinup/env/redis.env.
# Every value here is the default baked into compose.yaml.

REDIS_PORT=6379

# A comment between entries.
REDIS_PASSWORD=spinup
export EXPORTED=yes
QUOTED="double quoted"
SINGLE='single quoted'
SPACED   =   trimmed
INLINE=6379 # the default
HASH_IN_VALUE=p#ssword
URL=postgres://user:pass@localhost:5432/db?sslmode=disable
EMPTY=
ESCAPES="a\nb"
`
	env, err := config.ParseEnv([]byte(in))
	if err != nil {
		t.Fatalf("ParseEnv: %v", err)
	}

	for key, want := range map[string]string{
		"REDIS_PORT":     "6379",
		"REDIS_PASSWORD": "spinup",
		"EXPORTED":       "yes",
		"QUOTED":         "double quoted",
		"SINGLE":         "single quoted",
		"SPACED":         "trimmed",
		"INLINE":         "6379",
		"HASH_IN_VALUE":  "p#ssword", // a # with no space before it is part of the value
		"URL":            "postgres://user:pass@localhost:5432/db?sslmode=disable",
		"EMPTY":          "",
		"ESCAPES":        "a\nb",
	} {
		if got := env[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	if len(env) != 11 {
		t.Errorf("parsed %d keys, want 11: %v", len(env), env)
	}
}

func TestParseEnvRejects(t *testing.T) {
	for name, in := range map[string]string{
		"no equals": "REDIS_PORT 6379\n",
		"empty key": "=6379\n",
	} {
		if _, err := config.ParseEnv([]byte(in)); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

// Every .env.example spinup ships has to parse, or the CLI cannot report the
// ports and credentials of its own catalog.
func TestParseEveryShippedEnvExample(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("..", "..", "stacks", "*", ".env.example"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("no .env.example files found: %v", err)
	}

	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		env, err := config.ParseEnv(data)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if len(env) == 0 {
			t.Errorf("%s: parsed no variables", path)
		}
		for key, value := range env {
			if key != strings.ToUpper(key) {
				t.Errorf("%s: %q is not an upper-case env var name", path, key)
			}
			if value == "" {
				t.Errorf("%s: %s has an empty value", path, key)
			}
		}
	}
}

func TestParseEnvFileMissingIsNotAnError(t *testing.T) {
	env, err := config.ParseEnvFile(filepath.Join(t.TempDir(), "never-written.env"))
	if err != nil {
		t.Fatalf("ParseEnvFile: %v", err)
	}
	if len(env) != 0 {
		t.Errorf("env = %v, want empty", env)
	}
}

func TestMergeEnv(t *testing.T) {
	got := config.MergeEnv(
		config.Env{"A": "1", "B": "1"},
		config.Env{"B": "2", "C": "2"},
	)
	want := config.Env{"A": "1", "B": "2", "C": "2"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestEnvAccessors(t *testing.T) {
	env := config.Env{"PORT": "5433", "EMPTY": "", "NOT_A_NUMBER": "x"}

	if got := env.Get("PORT", "5432"); got != "5433" {
		t.Errorf("Get = %q", got)
	}
	if got := env.Get("EMPTY", "fallback"); got != "fallback" {
		t.Errorf("Get of an empty value = %q, want the fallback", got)
	}
	if got := env.Int("PORT", 5432); got != 5433 {
		t.Errorf("Int = %d", got)
	}
	if got := env.Int("NOT_A_NUMBER", 5432); got != 5432 {
		t.Errorf("Int of a non-number = %d, want the fallback", got)
	}

	clone := env.Clone()
	clone["PORT"] = "1"
	if env["PORT"] != "5433" {
		t.Error("Clone shares its map with the original")
	}
}

func stack(t *testing.T) *catalog.Stack {
	t.Helper()

	s, err := catalog.ParseStack("postgres", []byte(`name: postgres
description: PostgreSQL 16
category: database
primary: postgres
url: postgres://localhost:${POSTGRES_PORT}
ports:
  - name: POSTGRES_PORT
    default: 5432
  - name: PGADMIN_PORT
    default: 8080
`))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestResolveEnvPrecedence(t *testing.T) {
	dir := t.TempDir()
	userFile := filepath.Join(dir, "postgres.env")
	if err := os.WriteFile(userFile, []byte("POSTGRES_PASSWORD=mine\nPGADMIN_PORT=9090\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	example := []byte("POSTGRES_PORT=5432\nPOSTGRES_PASSWORD=spinup\nPGADMIN_PORT=8080\n")

	env, err := config.ResolveEnv(stack(t), example, userFile)
	if err != nil {
		t.Fatalf("ResolveEnv: %v", err)
	}

	// The user's file beats .env.example, which beats the spinup.yaml default.
	if got := env["PGADMIN_PORT"]; got != "9090" {
		t.Errorf("PGADMIN_PORT = %q, want the user's 9090", got)
	}
	if got := env["POSTGRES_PASSWORD"]; got != "mine" {
		t.Errorf("POSTGRES_PASSWORD = %q, want the user's value", got)
	}
	if got := env["POSTGRES_PORT"]; got != "5432" {
		t.Errorf("POSTGRES_PORT = %q, want 5432 from .env.example", got)
	}
}

// docker compose lets the shell environment win over --env-file, so spinup has
// to report the port compose will actually bind.
func TestResolveEnvShellWinsForKnownKeys(t *testing.T) {
	t.Setenv("POSTGRES_PORT", "5555")
	t.Setenv("SPINUP_UNRELATED", "leaked")

	env, err := config.ResolveEnv(stack(t), []byte("POSTGRES_PORT=5432\n"), "")
	if err != nil {
		t.Fatalf("ResolveEnv: %v", err)
	}

	if got := env["POSTGRES_PORT"]; got != "5555" {
		t.Errorf("POSTGRES_PORT = %q, want the shell's 5555", got)
	}
	// Only variables the stack already defines are taken from the environment;
	// otherwise every stack would inherit the user's whole shell.
	if _, ok := env["SPINUP_UNRELATED"]; ok {
		t.Error("an unrelated environment variable leaked into the stack env")
	}
	if _, ok := env["PATH"]; ok {
		t.Error("PATH leaked into the stack env")
	}
}

func TestResolveEnvDefaultsFromSpinupYAML(t *testing.T) {
	// No .env.example, no user file: the ports still resolve, because
	// spinup.yaml declares their defaults.
	env, err := config.ResolveEnv(stack(t), nil, "")
	if err != nil {
		t.Fatalf("ResolveEnv: %v", err)
	}
	if env["POSTGRES_PORT"] != "5432" || env["PGADMIN_PORT"] != "8080" {
		t.Errorf("env = %v", env)
	}
}
