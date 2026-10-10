package ui_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/setupforge/setupforge/internal/app"
	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/ui"
)

type mockDetector struct {
	info domain.SystemInfo
	err  error
}

func (m *mockDetector) Detect(ctx context.Context) (domain.SystemInfo, error) {
	if m.err != nil {
		return domain.SystemInfo{}, m.err
	}
	return m.info, nil
}

type mockUpdateChecker struct {
	status domain.UpdateStatus
	err    error
}

func (m *mockUpdateChecker) Check(ctx context.Context, opts domain.CheckOptions) (domain.UpdateStatus, error) {
	if m.err != nil {
		return m.status, m.err
	}
	return m.status, nil
}

func executeCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, c := range batch {
			msgs = append(msgs, executeCmd(c)...)
		}
		return msgs
	}
	return []tea.Msg{msg}
}

func setupTestApp() ui.AppModel {
	detector := &mockDetector{
		info: domain.SystemInfo{
			OS: domain.OSRelease{
				ID:         "fedora",
				Name:       "Fedora Linux",
				Version:    "44",
				PrettyName: "Fedora Linux 44 (Workstation Edition)",
			},
			Kernel:       "6.14.0-test",
			Architecture: "amd64",
			Hostname:     "test-box",
			IsSupported:  true,
		},
	}
	sysSvc := app.NewSystemService(detector)

	updateChecker := &mockUpdateChecker{
		status: domain.UpdateStatus{
			State:       domain.UpdateStateAvailable,
			UpdateCount: 3,
			Updates: []domain.PackageUpdate{
				{Name: "pkgA", Arch: "x86_64", EVR: "1.0-1.fc44", Repository: "updates"},
			},
		},
	}
	updateSvc := app.NewUpdateService(updateChecker)

	return ui.NewAppModel(sysSvc, updateSvc)
}

type mockUpdatePlanner struct {
	artifact domain.TransactionArtifact
	plan     domain.UpdatePlan
}

func (m *mockUpdatePlanner) Plan(ctx context.Context, opID string) (domain.TransactionArtifact, domain.UpdatePlan, error) {
	art := m.artifact
	art.OperationID = opID
	pln := m.plan
	pln.OperationID = opID
	pln.Artifact = art
	return art, pln, nil
}

func (m *mockUpdatePlanner) ValidateArtifact(artifact domain.TransactionArtifact) error {
	return nil
}

func setupTestAppWithPlanner(plan domain.UpdatePlan) ui.AppModel {
	detector := &mockDetector{
		info: domain.SystemInfo{
			OS: domain.OSRelease{
				ID:         "fedora",
				Name:       "Fedora Linux",
				Version:    "44",
				PrettyName: "Fedora Linux 44 (Workstation Edition)",
			},
			Kernel:       "6.14.0-test",
			Architecture: "amd64",
			Hostname:     "test-box",
			IsSupported:  true,
		},
	}
	sysSvc := app.NewSystemService(detector)

	updateChecker := &mockUpdateChecker{
		status: domain.UpdateStatus{
			State:       domain.UpdateStateAvailable,
			UpdateCount: plan.TotalPackageCount(),
		},
	}

	planner := &mockUpdatePlanner{
		artifact: plan.Artifact,
		plan:     plan,
	}

	updateSvc := app.NewUpdateService(updateChecker, app.WithUpdatePlanner(planner))
	return ui.NewAppModel(sysSvc, updateSvc)
}

func TestAppModel_InitAndSystemLoad(t *testing.T) {
	appModel := setupTestApp()

	cmd := appModel.Init()
	if cmd == nil {
		t.Fatal("expected Init to return a tea.Cmd for system detection and update check")
	}

	msgs := executeCmd(cmd)
	if len(msgs) == 0 {
		t.Fatal("expected messages from Init command batch")
	}

	var m tea.Model = appModel
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}

	app := m.(ui.AppModel)
	view := app.View()
	if !strings.Contains(view, "Fedora Linux 44") {
		t.Errorf("expected view to contain detected OS, got %q", view)
	}
	if !strings.Contains(view, "3 available") {
		t.Errorf("expected view to contain available updates count, got %q", view)
	}
	if !strings.Contains(view, "3 updates available") {
		t.Errorf("expected dock status to contain '3 updates available', got %q", view)
	}
}

func TestAppModel_UpdateStatus_UpToDate(t *testing.T) {
	appModel := setupTestApp()
	var m tea.Model = appModel
	for _, msg := range executeCmd(appModel.Init()) {
		m, _ = m.Update(msg)
	}

	upToDateMsg := ui.UpdateStatusLoadedMsg{
		Status: domain.UpdateStatus{
			State:       domain.UpdateStateUpToDate,
			UpdateCount: 0,
		},
	}

	updated, _ := m.Update(upToDateMsg)
	app := updated.(ui.AppModel)
	view := app.View()

	if !strings.Contains(view, "Up to date") {
		t.Errorf("expected view to reflect up to date, got %q", view)
	}
	if !strings.Contains(view, "System is up to date") {
		t.Errorf("expected dock status to reflect up to date, got %q", view)
	}
}

func TestAppModel_UpdateStatus_Error(t *testing.T) {
	appModel := setupTestApp()
	var m tea.Model = appModel
	for _, msg := range executeCmd(appModel.Init()) {
		m, _ = m.Update(msg)
	}

	failedMsg := ui.UpdateStatusLoadedMsg{
		Status: domain.UpdateStatus{
			State:    domain.UpdateStateFailed,
			ErrorMsg: "network down",
		},
		Err: errors.New("network down"),
	}

	updated, _ := m.Update(failedMsg)
	app := updated.(ui.AppModel)
	view := app.View()

	if !strings.Contains(view, "Check warning") {
		t.Errorf("expected overview to reflect update check warning, got %q", view)
	}
	if !strings.Contains(view, "Update check warning") {
		t.Errorf("expected dock to reflect update check warning, got %q", view)
	}
}

func TestAppModel_RefreshKey(t *testing.T) {
	appModel := setupTestApp()

	// Press 'r' to trigger refresh
	updated, refreshCmd := appModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	app := updated.(ui.AppModel)

	if refreshCmd == nil {
		t.Fatal("expected 'r' to return a tea.Cmd")
	}

	// Verify status indicates refreshing immediately
	view := app.View()
	if !strings.Contains(view, "Refreshing…") {
		t.Errorf("expected dock to show Refreshing…, got %q", view)
	}

	// Execute refresh commands
	msgs := executeCmd(refreshCmd)
	var m tea.Model = app
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}

	finalApp := m.(ui.AppModel)
	finalView := finalApp.View()
	if !strings.Contains(finalView, "3 available") {
		t.Errorf("expected view after refresh to contain updates, got %q", finalView)
	}
}

func TestAppModel_Navigation(t *testing.T) {
	appModel := setupTestApp()

	// 1. Jump to Software (Key "2")
	updated, _ := appModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	app := updated.(ui.AppModel)
	view := app.View()

	if !strings.Contains(view, "Software") {
		t.Errorf("expected view to switch to Software, got %q", view)
	}

	// 2. Jump to Settings (Key "7")
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'7'}})
	app = updated.(ui.AppModel)
	view = app.View()

	if !strings.Contains(view, "Settings") {
		t.Errorf("expected view to switch to Settings, got %q", view)
	}

	// 3. Jump back to Overview (Key "1")
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	app = updated.(ui.AppModel)
	view = app.View()

	if !strings.Contains(view, "Overview") {
		t.Errorf("expected view to return to Overview, got %q", view)
	}
}

func TestAppModel_HelpOverlay(t *testing.T) {
	appModel := setupTestApp()

	// Press '?' to open help modal
	updated, _ := appModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	app := updated.(ui.AppModel)

	view := app.View()
	if !strings.Contains(view, "Keyboard Help") {
		t.Errorf("expected view to show help modal, got %q", view)
	}

	// Press 'esc' to dismiss help modal
	updated, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	app = updated.(ui.AppModel)

	view = app.View()
	if strings.Contains(view, "Keyboard Help") {
		t.Errorf("expected help modal to be dismissed, got %q", view)
	}
}

func TestAppModel_ResponsiveBreakpoints(t *testing.T) {
	appModel := setupTestApp()

	sizes := []struct {
		width  int
		height int
	}{
		{80, 24},  // S
		{100, 30}, // M
		{120, 35}, // L
		{160, 45}, // XL
	}

	for _, sz := range sizes {
		updated, _ := appModel.Update(tea.WindowSizeMsg{Width: sz.width, Height: sz.height})
		app := updated.(ui.AppModel)

		view := app.View()
		if len(view) == 0 {
			t.Errorf("expected non-empty view for %dx%d", sz.width, sz.height)
		}
		if !strings.Contains(view, "SetupForge") {
			t.Errorf("expected brand in view for %dx%d", sz.width, sz.height)
		}
	}
}
