package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Styles bundles preconfigured Lip Gloss styles parameterized by a Theme.
type Styles struct {
	Theme Theme

	// Typography
	Brand   lipgloss.Style
	Title   lipgloss.Style
	Heading lipgloss.Style
	Body    lipgloss.Style
	Data    lipgloss.Style
	Meta    lipgloss.Style
	Hint    lipgloss.Style
	Key     lipgloss.Style

	// Semantic Statuses
	StatusSuccess lipgloss.Style
	StatusWarning lipgloss.Style
	StatusError   lipgloss.Style
	StatusInfo    lipgloss.Style

	// Interactive States
	Selection lipgloss.Style
	Disabled  lipgloss.Style

	// Rail styles
	RailActive   lipgloss.Style
	RailInactive lipgloss.Style

	// Pane borders
	PaneBorderFocused   lipgloss.Style
	PaneBorderUnfocused lipgloss.Style
	InnerRule           lipgloss.Style
}

// NewStyles builds a Styles set bound to the given Theme.
func NewStyles(t Theme) Styles {
	return Styles{
		Theme: t,

		Brand: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Accent),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Fg),

		Heading: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Fg),

		Body: lipgloss.NewStyle().
			Foreground(t.Fg),

		Data: lipgloss.NewStyle().
			Foreground(t.AccentSec),

		Meta: lipgloss.NewStyle().
			Foreground(t.FgMuted),

		Hint: lipgloss.NewStyle().
			Foreground(t.FgFaint),

		Key: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Accent),

		StatusSuccess: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Success),

		StatusWarning: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Warning),

		StatusError: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Error),

		StatusInfo: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Info),

		Selection: lipgloss.NewStyle().
			Foreground(t.SelectionFg).
			Background(t.SelectionBg),

		Disabled: lipgloss.NewStyle().
			Foreground(t.Disabled),

		RailActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Fg),

		RailInactive: lipgloss.NewStyle().
			Foreground(t.FgMuted),

		PaneBorderFocused: lipgloss.NewStyle().
			Foreground(t.Focus),

		PaneBorderUnfocused: lipgloss.NewStyle().
			Foreground(t.Border),

		InnerRule: lipgloss.NewStyle().
			Foreground(t.BorderMuted),
	}
}

// RenderPanelHeader constructs SetupForge's signature embedded top border:
// Format: ┌─ Title ─ count ─ pos ─┐
// When focused, the border and title adopt the accent/focus color.
func (s Styles) RenderPanelHeader(title, count, pos string, width int, focused bool) string {
	borderColor := s.Theme.Border
	titleStyle := s.Title
	if focused {
		borderColor = s.Theme.Focus
		titleStyle = titleStyle.Foreground(s.Theme.Focus)
	}

	borderStyle := lipgloss.NewStyle().Foreground(borderColor)

	// Minimal fallback if width is very small
	if width < 8 {
		return borderStyle.Render("┌" + strings.Repeat("─", max(0, width-2)) + "┐")
	}

	left := "┌─ "
	right := "─┐"

	var parts []string
	if title != "" {
		parts = append(parts, titleStyle.Render(title))
	}
	if count != "" {
		parts = append(parts, s.Meta.Render(count))
	}

	leftContent := strings.Join(parts, " "+borderStyle.Render("─")+" ")
	leftWidth := lipgloss.Width(left) + lipgloss.Width(leftContent)

	rightWidth := 0
	rightContent := ""
	if pos != "" {
		rightContent = borderStyle.Render("─ ") + s.Meta.Render(pos) + " "
		rightWidth = lipgloss.Width(rightContent)
	}
	rightWidth += lipgloss.Width(right)

	fillerCount := width - leftWidth - rightWidth
	if fillerCount < 1 {
		// Not enough room for right content or count, simplify to fit
		leftContent = titleStyle.Render(title)
		leftWidth = lipgloss.Width(left) + lipgloss.Width(leftContent)
		rightContent = ""
		rightWidth = lipgloss.Width(right)
		fillerCount = width - leftWidth - rightWidth
		if fillerCount < 0 {
			// Title too wide, just render simple border
			return borderStyle.Render("┌" + strings.Repeat("─", max(0, width-2)) + "┐")
		}
	}

	filler := borderStyle.Render(strings.Repeat("─", fillerCount))
	return borderStyle.Render(left) + leftContent + filler + rightContent + borderStyle.Render(right)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
