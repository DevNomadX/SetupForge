package pages

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/setupforge/setupforge/internal/ui/components"
	"github.com/setupforge/setupforge/internal/ui/layout"
	"github.com/setupforge/setupforge/internal/ui/theme"
)

// PlaceholderModel provides a standardized temporary view for upcoming roadmap modules.
type PlaceholderModel struct {
	Title       string
	Subtitle    string
	PhaseInfo   string
	Description string
	IsFocused   bool
}

// NewPlaceholderModel creates a placeholder page for a destination.
func NewPlaceholderModel(title, subtitle, phaseInfo, description string) PlaceholderModel {
	return PlaceholderModel{
		Title:       title,
		Subtitle:    subtitle,
		PhaseInfo:   phaseInfo,
		Description: description,
		IsFocused:   true,
	}
}

// Update handles messages for the placeholder page.
func (m PlaceholderModel) Update(msg tea.Msg) (PlaceholderModel, tea.Cmd) {
	return m, nil
}

// Render renders the placeholder page inside a standardized Whole pane.
func (m PlaceholderModel) Render(metrics layout.Metrics, styles theme.Styles) string {
	if metrics.StageWidth <= 0 || metrics.StageHeight <= 0 {
		return ""
	}

	var contentLines []string
	contentLines = append(contentLines, styles.Heading.Render(m.Subtitle))
	contentLines = append(contentLines, "")
	contentLines = append(contentLines, styles.Meta.Render(m.Description))
	contentLines = append(contentLines, "")

	status := fmt.Sprintf("%s %s", styles.Key.Render(theme.GlyphPending), styles.Hint.Render(m.PhaseInfo))
	contentLines = append(contentLines, status)
	contentLines = append(contentLines, "")
	contentLines = append(contentLines, styles.Hint.Render("Press 1 to return to Overview, or Tab to cycle focus."))

	return components.RenderPane(components.PaneConfig{
		Title:     m.Title,
		Count:     m.PhaseInfo,
		Width:     metrics.StageWidth,
		Height:    metrics.StageHeight,
		IsFocused: m.IsFocused,
	}, strings.Join(contentLines, "\n"), styles)
}
