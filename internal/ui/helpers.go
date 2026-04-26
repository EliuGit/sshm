package ui

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sshm/internal/domain"
	"sshm/internal/i18n"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newInput(theme Theme, placeholder string, width int) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.Width = width
	input.Prompt = "> "
	input.PromptStyle = theme.Styles.SubtleText
	input.PlaceholderStyle = theme.Styles.MutedText
	return input
}

func detectLocalStartPath() string {
	wd, err := os.Getwd()
	if err == nil {
		return wd
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return home
	}
	return "."
}

func (p *filePanel) rows() []domain.FileEntry {
	rows := make([]domain.FileEntry, 0, len(p.items)+1)
	if strings.TrimSpace(p.path) != "" {
		parent := parentPath(p.path, p.panel == domain.RemotePanel)
		if parent != p.path {
			rows = append(rows, domain.FileEntry{Name: "..", Path: parent, IsDir: true, Panel: p.panel})
		}
	}
	rows = append(rows, p.items...)
	return rows
}

func (p *filePanel) selected() (domain.FileEntry, bool) {
	rows := p.rows()
	if len(rows) == 0 || p.cursor < 0 || p.cursor >= len(rows) {
		return domain.FileEntry{}, false
	}
	return rows[p.cursor], true
}

func (p *filePanel) selectByName(name string) {
	rows := p.rows()
	for index, row := range rows {
		if row.Name == name {
			p.cursor = index
			return
		}
	}
	p.cursor = clamp(p.cursor, len(rows))
}

func clamp(current int, size int) int {
	if size <= 0 {
		return 0
	}
	if current < 0 {
		return 0
	}
	if current >= size {
		return size - 1
	}
	return current
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func humanSize(size int64) string {
	switch {
	case size >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(size)/(1<<30))
	case size >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(size)/(1<<20))
	case size >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(size)/(1<<10))
	default:
		return fmt.Sprintf("%dB", size)
	}
}

func parentPath(current string, remote bool) string {
	if remote {
		next := path.Dir(current)
		if next == "." {
			next = "/"
		}
		return next
	}
	return filepath.Dir(current)
}

func joinRemotePath(basePath string, name string) string {
	if basePath == "" {
		return path.Clean("/" + name)
	}
	return path.Clean(path.Join(basePath, name))
}

func renderTransferProgress(action string, progress domain.TransferProgress) string {
	if progress.Total <= 0 {
		return fmt.Sprintf("%s %s", action, progress.Path)
	}
	percent := float64(progress.Bytes) * 100 / float64(progress.Total)
	return fmt.Sprintf("%s %s %s/%s (%.0f%%)", action, progress.Path, humanSize(progress.Bytes), humanSize(progress.Total), percent)
}

func listenTransferProgress(source <-chan transferProgressMsg) tea.Cmd {
	if source == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-source
		if !ok {
			return nil
		}
		return msg
	}
}

func truncate(value string, width int) string {
	if lipgloss.Width(value) <= width {
		return value
	}
	runes := []rune(value)
	for len(runes) > 0 && lipgloss.Width(string(runes)+"…") > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func overlayCenter(base string, overlay string, width int, height int) string {
	if width <= 0 || height <= 0 {
		return base + "\n\n" + overlay
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}

func localizedShortcutHelpWidth(translator *i18n.Translator, theme Theme, width int, items ...string) string {
	cells := make([]shortcutHelpCell, 0, len(items)/2)
	for index := 0; index+1 < len(items); index += 2 {
		cells = append(cells, shortcutHelpCell{
			key:   items[index],
			label: translator.T(items[index+1]),
		})
	}
	return theme.Styles.HelpText.Render(renderShortcutGrid(cells, width, theme))
}

type shortcutHelpCell struct {
	key   string
	label string
}

func renderShortcutGrid(cells []shortcutHelpCell, width int, theme Theme) string {
	if len(cells) == 0 {
		return ""
	}
	gap := "   "
	if width <= 0 {
		parts := make([]string, 0, len(cells))
		for _, cell := range cells {
			parts = append(parts, renderShortcutCell(cell, 0, theme))
		}
		return strings.Join(parts, gap)
	}

	for columns := min(6, len(cells)); columns >= 1; columns-- {
		columnWidths, ok := shortcutColumnWidths(cells, columns, width, lipgloss.Width(gap), theme)
		if !ok {
			continue
		}
		return renderShortcutRows(cells, columns, columnWidths, gap, theme)
	}

	return renderShortcutRows(cells, 1, []int{width}, gap, theme)
}

func shortcutColumnWidths(cells []shortcutHelpCell, columns int, width int, gapWidth int, theme Theme) ([]int, bool) {
	available := width - gapWidth*(columns-1)
	if available <= 0 {
		return nil, false
	}

	minWidths := make([]int, columns)
	desiredWidths := make([]int, columns)
	for index, cell := range cells {
		column := index % columns
		keyWidth := lipgloss.Width(theme.Styles.Keycap.Render(cell.key))
		labelWidth := lipgloss.Width(cell.label)
		minLabelWidth := min(3, labelWidth)
		minWidth := keyWidth
		if labelWidth > 0 {
			minWidth += 1 + minLabelWidth
		}
		desiredWidth := keyWidth
		if labelWidth > 0 {
			desiredWidth += 1 + labelWidth
		}
		minWidths[column] = max(minWidths[column], minWidth)
		desiredWidths[column] = max(desiredWidths[column], desiredWidth)
	}

	minTotal := 0
	for _, item := range minWidths {
		minTotal += item
	}
	if minTotal > available {
		return nil, false
	}

	widths := append([]int{}, minWidths...)
	extra := available - minTotal
	for extra > 0 {
		changed := false
		for index := range widths {
			if widths[index] >= desiredWidths[index] {
				continue
			}
			widths[index]++
			extra--
			changed = true
			if extra == 0 {
				break
			}
		}
		if !changed {
			break
		}
	}
	return widths, true
}

func renderShortcutRows(cells []shortcutHelpCell, columns int, columnWidths []int, gap string, theme Theme) string {
	lines := make([]string, 0, (len(cells)+columns-1)/columns)
	for rowStart := 0; rowStart < len(cells); rowStart += columns {
		parts := []string{}
		for column := 0; column < columns; column++ {
			index := rowStart + column
			if index >= len(cells) {
				break
			}
			width := columnWidths[column]
			rendered := renderShortcutCell(cells[index], width, theme)
			parts = append(parts, lipgloss.NewStyle().Width(width).Render(rendered))
		}
		lines = append(lines, strings.Join(parts, gap))
	}
	return strings.Join(lines, "\n")
}

func renderShortcutCell(cell shortcutHelpCell, width int, theme Theme) string {
	key := theme.Styles.Keycap.Render(cell.key)
	if cell.label == "" {
		return key
	}
	if width <= 0 {
		return key + " " + theme.Styles.ShortcutLabel.Render(cell.label)
	}
	labelWidth := width - lipgloss.Width(key) - 1
	if labelWidth <= 0 {
		return key
	}
	return key + " " + theme.Styles.ShortcutLabel.Render(truncate(cell.label, labelWidth))
}
