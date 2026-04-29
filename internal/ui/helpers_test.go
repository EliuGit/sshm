package ui

import (
	"sshm/internal/themes"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderShortcutGridAlignsWrappedColumns(t *testing.T) {
	t.Parallel()

	cells := []shortcutHelpCell{
		{key: "enter", label: "aaaaaaaa"},
		{key: "enter", label: "bbbbbbbb"},
		{key: "enter", label: "cccccccc"},
		{key: "enter", label: "dddddddd"},
		{key: "enter", label: "eeeeeeee"},
		{key: "enter", label: "ffffffff"},
	}
	got := renderShortcutGrid(cells, 45, themes.MustStyles(themes.DefaultName))
	lines := strings.Split(got, "\n")

	if len(lines) != 2 {
		t.Fatalf("line count = %d, want 2: %q", len(lines), got)
	}
	firstColumns := strings.Split(lines[0], "   ")
	secondColumns := strings.Split(lines[1], "   ")
	if len(firstColumns) != 3 || len(secondColumns) != 3 {
		t.Fatalf("columns not aligned into 3 cells: %q", got)
	}
	for index := range firstColumns {
		if lipgloss.Width(firstColumns[index]) != lipgloss.Width(secondColumns[index]) {
			t.Fatalf("column %d width mismatch: %q", index, got)
		}
	}
}

func TestRenderShortcutCellKeepsKeyAndTruncatesLabel(t *testing.T) {
	t.Parallel()

	got := renderShortcutCell(shortcutHelpCell{key: "enter", label: "very-long-label"}, 12, themes.MustStyles(themes.DefaultName))

	if !strings.Contains(got, "enter") {
		t.Fatalf("key should stay visible: %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("label should be truncated: %q", got)
	}
	if lipgloss.Width(got) > 12 {
		t.Fatalf("width = %d, want <= 12: %q", lipgloss.Width(got), got)
	}
}
