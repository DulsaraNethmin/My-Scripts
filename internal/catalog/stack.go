package catalog

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Category groups stacks in `spin list`. The set is closed so a typo in a
// spinup.yaml is caught rather than quietly creating a new group of one.
type Category string

const (
	CategoryDatabase  Category = "database"
	CategoryMessaging Category = "messaging"
	CategoryStorage   Category = "storage"
	CategoryTooling   Category = "tooling"
	CategoryML        Category = "ml"
	CategoryWeb       Category = "web"
)

// Categories are the valid values of a stack's category, in display order.
var Categories = []Category{
	CategoryDatabase, CategoryMessaging, CategoryStorage,
	CategoryTooling, CategoryML, CategoryWeb,
}

// Origin says where a stack came from: the binary, or the user's own
// ~/.spinup/stacks.
type Origin string

const (
	OriginBuiltin Origin = "builtin"
	OriginUser    Origin = "user"
)

// Stack is one stack's spinup.yaml — the metadata the CLI reads. Everything
// the CLI needs comes from here or from the stack's compose.yaml; adding a
// stack never means editing Go code.
//
// See docs/PLAN.md §4. The schema below is what the catalog actually uses:
// default_profiles and gpu are not in the PLAN text but are load-bearing for
// stacks/pytorch.
type Stack struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Category    Category `yaml:"category"`
	Primary     string   `yaml:"primary"` // service used by shell/cli/logs by default
	CLI         string   `yaml:"cli"`     // native client command, run inside the primary service
	GUI         *GUI     `yaml:"gui"`
	URL         string   `yaml:"url"` // connection string, printed by `spin url`
	Ports       []Port   `yaml:"ports"`

	// Worker marks a stack that binds no host port at all — a scheduler or
	// background job that is consumed through its own CLI rather than over a
	// socket. It lifts the url and ports requirements and requires cli
	// instead: with nothing to connect to, `spin cli` is the only way in.
	Worker bool `yaml:"worker"`

	// Profiles are the Compose profiles the stack defines. Anything behind a
	// profile is off unless it is selected.
	Profiles []string `yaml:"profiles"`

	// DefaultProfiles are applied by `spin up` when the user selects none.
	// stacks/pytorch needs this: both of its services sit behind profiles and
	// share ports, so with no profile the stack starts nothing at all.
	DefaultProfiles []string `yaml:"default_profiles"`

	// GPU names the profile and service that use the NVIDIA runtime, for
	// `spin up --gpu`.
	GPU *GPU `yaml:"gpu"`

	// Origin is filled in by the catalog, not by the file.
	Origin Origin `yaml:"-"`
}

// GUI is the stack's web interface, started by the `gui` profile.
type GUI struct {
	Service string `yaml:"service"`
	URL     string `yaml:"url"`
	Login   string `yaml:"login"`
}

// Port is a host port the stack binds, named after the env var that sets it.
type Port struct {
	Name    string `yaml:"name"`
	Default int    `yaml:"default"`
}

// GPU is the profile/service pair that runs on an NVIDIA GPU.
type GPU struct {
	Profile string `yaml:"profile"`
	Service string `yaml:"service"`
}

// HasGUI reports whether the stack ships a web interface.
func (s *Stack) HasGUI() bool { return s.GUI != nil && s.GUI.Service != "" }

// HasGPU reports whether the stack has a GPU variant.
func (s *Stack) HasGPU() bool { return s.GPU != nil && s.GPU.Profile != "" }

// HasProfile reports whether the stack declares the named Compose profile.
func (s *Stack) HasProfile(name string) bool {
	for _, p := range s.Profiles {
		if p == name {
			return true
		}
	}
	return false
}

// PortNames returns the env var names of the stack's host ports.
func (s *Stack) PortNames() []string {
	names := make([]string, 0, len(s.Ports))
	for _, p := range s.Ports {
		names = append(names, p.Name)
	}
	return names
}

// Port names are environment variables: POSTGRES_PORT, not postgres-port.
var portNamePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// ParseStack decodes a spinup.yaml. dir is the stack's directory name, which
// the file's own name field has to match.
func ParseStack(dir string, data []byte) (*Stack, error) {
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true) // a mistyped key is a bug, not a silently ignored one

	var s Stack
	if err := dec.Decode(&s); err != nil {
		return nil, fmt.Errorf("%s/spinup.yaml: %w", dir, err)
	}

	if err := s.validate(dir); err != nil {
		return nil, fmt.Errorf("%s/spinup.yaml: %w", dir, err)
	}
	return &s, nil
}

// validate reports everything wrong with the stack at once, so a new stack can
// be fixed in one pass rather than one error at a time.
func (s *Stack) validate(dir string) error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	switch {
	case s.Name == "":
		bad("name is required")
	case !ValidName(s.Name):
		bad("name %q is not kebab-case", s.Name)
	case dir != "" && s.Name != dir:
		bad("name %q does not match the directory %q", s.Name, dir)
	}

	if s.Description == "" {
		bad("description is required")
	}
	if s.Category == "" {
		bad("category is required")
	} else if !validCategory(s.Category) {
		bad("category %q is not one of %v", s.Category, Categories)
	}
	if s.Primary == "" {
		bad("primary is required — it is the service shell, cli and logs default to")
	}
	// url and at least one port are required by scripts/lint-stacks.sh too.
	// The two validators have to agree, or a stack passes CI and then fails in
	// the CLI, or the other way round. A worker stack is the exception in
	// both: it binds nothing, so there is no address to print and no port to
	// claim — but its cli becomes required, or the stack is unreachable.
	if s.URL == "" && !s.Worker {
		bad("url is required — it is what `spin url` prints")
	}
	if len(s.Ports) == 0 && !s.Worker {
		bad("ports is required — a service nothing can connect to is not a stack")
	}
	if s.Worker && s.CLI == "" {
		bad("worker: true requires cli — with no port to connect to, the stack's own client is the only way in")
	}

	seen := map[string]bool{}
	for i, p := range s.Ports {
		switch {
		case p.Name == "":
			bad("ports[%d]: name is required", i)
		case !portNamePattern.MatchString(p.Name):
			bad("ports[%d]: %q is not an env var name", i, p.Name)
		case seen[p.Name]:
			bad("ports[%d]: %s is listed twice", i, p.Name)
		}
		seen[p.Name] = true

		if p.Default < 1 || p.Default > 65535 {
			bad("ports[%d]: %s default %d is not a port", i, p.Name, p.Default)
		}
	}

	if s.GUI != nil {
		if s.GUI.Service == "" {
			bad("gui.service is required")
		}
		if s.GUI.URL == "" {
			bad("gui.url is required")
		}
		// A GUI in its own container is optional and belongs behind the gui
		// profile, or it is always on and the profile means nothing. When the
		// GUI *is* the primary service — nginx, Nginx Proxy Manager, Jupyter —
		// there is nothing to gate, and stacks/nginx-static et al. correctly
		// declare no gui profile at all.
		if s.GUI.Service != s.Primary && !s.HasProfile("gui") {
			bad("gui.service %q is a container of its own but %q is not in profiles",
				s.GUI.Service, "gui")
		}
	}

	for _, p := range s.Profiles {
		if !ValidName(p) {
			bad("profiles: %q is not a valid profile name", p)
		}
	}
	for _, p := range s.DefaultProfiles {
		if !s.HasProfile(p) {
			bad("default_profiles: %q is not in profiles", p)
		}
	}

	if s.GPU != nil {
		if s.GPU.Profile == "" {
			bad("gpu.profile is required")
		} else if !s.HasProfile(s.GPU.Profile) {
			bad("gpu.profile %q is not in profiles", s.GPU.Profile)
		}
		if s.GPU.Service == "" {
			bad("gpu.service is required")
		}
	}

	return errors.Join(errs...)
}

func validCategory(c Category) bool {
	for _, known := range Categories {
		if c == known {
			return true
		}
	}
	return false
}

// Expand resolves ${VAR} references in a spinup.yaml value — connection
// strings, GUI URLs and cli commands are all written in terms of the stack's
// env vars. An unset variable expands to the empty string, as it would in
// Compose.
func Expand(s string, env map[string]string) string {
	return os.Expand(s, func(key string) string { return env[key] })
}
