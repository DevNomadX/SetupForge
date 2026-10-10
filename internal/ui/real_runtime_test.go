//go:build manual_diagnostic

package ui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/setupforge/setupforge/internal/app"
	"github.com/setupforge/setupforge/internal/infra/detector"
	"github.com/setupforge/setupforge/internal/infra/update"
	"github.com/setupforge/setupforge/internal/ui"
)

// TestRealRuntime_SetupForge verifies the exact runtime wiring of cmd/setupforge/main.go on the real host.
func TestRealRuntime_SetupForge(t *testing.T) {
	fmt.Println("=== RUNNING REAL SETUPFORGE RUNTIME WIRING ===")

	osDetector := detector.NewFedoraDetector()
	sysService := app.NewSystemService(osDetector)

	updateChecker := update.NewDNF5UpdateChecker()
	updateService := app.NewUpdateService(updateChecker)

	appModel := ui.NewAppModel(sysService, updateService)

	// Execute Init cmd batch on real system
	initCmd := appModel.Init()
	if initCmd == nil {
		t.Fatal("expected init cmd")
	}

	msgs := executeCmd(initCmd)
	var m tea.Model = appModel
	for _, msg := range msgs {
		fmt.Printf("Received message: %T\n", msg)
		if uMsg, ok := msg.(ui.UpdateStatusLoadedMsg); ok {
			fmt.Printf("UpdateStatusLoadedMsg: State=%s, Updates=%d, Err=%v\n", uMsg.Status.State, uMsg.Status.UpdateCount, uMsg.Err)
		}
		m, _ = m.Update(msg)
	}

	app := m.(ui.AppModel)
	view := app.View()

	fmt.Println("--- Rendered Real Runtime View Sample ---")
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		if i < 15 || i > len(lines)-4 {
			fmt.Println(line)
		} else if i == 15 {
			fmt.Println("... [stage content omitted] ...")
		}
	}

	// Verify update status is NOT warning
	if strings.Contains(view, "Check warning") {
		t.Errorf("FAIL: view contains 'Check warning'!")
	} else {
		fmt.Println("SUCCESS: View contains NO 'Check warning'!")
	}

	if strings.Contains(view, "available") || strings.Contains(view, "Up to date") {
		fmt.Println("SUCCESS: View contains valid update status!")
	} else {
		t.Errorf("FAIL: view missing expected update status")
	}
}
