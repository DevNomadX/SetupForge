package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/setupforge/setupforge/internal/ui/layout"
	"github.com/setupforge/setupforge/internal/ui/theme"
)

// KeyHint defines a contextual key binding and action label.
type KeyHint struct {
	Key      string
	Action   string
	Priority int // 1 = highest priority (never dropped), 4 = low priority
}

// DockModel renders the bottom contextual status and keybinding bar.
type DockModel struct {
	StatusText  string
	StatusGlyph string
	Hints       []KeyHint
}

// NewDockModel creates a default DockModel.
func NewDockModel() DockModel {
	return DockModel{
		StatusText:  "Ready",
		StatusGlyph: theme.GlyphMarked,
		Hints: []KeyHint{
			{Key: "1-7", Action: "jump", Priority: 1},
			{Key: "Tab", Action: "focus", Priority: 1},
			{Key: "j/k", Action: "move", Priority: 2},
			{Key: "Enter", Action: "select", Priority: 2},
			{Key: "Ctrl+B", Action: "rail", Priority: 3},
			{Key: "r", Action: "refresh", Priority: 3},
			{Key: "?", Action: "help", Priority: 1},
			{Key: "Ctrl+Q", Action: "quit", Priority: 1},
		},
	}
}

// Render renders the 1-row dock adhering to width and breakpoint limits.
func (d DockModel) Render(m layout.Metrics, styles theme.Styles) string {
	if m.TotalWidth <= 0 {
		return ""
	}

	// 1. Left status
	glyph := styles.Key.Render(d.StatusGlyph)
	status := styles.Meta.Render(d.StatusText)
	left := fmt.Sprintf(" %s %s", glyph, status)
	leftWidth := lipgloss.Width(left)

	// 2. Select hints based on breakpoint class
	var activeHints []KeyHint
	maxHints := 8
	if m.IsSmall {
		maxHints = 4 // Small terminals show at most 4 hints
	} else if m.IsMedium {
		maxHints = 6
	}

	for _, h := range d.Hints {
		if m.IsSmall && h.Priority > 1 {
			continue
		}
		activeHints = append(activeHints, h)
		if len(activeHints) >= maxHints {
			break
		}
	}

	var renderedHints []string
	for _, h := range activeHints {
		renderedHints = append(renderedHints, fmt.Sprintf("%s %s", styles.Key.Render(h.Key), styles.Hint.Render(h.Action)))
	}

	right := strings.Join(renderedHints, styles.Hint.Render(" · "))
	if len(renderedHints) > 0 {
		right += " "
	}
	rightWidth := lipgloss.Width(right)

	spaceCount := m.TotalWidth - leftWidth - rightWidth
	if spaceCount < 1 {
		// Drop lowest priority hints one by one until it fits
		for len(activeHints) > 2 && spaceCount < 1 {
			activeHints = activeHints[:len(activeHints)-1]
			var pruned []string
			for _, h := range activeHints {
				pruned = append(pruned, fmt.Sprintf("%s %s", styles.Key.Render(h.Key), styles.Hint.Render(h.Action)))
			}
			right = strings.Join(pruned, styles.Hint.Render(" · ")) + " "
			rightWidth = lipgloss.Width(right)
			spaceCount = m.TotalWidth - leftWidth - rightWidth
		}

		if spaceCount < 1 {
			// Even two hints don't fit: show status only
			right = ""
			spaceCount = max(0, m.TotalWidth-leftWidth)
		}
	}

	spacer := strings.Repeat(" ", max(0, spaceCount))
	return left + spacer + right
}
