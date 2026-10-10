package components_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/setupforge/setupforge/internal/ui/components"
	"github.com/setupforge/setupforge/internal/ui/theme"
)

func TestAnsiTruncate(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	styles := theme.NewStyles(theme.ForgeTheme)
	desc := styles.Meta.Render("— Browse packages, Flatpaks, and repositories")

	truncated := ansi.Truncate(desc, 25, "…")
	if lipgloss.Width(truncated) != 25 {
		t.Errorf("expected width 25, got %d", lipgloss.Width(truncated))
	}
}

func TestRenderPane_NoInternalBorders(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	styles := theme.NewStyles(theme.ForgeTheme)

	longContent := styles.Meta.Render("This is a very long line that exceeds pane interior width and would cause word wrapping if MaxWidth was used improperly.\nSecond line is also long and styled.")
	pane := components.RenderPane(components.PaneConfig{
		Title:     "Host System",
		Count:     "fedora",
		Position:  "1 of 1",
		Width:     45,
		Height:    10,
		IsFocused: true,
	}, longContent, styles)

	lines := strings.Split(pane, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected height 10, got %d", len(lines))
	}

	for i, line := range lines {
		stripped := stripAnsi(line)
		w := lipgloss.Width(line)
		if w != 45 {
			t.Errorf("row %d width = %d, expected 45 (line: %q)", i, w, stripped)
		}

		if i > 0 && i < len(lines)-1 {
			pipeCount := strings.Count(stripped, "│")
			if pipeCount != 2 {
				t.Errorf("row %d has %d vertical bars instead of 2: %q", i, pipeCount, stripped)
			}
		}
	}
}

func stripAnsi(s string) string {
	var b strings.Builder
	inSeq := false
	for _, r := range s {
		if r == 0x1b {
			inSeq = true
			continue
		}
		if inSeq {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inSeq = false
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
