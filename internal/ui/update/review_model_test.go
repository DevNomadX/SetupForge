package update

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/ui/theme"
)

func sampleTestPlan() domain.UpdatePlan {
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
			Name:      "firefox",
			Action:    domain.ActionReplaced,
			NEVRA:     "firefox-134.0-2.fc41.x86_64",
			Arch:      "x86_64",
			EVR:       "134.0-2.fc41",
			SizeBytes: 0,
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
			Name:      "old-tool",
			Action:    domain.ActionRemove,
			NEVRA:     "old-tool-1.0.0-1.fc41.x86_64",
			Arch:      "x86_64",
			EVR:       "1.0.0-1.fc41",
			OldEVR:    "1.0.0-1.fc41",
			SizeBytes: 0,
		},
		{
			Name:        "curl",
			Action:      domain.ActionDowngrade,
			NEVRA:       "curl-8.9.0-1.fc41.x86_64",
			OldNEVRA:    "curl-8.10.0-1.fc41.x86_64",
			Arch:        "x86_64",
			EVR:         "8.9.0-1.fc41",
			OldEVR:      "8.10.0-1.fc41",
			RepoID:      "fedora",
			SizeBytes:   500000,
			PackagePath: "./packages/curl.rpm",
		},
	}

	upgrades, installs, removes, replaced, downgrades, totalBytes, hasKernel, hasSystemd := domain.DeriveUpdatePlanMetadata(pkgs)

	return domain.UpdatePlan{
		OperationID: "op-test-20261001-123456",
		Artifact: domain.TransactionArtifact{
			OperationID:  "op-test-20261001-123456",
			Directory:    "/tmp/setupforge-test/artifacts/op-test-20261001-123456",
			Fingerprint:  "11223344556677889900aabbccddeeff11223344556677889900aabbccddeeff",
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
}

// 1. Conversion test: PlannedPackage -> ReviewItem
func TestReviewModel_Conversion(t *testing.T) {
	plan := sampleTestPlan()
	items := BuildReviewItems(plan.Packages)

	if len(items) != 4 {
		t.Fatalf("expected 4 presentation items (passive Replaced filtered), got %d", len(items))
	}

	// First item: firefox upgrade
	ff := items[0]
	if ff.Name != "firefox" || ff.Action != domain.ActionUpgrade {
		t.Errorf("unexpected first item: %+v", ff)
	}
	if ff.OldEVR != "134.0-2.fc41" || ff.NewEVR != "135.0-1.fc41" {
		t.Errorf("unexpected EVR pairing: old=%s new=%s", ff.OldEVR, ff.NewEVR)
	}
	if !strings.Contains(ff.ChangeSummary, "134.0-2.fc41 → 135.0-1.fc41") {
		t.Errorf("unexpected change summary: %s", ff.ChangeSummary)
	}
	if ff.FormattedSize != "71.5 MB" {
		t.Errorf("expected 71.5 MB, got %s", ff.FormattedSize)
	}
}

// 2. Counts test
func TestReviewModel_Counts(t *testing.T) {
	plan := sampleTestPlan()
	th := theme.ForgeTheme
	st := theme.NewStyles(th)
	m := NewUpdateReviewModel(th, st)

	m.SetPlan(plan)

	if m.Plan().TotalPackageCount() != 4 {
		t.Errorf("expected TotalPackageCount 4, got %d", m.Plan().TotalPackageCount())
	}
	if len(m.Items()) != m.Plan().TotalPackageCount() {
		t.Errorf("items count %d does not match TotalPackageCount %d", len(m.Items()), m.Plan().TotalPackageCount())
	}
	if m.Plan().UpgradeCount != 1 || m.Plan().InstallCount != 1 || m.Plan().RemoveCount != 1 || m.Plan().DowngradeCount != 1 {
		t.Errorf("action counts mismatch: U=%d I=%d R=%d D=%d",
			m.Plan().UpgradeCount, m.Plan().InstallCount, m.Plan().RemoveCount, m.Plan().DowngradeCount)
	}
}

// 3. Actions filtering test
func TestReviewModel_Actions(t *testing.T) {
	plan := sampleTestPlan()
	items := BuildReviewItems(plan.Packages)

	for _, item := range items {
		if item.Action == domain.ActionReplaced {
			t.Errorf("found passive ActionReplaced item which should have been filtered: %+v", item)
		}
	}
}

// 4. Download size formatting test
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 kB"},
		{153600, "150.0 kB"},
		{52428800, "50.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		got := FormatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

// 5. Fingerprint test
func TestReviewModel_Fingerprint(t *testing.T) {
	plan := sampleTestPlan()
	th := theme.ForgeTheme
	st := theme.NewStyles(th)
	m := NewUpdateReviewModel(th, st)
	m.SetDimensions(100, 30)
	m.SetPlan(plan)

	view := m.Render()
	// Fingerprint truncated or short prefix should be visible in view
	if !strings.Contains(view, "11223344") {
		t.Errorf("rendered view does not contain expected fingerprint snippet: %s", view)
	}

	// Confirm and check decision fingerprint binding
	m.focusedControl = ControlConfirmButton
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected non-nil cmd upon confirmation")
	}

	msg := cmd()
	confMsg, ok := msg.(UpdateConfirmedMsg)
	if !ok {
		t.Fatalf("expected UpdateConfirmedMsg, got %T", msg)
	}
	if confMsg.Decision.ArtifactFingerprint != plan.Artifact.Fingerprint {
		t.Errorf("expected decision fingerprint %q, got %q", plan.Artifact.Fingerprint, confMsg.Decision.ArtifactFingerprint)
	}
	if confMsg.Decision.OperationID != plan.OperationID {
		t.Errorf("expected decision operation ID %q, got %q", plan.OperationID, confMsg.Decision.OperationID)
	}
}

// 6. Confirmation test: Confirm button -> Confirmed status & UpdateConfirmedMsg
func TestReviewModel_Confirmation(t *testing.T) {
	plan := sampleTestPlan()
	th := theme.ForgeTheme
	st := theme.NewStyles(th)
	m := NewUpdateReviewModel(th, st)
	m.SetPlan(plan)

	// Tab from ControlTable -> ControlCancelButton -> ControlConfirmButton
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.FocusedControl() != ControlCancelButton {
		t.Fatalf("expected ControlCancelButton, got %v", m.FocusedControl())
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.FocusedControl() != ControlConfirmButton {
		t.Fatalf("expected ControlConfirmButton, got %v", m.FocusedControl())
	}

	// Press Enter to confirm
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.State() != ReviewStateConfirmed {
		t.Errorf("expected ReviewStateConfirmed, got %v", m.State())
	}
	if !m.Decision().IsConfirmed() {
		t.Errorf("expected Decision.IsConfirmed() == true")
	}

	msg := cmd()
	confMsg, ok := msg.(UpdateConfirmedMsg)
	if !ok || confMsg.Decision.Status != domain.ConfirmationStatusConfirmed {
		t.Errorf("unexpected message from confirm: %+v", msg)
	}
}

// 7. Cancellation test: Cancel button & Esc -> Cancelled status & UpdateCancelledMsg
func TestReviewModel_Cancellation(t *testing.T) {
	t.Run("CancelButton", func(t *testing.T) {
		plan := sampleTestPlan()
		th := theme.ForgeTheme
		st := theme.NewStyles(th)
		m := NewUpdateReviewModel(th, st)
		m.SetPlan(plan)

		// Tab to cancel button
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
		if m.FocusedControl() != ControlCancelButton {
			t.Fatalf("expected ControlCancelButton, got %v", m.FocusedControl())
		}

		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if m.State() != ReviewStateCancelled {
			t.Errorf("expected ReviewStateCancelled, got %v", m.State())
		}
		if !m.Decision().IsCancelled() {
			t.Errorf("expected Decision.IsCancelled() == true")
		}

		msg := cmd()
		cancMsg, ok := msg.(UpdateCancelledMsg)
		if !ok || cancMsg.Decision.Status != domain.ConfirmationStatusCancelled {
			t.Errorf("unexpected message from cancel: %+v", msg)
		}
	})

	t.Run("EscKey", func(t *testing.T) {
		plan := sampleTestPlan()
		th := theme.ForgeTheme
		st := theme.NewStyles(th)
		m := NewUpdateReviewModel(th, st)
		m.SetPlan(plan)

		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		if m.State() != ReviewStateCancelled {
			t.Errorf("expected ReviewStateCancelled on Esc, got %v", m.State())
		}

		msg := cmd()
		cancMsg, ok := msg.(UpdateCancelledMsg)
		if !ok || cancMsg.Decision.Status != domain.ConfirmationStatusCancelled {
			t.Errorf("unexpected message from Esc: %+v", msg)
		}
	})
}

// 8. Invalid plan test: Error handling
func TestReviewModel_InvalidPlan(t *testing.T) {
	th := theme.ForgeTheme
	st := theme.NewStyles(th)
	m := NewUpdateReviewModel(th, st)
	m.SetDimensions(80, 24)

	planErr := errors.New("transaction artifact corrupted: SHA-256 mismatch")
	m, _ = m.Update(PlanLoadedMsg{Err: planErr})

	if m.State() != ReviewStateError {
		t.Errorf("expected ReviewStateError, got %v", m.State())
	}

	view := m.Render()
	if !strings.Contains(view, "Planning Failed") || !strings.Contains(view, "SHA-256 mismatch") {
		t.Errorf("expected error message in view: %s", view)
	}

	// Press Esc to dismiss
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatalf("expected non-nil cmd on dismiss")
	}
	msg := cmd()
	if _, ok := msg.(UpdateReviewClosedMsg); !ok {
		t.Errorf("expected UpdateReviewClosedMsg, got %T", msg)
	}
}

// 9. Empty plan test
func TestReviewModel_EmptyPlan(t *testing.T) {
	th := theme.ForgeTheme
	st := theme.NewStyles(th)
	m := NewUpdateReviewModel(th, st)
	m.SetDimensions(80, 24)

	emptyPlan := domain.UpdatePlan{
		OperationID: "op-empty",
		Packages:    nil,
	}

	m, _ = m.Update(PlanLoadedMsg{Plan: emptyPlan})

	if m.State() != ReviewStateEmpty {
		t.Errorf("expected ReviewStateEmpty, got %v", m.State())
	}

	view := m.Render()
	if !strings.Contains(view, "Up to Date") {
		t.Errorf("expected up to date view, got: %s", view)
	}

	// Press Enter to dismiss
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatalf("expected non-nil cmd on dismiss")
	}
	msg := cmd()
	if _, ok := msg.(UpdateReviewClosedMsg); !ok {
		t.Errorf("expected UpdateReviewClosedMsg, got %T", msg)
	}
}

// 10. Keyboard navigation test: Tab, Shift+Tab, Arrow keys, Table cursor
func TestReviewModel_KeyboardNavigation(t *testing.T) {
	plan := sampleTestPlan()
	th := theme.ForgeTheme
	st := theme.NewStyles(th)
	m := NewUpdateReviewModel(th, st)
	m.SetDimensions(100, 30)
	m.SetPlan(plan)

	// Initial focus is Table
	if m.FocusedControl() != ControlTable {
		t.Errorf("initial focus should be ControlTable, got %v", m.FocusedControl())
	}

	// Tab -> Cancel
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.FocusedControl() != ControlCancelButton {
		t.Errorf("after Tab, expected ControlCancelButton, got %v", m.FocusedControl())
	}

	// Tab -> Confirm
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.FocusedControl() != ControlConfirmButton {
		t.Errorf("after second Tab, expected ControlConfirmButton, got %v", m.FocusedControl())
	}

	// Tab -> Table
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.FocusedControl() != ControlTable {
		t.Errorf("after third Tab, expected ControlTable, got %v", m.FocusedControl())
	}

	// Shift+Tab -> Confirm
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.FocusedControl() != ControlConfirmButton {
		t.Errorf("after Shift+Tab, expected ControlConfirmButton, got %v", m.FocusedControl())
	}

	// Left key -> Cancel
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.FocusedControl() != ControlCancelButton {
		t.Errorf("after Left, expected ControlCancelButton, got %v", m.FocusedControl())
	}

	// Right key -> Confirm
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.FocusedControl() != ControlConfirmButton {
		t.Errorf("after Right, expected ControlConfirmButton, got %v", m.FocusedControl())
	}

	// Tab back to Table
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Down arrow in Table
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.table.Cursor() != 1 {
		t.Errorf("expected table cursor 1 after KeyDown, got %d", m.table.Cursor())
	}
}

// 11. Focus safety test: Enter on table MUST NOT confirm the update
func TestReviewModel_FocusSafety(t *testing.T) {
	plan := sampleTestPlan()
	th := theme.ForgeTheme
	st := theme.NewStyles(th)
	m := NewUpdateReviewModel(th, st)
	m.SetDimensions(100, 30)
	m.SetPlan(plan)

	// Ensure Table is focused
	if m.FocusedControl() != ControlTable {
		t.Fatalf("expected ControlTable focused")
	}

	// Press Enter while Table is focused
	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// INVARIANT: State must remain Ready, decision must NOT be confirmed, cmd must NOT emit UpdateConfirmedMsg
	if m.State() != ReviewStateReady {
		t.Errorf("SAFETY VIOLATION: Enter on table changed state to %v", m.State())
	}
	if m.Decision().IsConfirmed() {
		t.Errorf("SAFETY VIOLATION: Enter on table confirmed the transaction")
	}
	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(UpdateConfirmedMsg); ok {
			t.Errorf("SAFETY VIOLATION: Enter on table dispatched UpdateConfirmedMsg")
		}
	}
}

// 12. Responsive layouts test: S (80x24), M (100x30), L (120x35), XL (160x45)
func TestReviewModel_ResponsiveLayouts(t *testing.T) {
	plan := sampleTestPlan()
	th := theme.ForgeTheme
	st := theme.NewStyles(th)

	breakpoints := []struct {
		name   string
		width  int
		height int
	}{
		{"S", 80, 24},
		{"M", 100, 30},
		{"L", 120, 35},
		{"XL", 160, 45},
	}

	for _, bp := range breakpoints {
		t.Run(bp.name, func(t *testing.T) {
			m := NewUpdateReviewModel(th, st)
			m.SetDimensions(bp.width, bp.height)
			m.SetPlan(plan)

			view := m.Render()
			lines := strings.Split(view, "\n")

			if len(lines) != bp.height {
				t.Errorf("breakpoint %s height mismatch: got %d lines, want %d", bp.name, len(lines), bp.height)
			}

			// Verify width of each line
			for i, line := range lines {
				// Each line should not exceed bp.width
				// Note: lipgloss.Width handles ANSI escape sequences
				lineWidth := lipgloss.Width(line)
				if lineWidth > bp.width {
					t.Errorf("breakpoint %s line %d width %d exceeds %d: %q", bp.name, i, lineWidth, bp.width, line)
				}
			}

			// Required elements in all layouts
			if !strings.Contains(view, "Update Transaction Review") {
				t.Errorf("breakpoint %s missing header title", bp.name)
			}
			if !strings.Contains(view, "[ Cancel ]") || !strings.Contains(view, "[ Confirm Update ]") {
				t.Errorf("breakpoint %s missing buttons", bp.name)
			}
			if !strings.Contains(view, "firefox") {
				t.Errorf("breakpoint %s missing firefox package row", bp.name)
			}
		})
	}
}
