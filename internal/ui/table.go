package ui

import (
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Table lays out aligned columns.
//
// It measures cells with lipgloss.Width rather than len, because a styled cell
// carries escape sequences that are bytes but not columns — the reason
// text/tabwriter cannot be used here once anything is coloured.
type Table struct {
	headers []string
	rows    [][]string
	gap     int
}

// NewTable returns a table with the given column headings.
func NewTable(headers ...string) *Table {
	return &Table{headers: headers, gap: 2}
}

// Row appends a row. Missing cells are treated as empty.
func (t *Table) Row(cells ...string) { t.rows = append(t.rows, cells) }

// Rows is how many rows the table holds.
func (t *Table) Rows() int { return len(t.rows) }

// Render writes the table.
func (t *Table) Render(w io.Writer) {
	widths := make([]int, len(t.headers))
	for i, h := range t.headers {
		widths[i] = lipgloss.Width(h)
	}
	for _, row := range t.rows {
		for i, cell := range row {
			if i < len(widths) && lipgloss.Width(cell) > widths[i] {
				widths[i] = lipgloss.Width(cell)
			}
		}
	}

	header := make([]string, len(t.headers))
	for i, h := range t.headers {
		header[i] = Dim(strings.ToUpper(h))
	}
	t.writeRow(w, header, widths)

	for _, row := range t.rows {
		t.writeRow(w, row, widths)
	}
}

func (t *Table) writeRow(w io.Writer, cells []string, widths []int) {
	var b strings.Builder

	for i, width := range widths {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}

		b.WriteString(cell)
		// The last column is never padded, so a table can be piped into
		// something else without trailing whitespace.
		if i < len(widths)-1 {
			b.WriteString(strings.Repeat(" ", width-lipgloss.Width(cell)+t.gap))
		}
	}

	b.WriteString("\n")
	_, _ = io.WriteString(w, b.String())
}
