package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/setupforge/setupforge/internal/ui/layout"
	"github.com/setupforge/setupforge/internal/ui/theme"
)

// Destination represents one of the seven primary rail targets.
type Destination struct {
	Index int
	Title string
}

// DefaultDestinations defines the 7 canonical rail destinations per docs/UI-UX-DESIGN.md §4.1.
var DefaultDestinations = []Destination{
	{Index: 1, Title: "Overview"},
	{Index: 2, Title: "Software"},
	{Index: 3, Title: "Workstation"},
	{Index: 4, Title: "Development"},
	{Index: 5, Title: "System"},
	{Index: 6, Title: "Journal"},
	{Index: 7, Title: "Settings"},
}

// RailModel manages and renders the primary navigation rail.
type RailModel struct {
	Destinations []Destination
	ActiveIndex  int // 0-indexed into Destinations (0..6)
	IsFocused    bool
}

// NewRailModel constructs a rail initialized to the Overview destination.
func NewRailModel() RailModel {
	return RailModel{
		Destinations: DefaultDestinations,
		ActiveIndex:  0,
		IsFocused:    false,
	}
}

// Render renders the rail column matching layout metrics and styles.
func (r RailModel) Render(m layout.Metrics, styles theme.Styles) string {
	if m.RailWidth <= 0 || m.StageHeight <= 0 {
		return ""
	}

	contentWidth := m.RailWidth - 1 // Leave 1 column for the vertical divider rule
	if contentWidth < 4 {
		return ""
	}

	borderStyle := styles.InnerRule
	if r.IsFocused {
		borderStyle = styles.PaneBorderFocused
	}

	divider := borderStyle.Render("│")

	var lines []string

	// Optional top padding row
	lines = append(lines, strings.Repeat(" ", contentWidth)+divider)

	for i, dest := range r.Destinations {
		isActive := i == r.ActiveIndex

		var row string
		if isActive {
			indicator := styles.Key.Render(theme.GlyphRailActive)
			label := styles.RailActive.Render(fmt.Sprintf("%d %s", dest.Index, dest.Title))
			row = indicator + " " + label
		} else {
			label := styles.RailInactive.Render(fmt.Sprintf("  %d %s", dest.Index, dest.Title))
			row = label
		}

		// Truncate or pad to exactly contentWidth
		rawLen := lipgloss.Width(row)
		if rawLen < contentWidth {
			row += strings.Repeat(" ", contentWidth-rawLen)
		} else if rawLen > contentWidth {
			// Squeeze label if needed
			shortLabel := fmt.Sprintf("%d %s", dest.Index, dest.Title)
			if lipgloss.Width(shortLabel) > contentWidth-2 {
				shortLabel = fmt.Sprintf("%d %s", dest.Index, dest.Title[:max(1, contentWidth-5)]+"…")
			}
			if isActive {
				row = styles.Key.Render(theme.GlyphRailActive) + " " + styles.RailActive.Render(shortLabel)
			} else {
				row = styles.RailInactive.Render("  " + shortLabel)
			}
			rawLen = lipgloss.Width(row)
			if rawLen < contentWidth {
				row += strings.Repeat(" ", contentWidth-rawLen)
			}
		}

		lines = append(lines, row+divider)
	}

	// Fill remaining stage rows with blank lines + divider
	for len(lines) < m.StageHeight {
		lines = append(lines, strings.Repeat(" ", contentWidth)+divider)
	}

	// Truncate if we exceeded stage height
	if len(lines) > m.StageHeight {
		lines = lines[:m.StageHeight]
	}

	return strings.Join(lines, "\n")
}
