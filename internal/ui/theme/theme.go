package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ForgeTheme is the default SetupForge theme: warm graphite surfaces, cream foreground, copper accent.
var ForgeTheme = Theme{
	Name:         "forge",
	Bg:           lipgloss.Color("#1b1a18"),
	Surface:      lipgloss.Color("#232120"),
	SurfaceMuted: lipgloss.Color("#2a2826"),
	Overlay:      lipgloss.Color("#2e2c29"),
	Fg:           lipgloss.Color("#e6e1d8"),
	FgMuted:      lipgloss.Color("#a8a29a"),
	FgFaint:      lipgloss.Color("#6b6660"),
	Accent:       lipgloss.Color("#d08743"),
	AccentSec:    lipgloss.Color("#8fbbb7"),
	Focus:        lipgloss.Color("#d08743"),
	Success:      lipgloss.Color("#a0b56a"),
	Warning:      lipgloss.Color("#e0a84f"),
	Error:        lipgloss.Color("#d05c5c"),
	Info:         lipgloss.Color("#7f9cbc"),
	Border:       lipgloss.Color("#3a3733"),
	BorderMuted:  lipgloss.Color("#2c2a27"),
	SelectionFg:  lipgloss.Color("#e6e1d8"),
	SelectionBg:  lipgloss.Color("#4a4238"),
	Marked:       lipgloss.Color("#d08743"),
	Disabled:     lipgloss.Color("#6b6660"),
}

// RosePineTheme maps the Rosé Pine palette to SetupForge semantic tokens.
var RosePineTheme = Theme{
	Name:         "rose-pine",
	Bg:           lipgloss.Color("#191724"),
	Surface:      lipgloss.Color("#1f1d2e"),
	SurfaceMuted: lipgloss.Color("#26233a"),
	Overlay:      lipgloss.Color("#26233a"),
	Fg:           lipgloss.Color("#e0def4"),
	FgMuted:      lipgloss.Color("#908caa"),
	FgFaint:      lipgloss.Color("#6e6a86"),
	Accent:       lipgloss.Color("#ebbcba"),
	AccentSec:    lipgloss.Color("#9ccfd8"),
	Focus:        lipgloss.Color("#ebbcba"),
	Success:      lipgloss.Color("#31748f"),
	Warning:      lipgloss.Color("#f6c177"),
	Error:        lipgloss.Color("#eb6f92"),
	Info:         lipgloss.Color("#c4a7e7"),
	Border:       lipgloss.Color("#403d52"),
	BorderMuted:  lipgloss.Color("#26233a"),
	SelectionFg:  lipgloss.Color("#e0def4"),
	SelectionBg:  lipgloss.Color("#393552"),
	Marked:       lipgloss.Color("#ebbcba"),
	Disabled:     lipgloss.Color("#6e6a86"),
}

// GruvboxDarkTheme maps Gruvbox Dark to SetupForge semantic tokens.
var GruvboxDarkTheme = Theme{
	Name:         "gruvbox-dark",
	Bg:           lipgloss.Color("#282828"),
	Surface:      lipgloss.Color("#3c3836"),
	SurfaceMuted: lipgloss.Color("#32302f"),
	Overlay:      lipgloss.Color("#3c3836"),
	Fg:           lipgloss.Color("#ebdbb2"),
	FgMuted:      lipgloss.Color("#a89984"),
	FgFaint:      lipgloss.Color("#928374"),
	Accent:       lipgloss.Color("#fe8019"),
	AccentSec:    lipgloss.Color("#8ec07c"),
	Focus:        lipgloss.Color("#fe8019"),
	Success:      lipgloss.Color("#b8bb26"),
	Warning:      lipgloss.Color("#fabd2f"),
	Error:        lipgloss.Color("#fb4934"),
	Info:         lipgloss.Color("#83a598"),
	Border:       lipgloss.Color("#504945"),
	BorderMuted:  lipgloss.Color("#3c3836"),
	SelectionFg:  lipgloss.Color("#ebdbb2"),
	SelectionBg:  lipgloss.Color("#504945"),
	Marked:       lipgloss.Color("#fe8019"),
	Disabled:     lipgloss.Color("#928374"),
}

// GetTheme retrieves a theme by name, returning ForgeTheme if unrecognized.
func GetTheme(name string) Theme {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "rose-pine", "rosepine":
		return RosePineTheme
	case "gruvbox", "gruvbox-dark", "gruvbox_dark":
		return GruvboxDarkTheme
	default:
		return ForgeTheme
	}
}
