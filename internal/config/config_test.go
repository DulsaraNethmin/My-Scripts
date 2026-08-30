package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/config"
)

// No config.yaml is the normal case: everything has a working default.
func TestLoadConfigMissingFile(t *testing.T) {
	cfg, err := config.LoadConfig(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg != config.DefaultConfig() {
		t.Errorf("LoadConfig = %+v, want the defaults %+v", cfg, config.DefaultConfig())
	}
	if !cfg.GUI {
		t.Error("GUIs are off by default; docs/PLAN.md §7 says they are on")
	}
}

func TestLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("gui: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.GUI {
		t.Error("gui: false was not applied")
	}
}

func TestLoadConfigEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig of an empty file: %v", err)
	}
	if cfg != config.DefaultConfig() {
		t.Errorf("LoadConfig = %+v, want the defaults", cfg)
	}
}

// A key that does nothing is worse than an error: the user thinks they changed
// something.
func TestLoadConfigRejectsUnknownKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("guy: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := config.LoadConfig(path)
	if err == nil {
		t.Fatal("want an error naming the unknown key")
	}
	if !strings.Contains(err.Error(), "guy") {
		t.Errorf("error does not name the key: %v", err)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")

	cfg := config.DefaultConfig()
	if err := cfg.Set("gui", "false"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got != cfg {
		t.Errorf("round trip = %+v, want %+v", got, cfg)
	}

	// The file is edited by hand as often as by `spinup config set`.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "#") {
		t.Errorf("saved config has no header comment:\n%s", data)
	}
}

func TestGetSet(t *testing.T) {
	cfg := config.DefaultConfig()

	if got, err := cfg.Get("gui"); err != nil || got != "true" {
		t.Errorf("Get(gui) = %q, %v; want \"true\"", got, err)
	}

	if err := cfg.Set("gui", "false"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if cfg.GUI {
		t.Error("Set(gui, false) did not take")
	}

	if err := cfg.Set("gui", "yes please"); err == nil {
		t.Error("Set(gui, \"yes please\"): want an error")
	}

	for _, key := range []string{"guy", "", "GUI"} {
		if _, err := cfg.Get(key); err == nil {
			t.Errorf("Get(%q): want an error", key)
		}
		if err := cfg.Set(key, "true"); err == nil {
			t.Errorf("Set(%q): want an error", key)
		}
	}

	if len(config.Keys()) == 0 {
		t.Error("Keys() is empty")
	}
}
