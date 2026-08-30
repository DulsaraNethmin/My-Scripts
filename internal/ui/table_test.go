package ui_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/DulsaraNethmin/spinup/internal/ui"
)

func render(t *testing.T, table *ui.Table) []string {
	t.Helper()

	var buf bytes.Buffer
	table.Render(&buf)
	return strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
}

func TestTable(t *testing.T) {
	table := ui.NewTable("name", "status")
	table.Row("nginx-proxy-manager", "running")
	table.Row("redis", "-")

	lines := render(t, table)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want a header and two rows:\n%s", len(lines), strings.Join(lines, "\n"))
	}

	if !strings.HasPrefix(lines[0], "NAME") {
		t.Errorf("header = %q", lines[0])
	}
	// Columns line up: status starts at the same offset on every row.
	want := strings.Index(lines[1], "running")
	if got := strings.Index(lines[2], "-"); got != want {
		t.Errorf("columns are ragged:\n%s", strings.Join(lines, "\n"))
	}
	// Nothing is padded past the last column.
	for _, line := range lines {
		if strings.HasSuffix(line, " ") {
			t.Errorf("line has trailing whitespace: %q", line)
		}
	}
}

// A styled cell carries escape sequences that are bytes but not columns. If
// the table measured with len, one colour would knock the whole row out of
// alignment.
func TestTableAlignsStyledCells(t *testing.T) {
	ui.SetColor(true)
	t.Cleanup(func() { ui.SetColor(true) })

	plain := ui.NewTable("name", "status")
	plain.Row("redis", "running")
	plain.Row("postgres", "stopped")

	styled := ui.NewTable("name", "status")
	styled.Row("redis", ui.Success("running"))
	styled.Row("postgres", ui.Error("stopped"))

	for i, line := range render(t, styled) {
		want := ui.Width(render(t, plain)[i])
		if got := ui.Width(line); got != want {
			t.Errorf("row %d is %d columns wide, want %d", i, got, want)
		}
	}
}

func TestTableEmpty(t *testing.T) {
	table := ui.NewTable("name")
	if table.Rows() != 0 {
		t.Errorf("Rows = %d", table.Rows())
	}

	lines := render(t, table)
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "NAME") {
		t.Errorf("an empty table should still print its header, got %q", lines)
	}
}
