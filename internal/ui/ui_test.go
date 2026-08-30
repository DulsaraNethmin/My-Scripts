package ui_test

import (
	"strings"
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func TestSetColorOff(t *testing.T) {
	t.Cleanup(func() { ui.SetColor(true) })

	ui.SetColor(false)
	if ui.Color() {
		t.Error("Color() is true after SetColor(false)")
	}

	// With colour off the text must come back byte for byte: spinup's output is
	// piped into scripts, and a stray escape sequence breaks them.
	for name, f := range map[string]func(string) string{
		"Bold":    ui.Bold,
		"Dim":     ui.Dim,
		"Error":   ui.Error,
		"Success": ui.Success,
		"Warn":    ui.Warn,
	} {
		const in = "postgres 5432"
		if got := f(in); got != in {
			t.Errorf("%s(%q) = %q, want it unchanged", name, in, got)
		}
	}
}

func TestSetColorOn(t *testing.T) {
	t.Cleanup(func() { ui.SetColor(true) })

	ui.SetColor(true)
	if !ui.Color() {
		t.Error("Color() is false after SetColor(true)")
	}

	// Under `go test` stdout is not a terminal, so lipgloss strips the styling
	// itself. Either way the text must survive.
	if got := ui.Bold("spinup"); !strings.Contains(got, "spinup") {
		t.Errorf("Bold dropped the text: %q", got)
	}
}
