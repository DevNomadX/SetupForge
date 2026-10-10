package components_test

import (
	"strings"
	"testing"
	"time"

	"github.com/setupforge/setupforge/internal/ui/components"
	"github.com/setupforge/setupforge/internal/ui/layout"
	"github.com/setupforge/setupforge/internal/ui/theme"
)

func TestHeaderModel_Render(t *testing.T) {
	styles := theme.NewStyles(theme.ForgeTheme)
	h := components.NewHeaderModel()
	h.DistroBadge = "Fedora Linux 44"
	h.UpdateCount = 3
	h.TimeProvider = func() time.Time {
		return time.Date(2026, 9, 29, 14, 30, 0, 0, time.UTC)
	}

	t.Run("Large Screen 120x35", func(t *testing.T) {
		m := layout.Calculate(120, 35, false)
		rendered := h.Render(m, styles)

		if !strings.Contains(rendered, "SetupForge") {
			t.Errorf("expected header to contain 'SetupForge', got %q", rendered)
		}
		if !strings.Contains(rendered, "Overview") {
			t.Errorf("expected header to contain 'Overview', got %q", rendered)
		}
		if !strings.Contains(rendered, "Fedora Linux 44") {
			t.Errorf("expected header to contain 'Fedora Linux 44', got %q", rendered)
		}
		if !strings.Contains(rendered, "3 updates") {
			t.Errorf("expected header to contain update badge '3 updates', got %q", rendered)
		}
		if !strings.Contains(rendered, "14:30") {
			t.Errorf("expected header to contain clock '14:30', got %q", rendered)
		}
	})

	t.Run("Small Screen 80x24", func(t *testing.T) {
		m := layout.Calculate(80, 24, false)
		rendered := h.Render(m, styles)

		if !strings.Contains(rendered, "SetupForge") {
			t.Errorf("expected header to contain 'SetupForge', got %q", rendered)
		}
		// In small screen, clock and update badge should be dropped per design spec
		if strings.Contains(rendered, "14:30") {
			t.Errorf("expected clock to be dropped in small screen, got %q", rendered)
		}
	})
}

func TestRailModel_Render(t *testing.T) {
	styles := theme.NewStyles(theme.ForgeTheme)
	r := components.NewRailModel()
	r.ActiveIndex = 0

	m := layout.Calculate(120, 35, false)
	rendered := r.Render(m, styles)

	lines := strings.Split(rendered, "\n")
	if len(lines) != m.StageHeight {
		t.Errorf("expected rail lines %d, got %d", m.StageHeight, len(lines))
	}

	if !strings.Contains(rendered, "1 Overview") {
		t.Errorf("expected rail to contain '1 Overview', got %q", rendered)
	}
	if !strings.Contains(rendered, "7 Settings") {
		t.Errorf("expected rail to contain '7 Settings', got %q", rendered)
	}
	if !strings.Contains(rendered, theme.GlyphRailActive) {
		t.Errorf("expected active indicator glyph %q in rail", theme.GlyphRailActive)
	}
}

func TestDockModel_Render(t *testing.T) {
	styles := theme.NewStyles(theme.ForgeTheme)
	d := components.NewDockModel()
	d.StatusText = "System Ready"

	t.Run("Large Screen hints", func(t *testing.T) {
		m := layout.Calculate(120, 35, false)
		rendered := d.Render(m, styles)

		if !strings.Contains(rendered, "System Ready") {
			t.Errorf("expected dock to contain status 'System Ready', got %q", rendered)
		}
		if !strings.Contains(rendered, "1-7") || !strings.Contains(rendered, "quit") {
			t.Errorf("expected primary key hints in dock, got %q", rendered)
		}
	})

	t.Run("Small Screen priority cap", func(t *testing.T) {
		m := layout.Calculate(80, 24, false)
		rendered := d.Render(m, styles)

		if !strings.Contains(rendered, "System Ready") {
			t.Errorf("expected dock to contain status 'System Ready', got %q", rendered)
		}
	})
}

func TestRenderPane(t *testing.T) {
	styles := theme.NewStyles(theme.ForgeTheme)
	content := "Line 1\nLine 2"

	pane := components.RenderPane(components.PaneConfig{
		Title:     "Test Pane",
		Count:     "2 items",
		Width:     40,
		Height:    8,
		IsFocused: true,
	}, content, styles)

	lines := strings.Split(pane, "\n")
	if len(lines) != 8 {
		t.Errorf("expected pane height 8, got %d", len(lines))
	}

	if !strings.Contains(lines[0], "Test Pane") {
		t.Errorf("expected header to contain 'Test Pane', got %q", lines[0])
	}
	if !strings.Contains(lines[len(lines)-1], "┘") {
		t.Errorf("expected bottom border with '┘', got %q", lines[len(lines)-1])
	}
}
