package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/setupforge/setupforge/internal/ui/theme"
)

// PaneConfig specifies rendering options for a framed pane.
type PaneConfig struct {
	Title     string
	Count     string
	Position  string
	Width     int
	Height    int
	IsFocused bool
}

// RenderPane wraps content inside a standardized single-line pane frame.
// Content inside receives 1 row vertical padding and 2 columns horizontal padding.
func RenderPane(config PaneConfig, content string, styles theme.Styles) string {
	if config.Width < 4 || config.Height < 3 {
		return ""
	}

	// 1. Top border with embedded title
	topBorder := styles.RenderPanelHeader(config.Title, config.Count, config.Position, config.Width, config.IsFocused)

	// 2. Determine border styles
	borderStyle := styles.PaneBorderUnfocused
	if config.IsFocused {
		borderStyle = styles.PaneBorderFocused
	}

	leftBorder := borderStyle.Render("│")
	rightBorder := borderStyle.Render("│")

	// 3. Bottom border
	bottomFiller := strings.Repeat("─", max(0, config.Width-2))
	bottomBorder := borderStyle.Render("└" + bottomFiller + "┘")

	// 4. Content region dimensions
	contentInteriorWidth := config.Width - 2 - (theme.SpaceSM * 2) // borders + 2-col padding each side
	if contentInteriorWidth < 1 {
		contentInteriorWidth = 1
	}
	interiorHeight := config.Height - 2 // minus top and bottom borders

	// Split user content lines
	rawLines := strings.Split(content, "\n")
	var formattedContent []string

	padLeft := strings.Repeat(" ", theme.SpaceSM)
	padRight := strings.Repeat(" ", theme.SpaceSM)

	for _, line := range rawLines {
		w := lipgloss.Width(line)
		paddedLine := line
		if w < contentInteriorWidth {
			paddedLine += strings.Repeat(" ", contentInteriorWidth-w)
		} else if w > contentInteriorWidth {
			paddedLine = ansi.Truncate(line, contentInteriorWidth, "…")
			w = lipgloss.Width(paddedLine)
			if w < contentInteriorWidth {
				paddedLine += strings.Repeat(" ", contentInteriorWidth-w)
			}
		}
		formattedContent = append(formattedContent, leftBorder+padLeft+paddedLine+padRight+rightBorder)
	}

	// Assemble rows with vertical padding
	var rows []string
	rows = append(rows, topBorder)

	// Top padding row
	emptyRow := leftBorder + strings.Repeat(" ", config.Width-2) + rightBorder
	if interiorHeight > 2 {
		rows = append(rows, emptyRow)
	}

	for i := 0; i < len(formattedContent) && len(rows) < config.Height-2; i++ {
		rows = append(rows, formattedContent[i])
	}

	// Bottom padding / filler rows
	for len(rows) < config.Height-1 {
		rows = append(rows, emptyRow)
	}

	rows = append(rows, bottomBorder)
	return strings.Join(rows, "\n")
}
