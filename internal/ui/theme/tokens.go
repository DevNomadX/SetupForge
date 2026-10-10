package theme

import "github.com/charmbracelet/lipgloss"

// Theme encapsulates the semantic color and style tokens defined in docs/DESIGN-SYSTEM.md.
// Screens and components consume these tokens; they never hardcode raw hex values.
type Theme struct {
	Name string

	// Surfaces & Elevation
	Bg           lipgloss.TerminalColor // Workspace backdrop
	Surface      lipgloss.TerminalColor // Pane interior
	SurfaceMuted lipgloss.TerminalColor // Insets, input fields, header strips
	Overlay      lipgloss.TerminalColor // Dialogs, command palette, sheets

	// Content & Typography
	Fg      lipgloss.TerminalColor // Primary text
	FgMuted lipgloss.TerminalColor // Secondary text, metadata
	FgFaint lipgloss.TerminalColor // Hints, timestamps, disabled text

	// Brand & Focus
	Accent    lipgloss.TerminalColor // Focused border, active tab indicator, wordmark
	AccentSec lipgloss.TerminalColor // Secondary data, links, identifiers
	Focus     lipgloss.TerminalColor // Focus ring and active input bar

	// Semantic Statuses
	Success lipgloss.TerminalColor // Healthy, installed, completed
	Warning lipgloss.TerminalColor // Pending, updates available
	Error   lipgloss.TerminalColor // Destructive, failed
	Info    lipgloss.TerminalColor // Informational notes, neutral highlight

	// Structural Borders
	Border      lipgloss.TerminalColor // Default unfocused pane border
	BorderMuted lipgloss.TerminalColor // Table dividers, inner rules, collapsed rails

	// Interactive States
	SelectionFg lipgloss.TerminalColor // Selected row foreground (reverse video)
	SelectionBg lipgloss.TerminalColor // Selected row background
	Marked      lipgloss.TerminalColor // Batch selection indicator
	Disabled    lipgloss.TerminalColor // Non-interactive controls
}

// Spacing tokens (in cells / rows) per docs/DESIGN-SYSTEM.md §5.
const (
	SpaceXS = 1  // Inner rule margins, glyph padding, vertical pane padding
	SpaceSM = 2  // Default inner padding (left/right of pane content)
	SpaceMD = 4  // Between form fields, groups
	SpaceLG = 6  // Between major sections within a pane
	SpaceXL = 12 // Reserved
)

// Glyphs and ASCII fallbacks per docs/DESIGN-SYSTEM.md §8.
const (
	GlyphSelect       = "❯"
	GlyphSelectAscii  = ">"
	GlyphMarked       = "●"
	GlyphMarkedAscii  = "*"
	GlyphSuccess      = "✓"
	GlyphSuccessAscii = "+"
	GlyphError        = "✗"
	GlyphErrorAscii   = "x"
	GlyphWarning      = "⚠"
	GlyphWarningAscii = "!"
	GlyphInfo         = "ℹ"
	GlyphInfoAscii    = "i"
	GlyphRunning      = "▶"
	GlyphRunningAscii = ">"
	GlyphPending      = "○"
	GlyphPendingAscii = "o"
	GlyphRailActive   = "▌"
	GlyphRailAscii    = "|"
	GlyphBullet       = "·"
	GlyphBulletAscii  = "."
)
