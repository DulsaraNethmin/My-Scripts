// Package ui renders spinup's terminal output: colours, styles and (later)
// tables and spinners.
//
// It formats strings and never writes to a stream — commands decide what to
// print and where.
package ui

import (
	"os"
	"sync/atomic"

	"github.com/charmbracelet/lipgloss"
)

// colour is off when NO_COLOR is set to anything, per no-color.org, and can be
// turned off again by --no-color. lipgloss handles the other half of the
// problem: it strips styling on its own when stdout is not a terminal.
var colour atomic.Bool

func init() {
	_, noColor := os.LookupEnv("NO_COLOR")
	colour.Store(!noColor)
}

// SetColor enables or disables colourised output.
func SetColor(on bool) { colour.Store(on) }

// Color reports whether colourised output is enabled.
func Color() bool { return colour.Load() }

var (
	boldStyle    = lipgloss.NewStyle().Bold(true)
	dimStyle     = lipgloss.NewStyle().Faint(true)
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

// Bold renders s as bold.
func Bold(s string) string { return render(boldStyle, s) }

// Dim renders s de-emphasised, for labels and secondary detail.
func Dim(s string) string { return render(dimStyle, s) }

// Error renders s as a failure.
func Error(s string) string { return render(errorStyle, s) }

// Success renders s as a success.
func Success(s string) string { return render(successStyle, s) }

// Warn renders s as a warning.
func Warn(s string) string { return render(warnStyle, s) }

// Width is the number of terminal columns a string occupies, ignoring any
// escape sequences in it.
func Width(s string) int { return lipgloss.Width(s) }

func render(style lipgloss.Style, s string) string {
	if !colour.Load() {
		return s
	}
	return style.Render(s)
}
