package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/setupforge/setupforge/internal/app"
	"github.com/setupforge/setupforge/internal/infra/detector"
	"github.com/setupforge/setupforge/internal/infra/journal"
	"github.com/setupforge/setupforge/internal/infra/privilege"
	"github.com/setupforge/setupforge/internal/infra/storage"
	"github.com/setupforge/setupforge/internal/infra/update"
	"github.com/setupforge/setupforge/internal/ui"
)

var (
	version = "0.1.0-dev"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("SetupForge v%s (Fedora-first TUI)\n", version)
			return
		case "--help", "-h":
			fmt.Println("SetupForge — Fedora Post-Installation Control Center")
			fmt.Println("Usage: setupforge [options]")
			fmt.Println("\nOptions:")
			fmt.Println("  -v, --version    Print version and exit")
			fmt.Println("  -h, --help       Print help and exit")
			return
		}
	}

	// 1. Initialize storage, journal, and advisory engine lock
	localStore := storage.NewLocalFilesystemStore()
	fileJournal := journal.NewFileTransactionJournal()
	engineLock := journal.NewEngineLock()

	// Acquire advisory flock(2) ensuring single-engine ownership
	if acquired, err := engineLock.TryLock(); err != nil || !acquired {
		fmt.Fprintf(os.Stderr, "Notice: Another SetupForge instance holds the workspace lock.\n")
	} else {
		defer func() {
			_ = engineLock.Unlock()
		}()
	}

	// 2. Initialize domain adapters and application services
	osDetector := detector.NewFedoraDetector()
	sysService := app.NewSystemService(osDetector)

	updateChecker := update.NewDNF5UpdateChecker()
	updatePlanner := update.NewDNF5UpdatePlanner(update.WithPlannerArtifactStore(localStore))
	sudoAuth := privilege.NewSudoAuthorizationProvider()
	sudoRunner := privilege.NewSudoRunner()
	updateService := app.NewUpdateService(
		updateChecker,
		app.WithUpdatePlanner(updatePlanner),
		app.WithArtifactStore(localStore),
		app.WithTransactionJournal(fileJournal),
		app.WithEngineLock(engineLock),
		app.WithAuthorizationProvider(sudoAuth),
		app.WithPrivilegedRunner(sudoRunner),
	)

	// 3. Startup crash recovery and garbage collection reconciliation
	_, _ = updateService.RecoverOrReconcile(context.Background())

	// 4. Initialize Root Bubble Tea presentation model
	appModel := ui.NewAppModel(sysService, updateService)

	// 5. Run the TUI program with alternate screen buffer and mouse support
	p := tea.NewProgram(appModel, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running SetupForge: %v\n", err)
		os.Exit(1)
	}
}
