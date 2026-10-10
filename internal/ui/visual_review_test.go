package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/termenv"
	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/ui/pages"
	"github.com/setupforge/setupforge/internal/ui/update"
)

func TestVisualReview_LayoutDimensions(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	sizes := []struct {
		name         string
		width        int
		height       int
		expectDual   bool
		checkFullTxt bool
	}{
		{"Small 80x24", 80, 24, false, false},
		{"Medium 100x30", 100, 30, false, false},
		{"Large 120x35", 120, 35, true, false},
		{"ExtraLarge 160x45", 160, 45, true, true},
		{"ExtraLarge 180x50", 180, 50, true, true},
		{"ExtraLarge 200x50", 200, 50, true, true},
	}

	for _, sz := range sizes {
		t.Run(sz.name, func(t *testing.T) {
			app := setupTestApp()
			m, _ := app.Update(tea.WindowSizeMsg{Width: sz.width, Height: sz.height})
			for _, msg := range executeCmd(m.Init()) {
				m, _ = m.Update(msg)
			}

			view := m.View()
			lines := strings.Split(view, "\n")

			if len(lines) != sz.height {
				t.Fatalf("expected %d rows, got %d", sz.height, len(lines))
			}

			for i, line := range lines {
				stripped := stripANSI(line)
				w := runewidth.StringWidth(stripped)
				if w > sz.width {
					t.Errorf("row %d exceeded width: expected <= %d, got %d (line: %q)", i, sz.width, w, stripped)
				}

				// Check for stray internal borders in dual-pane stage rows
				// A dual pane row with rail should have:
				// 1 rail divider + 1 left pane left border + 1 left pane right border + 1 right pane left border + 1 right pane right border = 5 pipes
				// A dual pane row without rail (overlay) has 4 pipes
				// A single pane row has 2 pipes (or 3 if rail attached)
				if sz.expectDual && i > 1 && i < sz.height-2 {
					pipeCount := strings.Count(stripped, "│")
					expectedPipes := 5 // rail divider (1) + 2 panes * 2 borders (4)
					if pipeCount != expectedPipes {
						t.Errorf("row %d has unexpected pipe count %d (expected %d): %q", i, pipeCount, expectedPipes, stripped)
					}
				}
			}

			// On XL screens (160+, 180+, 200+), verify full descriptions are rendered without ellipsis
			if sz.checkFullTxt {
				if strings.Contains(view, "Browse packages, Flatpaks, and…") || strings.Contains(view, "Browse packages, Flatpaks, and rep ") {
					t.Errorf("[%s] Launchpad item description was clipped or truncated when room was available", sz.name)
				}
				if !strings.Contains(view, "Browse packages, Flatpaks, and repositories") {
					t.Errorf("[%s] Expected full description 'Browse packages, Flatpaks, and repositories' on XL terminal", sz.name)
				}
			}
		})
	}
}

func TestVisualReview_PrintSnapshots(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	sizes := []struct {
		w, h int
	}{
		{80, 24},
		{100, 30},
		{120, 35},
		{160, 45},
		{180, 50},
		{200, 50},
	}
	for _, s := range sizes {
		app := setupTestApp()
		m, _ := app.Update(tea.WindowSizeMsg{Width: s.w, Height: s.h})
		cmd := m.Init()
		msg := cmd()
		m, _ = m.Update(msg)
		view := m.View()
		t.Logf("Snapshot (%dx%d):\n%s", s.w, s.h, stripANSI(view))
	}
}

func TestVisualReview_UpdatePlanPresentation(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)

	pkgs := []domain.PlannedPackage{
		{
			Name:        "firefox",
			Action:      domain.ActionUpgrade,
			NEVRA:       "firefox-135.0-1.fc41.x86_64",
			OldNEVRA:    "firefox-134.0-2.fc41.x86_64",
			Arch:        "x86_64",
			EVR:         "135.0-1.fc41",
			OldEVR:      "134.0-2.fc41",
			RepoID:      "updates",
			SizeBytes:   75000000,
			PackagePath: "./packages/firefox.rpm",
		},
		{
			Name:        "kernel",
			Action:      domain.ActionInstall,
			NEVRA:       "kernel-6.13.0-200.fc41.x86_64",
			Arch:        "x86_64",
			EVR:         "6.13.0-200.fc41",
			RepoID:      "updates",
			SizeBytes:   15000000,
			PackagePath: "./packages/kernel.rpm",
		},
		{
			Name:        "glibc",
			Action:      domain.ActionUpgrade,
			NEVRA:       "glibc-2.40-4.fc41.x86_64",
			OldNEVRA:    "glibc-2.40-3.fc41.x86_64",
			Arch:        "x86_64",
			EVR:         "2.40-4.fc41",
			OldEVR:      "2.40-3.fc41",
			RepoID:      "updates",
			SizeBytes:   2200000,
			PackagePath: "./packages/glibc.rpm",
		},
	}

	upgrades, installs, removes, replaced, downgrades, totalBytes, hasKernel, hasSystemd := domain.DeriveUpdatePlanMetadata(pkgs)
	plan := domain.UpdatePlan{
		OperationID: "op-visual-qa-999",
		Artifact: domain.TransactionArtifact{
			OperationID:  "op-visual-qa-999",
			Directory:    "/tmp/artifacts/op-visual-qa-999",
			Fingerprint:  "deadbeef1234567890abcdef1234567890abcdef1234567890abcdef12345678",
			CreatedAt:    time.Now(),
			PackageCount: 3,
			TotalBytes:   totalBytes,
		},
		Packages:           pkgs,
		UpgradeCount:       upgrades,
		InstallCount:       installs,
		RemoveCount:        removes,
		ReplacedCount:      replaced,
		DowngradeCount:     downgrades,
		TotalDownloadBytes: totalBytes,
		HasKernelUpdate:    hasKernel,
		HasSystemdUpdate:   hasSystemd,
		CreatedAt:          time.Now(),
	}

	sizes := []struct {
		name string
		w, h int
	}{
		{"S", 80, 24},
		{"M", 100, 30},
		{"L", 120, 35},
		{"XL", 160, 45},
		{"XL-Wide", 180, 50},
		{"XL-Ultra", 200, 50},
	}

	for _, s := range sizes {
		t.Run(s.name, func(t *testing.T) {
			app := setupTestAppWithPlanner(plan)
			m, _ := app.Update(tea.WindowSizeMsg{Width: s.w, Height: s.h})

			// Request update review and load plan
			m, _ = m.Update(pages.RequestUpdateReviewMsg{})
			m, _ = m.Update(update.PlanLoadedMsg{Plan: plan})

			view := m.View()
			lines := strings.Split(view, "\n")

			if len(lines) != s.h {
				t.Fatalf("[%s] expected %d rows, got %d", s.name, s.h, len(lines))
			}

			for i, line := range lines {
				stripped := stripANSI(line)
				w := runewidth.StringWidth(stripped)
				if w > s.w {
					t.Errorf("[%s] row %d exceeded width: expected <= %d, got %d (line: %q)", s.name, i, s.w, w, stripped)
				}
			}

			if !strings.Contains(view, "Update Transaction Review") {
				t.Errorf("[%s] view missing review header", s.name)
			}
			if !strings.Contains(view, "firefox") || !strings.Contains(view, "kernel") {
				t.Errorf("[%s] view missing package names", s.name)
			}
			if !strings.Contains(view, "[ Confirm Update ]") {
				t.Errorf("[%s] view missing confirm button", s.name)
			}

			t.Logf("Snapshot Update Review (%dx%d):\n%s", s.w, s.h, stripANSI(view))
		})
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
