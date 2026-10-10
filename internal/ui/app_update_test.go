package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/setupforge/setupforge/internal/app"
	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/ui/pages"
	"github.com/setupforge/setupforge/internal/ui/update"
)

// fakeUpdateChecker provides deterministic update status for UI tests.
type fakeUpdateChecker struct {
	status domain.UpdateStatus
	err    error
}

func (f *fakeUpdateChecker) Check(ctx context.Context, opts domain.CheckOptions) (domain.UpdateStatus, error) {
	return f.status, f.err
}

// fakeUpdatePlanner provides deterministic artifacts and plans for UI tests.
type fakeUpdatePlanner struct {
	artifact domain.TransactionArtifact
	plan     domain.UpdatePlan
	err      error
}

func (f *fakeUpdatePlanner) Plan(ctx context.Context, opID string) (domain.TransactionArtifact, domain.UpdatePlan, error) {
	if f.err != nil {
		return domain.TransactionArtifact{}, domain.UpdatePlan{}, f.err
	}
	art := f.artifact
	art.OperationID = opID
	pln := f.plan
	pln.OperationID = opID
	pln.Artifact = art
	return art, pln, nil
}

func (f *fakeUpdatePlanner) ValidateArtifact(art domain.TransactionArtifact) error {
	if art.Fingerprint != f.artifact.Fingerprint {
		return domain.ErrTransactionArtifactCorrupted
	}
	return nil
}

func setupTestAppWithPlanner(t *testing.T) (AppModel, domain.UpdatePlan) {
	t.Helper()

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
	}

	upgrades, installs, removes, replaced, downgrades, totalBytes, hasKernel, hasSystemd := domain.DeriveUpdatePlanMetadata(pkgs)

	artifact := domain.TransactionArtifact{
		OperationID:  "op-app-test-123",
		Directory:    "/tmp/test-artifact",
		Fingerprint:  "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
		CreatedAt:    time.Now(),
		PackageCount: 2,
		TotalBytes:   totalBytes,
	}

	plan := domain.UpdatePlan{
		OperationID:        "op-app-test-123",
		Artifact:           artifact,
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

	checker := &fakeUpdateChecker{
		status: domain.UpdateStatus{
			State:       domain.UpdateStateAvailable,
			UpdateCount: 2,
			LastChecked: time.Now(),
		},
	}

	planner := &fakeUpdatePlanner{
		artifact: artifact,
		plan:     plan,
	}

	updateSvc := app.NewUpdateService(checker, app.WithUpdatePlanner(planner))
	appModel := NewAppModel(nil, updateSvc)

	// Simulate initial update status loaded
	m, _ := appModel.Update(UpdateStatusLoadedMsg{
		Status: checker.status,
	})
	return m.(AppModel), plan
}

// TestAppModel_UpdateReview_OpenOnKeyU verifies pressing 'u' opens the update review modal.
func TestAppModel_UpdateReview_OpenOnKeyU(t *testing.T) {
	appModel, _ := setupTestAppWithPlanner(t)

	// Press 'u'
	m, cmd := appModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if cmd == nil {
		t.Fatalf("expected command on pressing 'u'")
	}

	// Command should dispatch RequestUpdateReviewMsg
	msg := cmd()
	if _, ok := msg.(pages.RequestUpdateReviewMsg); !ok {
		t.Fatalf("expected RequestUpdateReviewMsg, got %T", msg)
	}

	// Pass RequestUpdateReviewMsg to Update
	m, planCmd := m.Update(msg)
	appModel = m.(AppModel)

	if !appModel.ReviewModalOpen() {
		t.Errorf("expected reviewModalOpen == true")
	}
	if appModel.ReviewModel().State() != update.ReviewStateLoading {
		t.Errorf("expected reviewModel state == ReviewStateLoading, got %v", appModel.ReviewModel().State())
	}
	if planCmd == nil {
		t.Fatalf("expected planCmd to be dispatched")
	}

	// Execute planning command asynchronously
	planMsg := planCmd()
	loadedMsg, ok := planMsg.(update.PlanLoadedMsg)
	if !ok {
		t.Fatalf("expected PlanLoadedMsg, got %T", planMsg)
	}
	if loadedMsg.Err != nil {
		t.Fatalf("unexpected planning error: %v", loadedMsg.Err)
	}

	// Update app with loaded plan
	m, _ = appModel.Update(loadedMsg)
	appModel = m.(AppModel)

	if appModel.ReviewModel().State() != update.ReviewStateReady {
		t.Errorf("expected state ReviewStateReady, got %v", appModel.ReviewModel().State())
	}
	if len(appModel.ReviewModel().Items()) != 2 {
		t.Errorf("expected 2 items, got %d", len(appModel.ReviewModel().Items()))
	}
}

// TestAppModel_UpdateReview_FocusTrap verifies that keystrokes are trapped by the review modal.
func TestAppModel_UpdateReview_FocusTrap(t *testing.T) {
	appModel, plan := setupTestAppWithPlanner(t)

	// Open modal and load plan
	m, _ := appModel.Update(pages.RequestUpdateReviewMsg{})
	m, _ = m.Update(update.PlanLoadedMsg{Plan: plan})
	appModel = m.(AppModel)

	if !appModel.ReviewModalOpen() {
		t.Fatalf("expected modal open")
	}

	// Send destination jump '2' - modal focus trap should NOT change destination!
	m, _ = appModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	appModel = m.(AppModel)

	if appModel.activeDestination != 0 {
		t.Errorf("focus trap failed: active destination changed to %d", appModel.activeDestination)
	}
}

// TestAppModel_UpdateReview_ExplicitConfirmation verifies confirmation flow and safety invariant.
func TestAppModel_UpdateReview_ExplicitConfirmation(t *testing.T) {
	appModel, plan := setupTestAppWithPlanner(t)

	// Open and load
	m, _ := appModel.Update(pages.RequestUpdateReviewMsg{})
	m, _ = m.Update(update.PlanLoadedMsg{Plan: plan})
	appModel = m.(AppModel)

	// Cycle focus to Confirm Button: Tab -> Cancel, Tab -> Confirm
	m, _ = appModel.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	appModel = m.(AppModel)

	if appModel.ReviewModel().FocusedControl() != update.ControlConfirmButton {
		t.Fatalf("expected Confirm Button focused, got %v", appModel.ReviewModel().FocusedControl())
	}

	// Press Enter to confirm
	m, confirmCmd := appModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if confirmCmd == nil {
		t.Fatalf("expected confirmCmd from Enter on confirm button")
	}

	msg := confirmCmd()
	confMsg, ok := msg.(update.UpdateConfirmedMsg)
	if !ok {
		t.Fatalf("expected UpdateConfirmedMsg, got %T", msg)
	}

	// Deliver UpdateConfirmedMsg back to AppModel
	m, _ = m.Update(confMsg)
	appModel = m.(AppModel)

	// Validate Decision invariants
	if !appModel.LastDecision().IsConfirmed() {
		t.Errorf("expected LastDecision().IsConfirmed() == true")
	}
	if appModel.LastDecision().ArtifactFingerprint != plan.Artifact.Fingerprint {
		t.Errorf("expected fingerprint match: got %s, want %s",
			appModel.LastDecision().ArtifactFingerprint, plan.Artifact.Fingerprint)
	}
	if appModel.LastDecision().OperationID != plan.OperationID {
		t.Errorf("expected operation ID match: got %s, want %s",
			appModel.LastDecision().OperationID, plan.OperationID)
	}

	// MANDATORY SAFETY INVARIANT: Phase 2B.3 not started
	dockText := appModel.dock.StatusText
	if !strings.Contains(dockText, "Phase 2B.3 not started") {
		t.Errorf("expected dock status to declare Phase 2B.3 not started, got: %s", dockText)
	}

	// Verify rendered view displays confirmation state without execution
	view := appModel.View()
	if !strings.Contains(view, "Confirmed by User") {
		t.Errorf("view missing confirmation banner: %s", view)
	}
	if !strings.Contains(view, "Phase 2B.3 Boundary Invariant") {
		t.Errorf("view missing boundary invariant statement: %s", view)
	}
}

// TestAppModel_UpdateReview_Cancellation verifies user cancellation and cleanup.
func TestAppModel_UpdateReview_Cancellation(t *testing.T) {
	appModel, plan := setupTestAppWithPlanner(t)

	// Open and load
	m, _ := appModel.Update(pages.RequestUpdateReviewMsg{})
	m, _ = m.Update(update.PlanLoadedMsg{Plan: plan})
	appModel = m.(AppModel)

	// Press Esc to cancel
	m, cancelCmd := appModel.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cancelCmd == nil {
		t.Fatalf("expected cancelCmd on Esc")
	}

	msg := cancelCmd()
	cancMsg, ok := msg.(update.UpdateCancelledMsg)
	if !ok {
		t.Fatalf("expected UpdateCancelledMsg, got %T", msg)
	}

	m, _ = m.Update(cancMsg)
	appModel = m.(AppModel)

	if !appModel.LastDecision().IsCancelled() {
		t.Errorf("expected LastDecision().IsCancelled() == true")
	}

	// Dismiss review with Enter
	m, closeCmd := appModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if closeCmd == nil {
		t.Fatalf("expected closeCmd on dismiss")
	}

	closeMsg := closeCmd()
	m, _ = m.Update(closeMsg)
	appModel = m.(AppModel)

	if appModel.ReviewModalOpen() {
		t.Errorf("expected review modal closed after dismiss")
	}
}
