package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/setupforge/setupforge/internal/ui/layout"
	"github.com/setupforge/setupforge/internal/ui/theme"
)

// HeaderModel renders Row 0 of SetupForge workspace.
type HeaderModel struct {
	BrandName    string
	Breadcrumb   string
	DistroBadge  string
	UpdateCount  int
	TimeProvider func() time.Time
}

// NewHeaderModel creates a new header model.
func NewHeaderModel() HeaderModel {
	return HeaderModel{
		BrandName:    "SetupForge",
		Breadcrumb:   "Overview",
		DistroBadge:  "Fedora",
		UpdateCount:  0,
		TimeProvider: time.Now,
	}
}

// Render renders the 1-row header matching layout constraints and current theme.
func (h HeaderModel) Render(m layout.Metrics, styles theme.Styles) string {
	if m.TotalWidth <= 0 {
		return ""
	}

	// 1. Left segment: Wordmark + Breadcrumb
	wordmark := styles.Brand.Render(h.BrandName)
	bc := styles.Meta.Render(h.Breadcrumb)
	left := fmt.Sprintf(" %s  %s", wordmark, bc)
	leftWidth := lipgloss.Width(left)

	// 2. Right cluster: Distro badge, updates, clock
	var rightParts []string

	if h.DistroBadge != "" && !m.IsSmall {
		rightParts = append(rightParts, styles.Data.Render(h.DistroBadge))
	}

	if h.UpdateCount > 0 && !m.IsSmall {
		badge := fmt.Sprintf("%d updates %s", h.UpdateCount, theme.GlyphMarked)
		rightParts = append(rightParts, styles.StatusWarning.Render(badge))
	}

	if m.IsLarge || m.IsExtraLarge {
		t := h.TimeProvider().Format("15:04")
		rightParts = append(rightParts, styles.Hint.Render(t))
	}

	right := strings.Join(rightParts, styles.Hint.Render(" · "))
	if len(rightParts) > 0 {
		right += " "
	}
	rightWidth := lipgloss.Width(right)

	// Calculate space between
	spaceCount := m.TotalWidth - leftWidth - rightWidth
	if spaceCount < 1 {
		// Terminal is very narrow: truncate breadcrumb or show wordmark only
		if m.TotalWidth > lipgloss.Width(wordmark)+4 {
			left = fmt.Sprintf(" %s", wordmark)
			spaceCount = m.TotalWidth - lipgloss.Width(left) - rightWidth
			if spaceCount < 0 {
				right = ""
				spaceCount = max(0, m.TotalWidth-lipgloss.Width(left))
			}
		} else {
			return styles.Brand.Render(h.BrandName[:min(len(h.BrandName), m.TotalWidth)])
		}
	}

	spacer := strings.Repeat(" ", max(0, spaceCount))
	return left + spacer + right
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
