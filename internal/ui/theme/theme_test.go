package theme_test

import (
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"
	"github.com/setupforge/setupforge/internal/ui/theme"
)

func TestGetTheme(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"forge", "forge"},
		{"FORGE", "forge"},
		{"rose-pine", "rose-pine"},
		{"rosepine", "rose-pine"},
		{"gruvbox", "gruvbox-dark"},
		{"gruvbox-dark", "gruvbox-dark"},
		{"unknown", "forge"},
		{"", "forge"},
	}

	for _, tc := range tests {
		th := theme.GetTheme(tc.input)
		if th.Name != tc.expected {
			t.Errorf("GetTheme(%q) = %q, expected %q", tc.input, th.Name, tc.expected)
		}
	}
}

func TestRenderPanelHeader(t *testing.T) {
	th := theme.ForgeTheme
	styles := theme.NewStyles(th)

	width := 60
	header := styles.RenderPanelHeader("Packages", "142 available", "12 of 142", width, false)

	// Strip ANSI sequences to verify display width
	stripped := stripANSI(header)
	actualWidth := runewidth.StringWidth(stripped)

	if actualWidth != width {
		t.Errorf("expected header display width %d, got %d (rendered: %q)", width, actualWidth, stripped)
	}

	if !strings.HasPrefix(stripped, "┌─ Packages") {
		t.Errorf("expected header to start with '┌─ Packages', got %q", stripped)
	}

	if !strings.HasSuffix(stripped, "─┐") {
		t.Errorf("expected header to end with '─┐', got %q", stripped)
	}

	if !strings.Contains(stripped, "142 available") {
		t.Errorf("expected header to contain count '142 available', got %q", stripped)
	}

	if !strings.Contains(stripped, "12 of 142") {
		t.Errorf("expected header to contain pos '12 of 142', got %q", stripped)
	}
}

func stripANSI(s string) string {
	var b strings.Builder
	inSeq := false
	for _, r := range s {
		if r == 0x1b { // ESC
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
