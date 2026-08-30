package config

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is ~/.spinup/config.yaml: the preferences that change what spinup
// does by default. Everything here has a working default, so the file only
// exists once someone changes something.
type Config struct {
	// GUI brings a stack's gui profile up with it, so `spinup up postgres`
	// starts pgAdmin too. --no-gui turns it off for a single run.
	GUI bool `yaml:"gui"`
}

// DefaultConfig is the configuration spinup uses when there is no config.yaml.
//
// GUIs are on: the GUI is half the reason to run a stack locally, and a user
// who wants `up` lean can turn it off once. See docs/PLAN.md §7.
func DefaultConfig() Config {
	return Config{GUI: true}
}

// LoadConfig reads config.yaml. A missing file is not an error — it means the
// defaults. Unknown keys are, so a typo is reported rather than ignored.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		// An empty file decodes to io.EOF and means "defaults".
		if errors.Is(err, io.EOF) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Save writes config.yaml, creating its directory if needed.
func (c Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}

	header := "# spinup configuration. `spinup config set <key> <value>` edits it.\n"
	if err := os.WriteFile(path, append([]byte(header), data...), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// Keys are the settable configuration keys, in display order.
func Keys() []string { return []string{"gui"} }

// Get returns a key's value as a string.
func (c Config) Get(key string) (string, error) {
	switch key {
	case "gui":
		return strconv.FormatBool(c.GUI), nil
	default:
		return "", unknownKey(key)
	}
}

// Set updates a key from its string form, as `spinup config set` supplies it.
func (c *Config) Set(key, value string) error {
	switch key {
	case "gui":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("gui: %q is not true or false", value)
		}
		c.GUI = b
		return nil
	default:
		return unknownKey(key)
	}
}

func unknownKey(key string) error {
	return fmt.Errorf("unknown config key %q (known keys: %s)", key, strings.Join(Keys(), ", "))
}
