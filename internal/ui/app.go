package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/setupforge/setupforge/internal/app"
	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/ui/components"
	"github.com/setupforge/setupforge/internal/ui/layout"
	"github.com/setupforge/setupforge/internal/ui/pages"
	"github.com/setupforge/setupforge/internal/ui/theme"
	"github.com/setupforge/setupforge/internal/ui/update"
)

// FocusArea designates which region receives user keystrokes.
type FocusArea int

const (
	FocusStage FocusArea = iota
	FocusRail
	FocusHelpOverlay
)

// SystemInfoLoadedMsg carries detected host properties back into the Bubble Tea runtime.
type SystemInfoLoadedMsg struct {
	Info domain.SystemInfo
	Err  error
}

// UpdateStatusLoadedMsg carries detected package update availability back into the Bubble Tea runtime.
type UpdateStatusLoadedMsg struct {
	Status domain.UpdateStatus
	Err    error
}

// AppModel is the root Bubble Tea model managing SetupForge workspace lifecycle.
type AppModel struct {
	systemService *app.SystemService
	updateService *app.UpdateService
	currentTheme  theme.Theme
	styles        theme.Styles
	metrics       layout.Metrics

	// Navigation & Focus
	activeDestination int
	currentFocus      FocusArea
	railOverlayOpen   bool
	helpModalOpen     bool

	// Update Review Modal
	reviewModalOpen bool
	reviewModel     update.UpdateReviewModel
	lastDecision    domain.ConfirmationDecision

	// Components
	header components.HeaderModel
	rail   components.RailModel
	dock   components.DockModel

	// Page models
	overviewPage pages.OverviewModel
	placeholders []pages.PlaceholderModel
}

// NewAppModel initializes the root application model with the given SystemService and optional UpdateService.
func NewAppModel(sysService *app.SystemService, updateService *app.UpdateService) AppModel {
	th := theme.ForgeTheme
	st := theme.NewStyles(th)

	placeholders := []pages.PlaceholderModel{
		pages.NewPlaceholderModel("Software", "Software & Package Management", "Phase 3", "Browse DNF packages, Flatpaks, and manage repositories."),
		pages.NewPlaceholderModel("Workstation", "Desktop & Shell Customization", "Phase 4", "Configure GNOME desktop, terminal fonts, and shell configuration."),
		pages.NewPlaceholderModel("Development", "Developer Toolchains", "Phase 4", "One-click toolchain bundles for Go, Rust, C/C++, Node.js, and Python."),
		pages.NewPlaceholderModel("System", "System Health & Updates", "Phase 2", "Manage system updates, systemd services, and view hardware info."),
		pages.NewPlaceholderModel("Journal", "Operation History & Logs", "Phase 5", "Filterable audit log of system modifications and error diagnostics."),
		pages.NewPlaceholderModel("Settings", "SetupForge Configuration", "Phase 5", "Appearance themes, safety confirmation thresholds, and preferences."),
	}

	reviewModel := update.NewUpdateReviewModel(th, st)

	return AppModel{
		systemService: sysService,
		updateService: updateService,
		currentTheme:  th,
		styles:        st,
		metrics:       layout.Calculate(120, 35, false),

		activeDestination: 0,
		currentFocus:      FocusStage,
		railOverlayOpen:   false,
		helpModalOpen:     false,

		reviewModalOpen: false,
		reviewModel:     reviewModel,

		header: components.NewHeaderModel(),
		rail:   components.NewRailModel(),
		dock:   components.NewDockModel(),

		overviewPage: pages.NewOverviewModel(),
		placeholders: placeholders,
	}
}

// Init begins initial background discovery of host system properties and update availability.
func (m AppModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.systemService != nil {
		cmds = append(cmds, m.detectSystemCmd())
	}
	if m.updateService != nil {
		cmds = append(cmds, m.checkUpdatesCmd(false))
	}
	return tea.Batch(cmds...)
}

// detectSystemCmd dispatches an async call to SystemService.GetSystemInfo.
func (m AppModel) detectSystemCmd() tea.Cmd {
	return func() tea.Msg {
		info, err := m.systemService.GetSystemInfo(context.Background())
		return SystemInfoLoadedMsg{
			Info: info,
			Err:  err,
		}
	}
}

// checkUpdatesCmd dispatches an async call to UpdateService to check package updates.
func (m AppModel) checkUpdatesCmd(refreshMetadata bool) tea.Cmd {
	if m.updateService == nil {
		return nil
	}
	return func() tea.Msg {
		var status domain.UpdateStatus
		var err error
		if refreshMetadata {
			status, err = m.updateService.RefreshUpdateStatus(context.Background())
		} else {
			status, err = m.updateService.GetUpdateStatus(context.Background())
		}
		return UpdateStatusLoadedMsg{
			Status: status,
			Err:    err,
		}
	}
}

// Update processes Bubble Tea messages and executes state transitions.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	// 1. Terminal window resize
	case tea.WindowSizeMsg:
		m.metrics = layout.Calculate(msg.Width, msg.Height, m.railOverlayOpen)
		m.reviewModel.SetDimensions(m.reviewDialogWidth(), m.reviewDialogHeight())
		return m, nil

	// 2. System info loaded from application service
	case SystemInfoLoadedMsg:
		if msg.Err == nil {
			m.overviewPage.SetSystemInfo(msg.Info)
			m.header.DistroBadge = msg.Info.OS.PrettyName
			if msg.Info.OS.ID != "" && m.dock.StatusText == "" {
				m.dock.StatusText = fmt.Sprintf("%s Ready", msg.Info.OS.Name)
			}
		} else {
			m.dock.StatusText = "Host check warning"
		}
		return m, nil

	// 3. Update status loaded from application service
	case UpdateStatusLoadedMsg:
		m.overviewPage.SetUpdateStatus(msg.Status, msg.Err)
		if msg.Err == nil {
			if msg.Status.IsAvailable() {
				m.header.UpdateCount = msg.Status.UpdateCount
				m.dock.StatusText = fmt.Sprintf("%d updates available", msg.Status.UpdateCount)
			} else if msg.Status.IsUpToDate() {
				m.header.UpdateCount = 0
				m.dock.StatusText = "System is up to date"
			}
		} else {
			m.header.UpdateCount = 0
			m.dock.StatusText = "Update check warning"
		}
		return m, nil

	// 4. Update Review requested
	case pages.RequestUpdateReviewMsg, update.RequestUpdateReviewMsg:
		if m.updateService != nil && m.updateService.HasPlanner() {
			m.reviewModalOpen = true
			m.reviewModel.SetLoading()
			m.reviewModel.SetDimensions(m.reviewDialogWidth(), m.reviewDialogHeight())
			m.dock.StatusText = "Planning update transaction…"
			opID := fmt.Sprintf("op-update-%d", time.Now().Unix())
			return m, m.planUpdateCmd(opID)
		}
		m.dock.StatusText = "Update planner not available"
		return m, nil

	// 5. Update Plan loaded asynchronously
	case update.PlanLoadedMsg:
		var cmd tea.Cmd
		m.reviewModel, cmd = m.reviewModel.Update(msg)
		if msg.Err != nil {
			m.dock.StatusText = "Update planning failed"
		} else if msg.Plan.TotalPackageCount() == 0 {
			m.dock.StatusText = "System is up to date"
		} else {
			m.dock.StatusText = fmt.Sprintf("Reviewing %d updates", msg.Plan.TotalPackageCount())
		}
		return m, cmd

	// 6. Update confirmed by user (explicit confirmation decision recorded)
	case update.UpdateConfirmedMsg:
		m.lastDecision = msg.Decision
		var cmd tea.Cmd
		m.reviewModel, cmd = m.reviewModel.Update(msg)
		m.dock.StatusText = fmt.Sprintf("Plan confirmed (%d pkgs) · Phase 2B.3 not started", m.reviewModel.Plan().TotalPackageCount())
		if m.updateService != nil {
			_ = m.updateService.ConfirmActiveTransaction(context.Background(), msg.Decision)
		}
		// MANDATORY INVARIANT: Confirmation DOES NOT execute transaction replay. Privileged execution remains Phase 2B.4.
		return m, cmd

	// 7. Update cancelled by user
	case update.UpdateCancelledMsg:
		m.lastDecision = msg.Decision
		var cmd tea.Cmd
		m.reviewModel, cmd = m.reviewModel.Update(msg)
		m.dock.StatusText = "Update transaction cancelled"
		if m.updateService != nil {
			_ = m.updateService.CancelActiveTransaction(context.Background(), msg.Decision.OperationID)
		}
		return m, cmd

	// 8. Review modal closed
	case update.UpdateReviewClosedMsg:
		m.reviewModalOpen = false
		m.dock.StatusText = "Ready"
		return m, nil

	// 9. Jump destination request from launchpad
	case pages.JumpDestinationMsg:
		if msg.DestinationIndex >= 0 && msg.DestinationIndex < len(components.DefaultDestinations) {
			m.setDestination(msg.DestinationIndex)
		}
		return m, nil

	// 10. Keyboard interaction
	case tea.KeyMsg:
		// Global quit
		if msg.String() == "ctrl+q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Modal focus trap: if review modal is open, direct all keystrokes to reviewModel
		if m.reviewModalOpen {
			var cmd tea.Cmd
			m.reviewModel, cmd = m.reviewModel.Update(msg)
			return m, cmd
		}

		// Help modal handling
		if m.helpModalOpen {
			if msg.String() == "esc" || msg.String() == "?" || msg.String() == "enter" {
				m.helpModalOpen = false
				m.currentFocus = FocusStage
			}
			return m, nil
		}

		// Global toggle help
		if msg.String() == "?" {
			m.helpModalOpen = true
			m.currentFocus = FocusHelpOverlay
			return m, nil
		}

		// Global shortcut 'u': review updates when available
		if msg.String() == "u" && (m.header.UpdateCount > 0 || m.activeDestination == 4) {
			return m, func() tea.Msg { return pages.RequestUpdateReviewMsg{} }
		}

		// Toggle rail overlay (S class)
		if msg.String() == "ctrl+b" {
			m.railOverlayOpen = !m.railOverlayOpen
			m.metrics = layout.Calculate(m.metrics.TotalWidth, m.metrics.TotalHeight, m.railOverlayOpen)
			return m, nil
		}

		// Refresh system info and update availability
		if msg.String() == "r" {
			m.dock.StatusText = "Refreshing…"
			m.overviewPage.SetCheckingUpdates()
			var refreshCmds []tea.Cmd
			if m.systemService != nil {
				refreshCmds = append(refreshCmds, func() tea.Msg {
					info, err := m.systemService.RefreshSystemInfo(context.Background())
					return SystemInfoLoadedMsg{Info: info, Err: err}
				})
			}
			if m.updateService != nil {
				refreshCmds = append(refreshCmds, m.checkUpdatesCmd(true))
			}
			return m, tea.Batch(refreshCmds...)
		}

		// Rail numeric jump shortcuts: '1' through '7'
		if len(msg.String()) == 1 && msg.String()[0] >= '1' && msg.String()[0] <= '7' {
			destIdx := int(msg.String()[0] - '1')
			m.setDestination(destIdx)
			return m, nil
		}

		// Cycle focus: Tab / Shift+Tab
		if msg.String() == "tab" {
			if m.currentFocus == FocusStage && !m.metrics.IsSmall {
				m.currentFocus = FocusRail
			} else {
				m.currentFocus = FocusStage
			}
			m.updateComponentFocus()
			return m, nil
		}
		if msg.String() == "shift+tab" {
			if m.currentFocus == FocusRail {
				m.currentFocus = FocusStage
			} else if !m.metrics.IsSmall {
				m.currentFocus = FocusRail
			}
			m.updateComponentFocus()
			return m, nil
		}

		// Route input to focused area
		if m.currentFocus == FocusRail {
			switch msg.String() {
			case "up", "k":
				if m.activeDestination > 0 {
					m.setDestination(m.activeDestination - 1)
				}
			case "down", "j":
				if m.activeDestination < len(components.DefaultDestinations)-1 {
					m.setDestination(m.activeDestination + 1)
				}
			case "enter", "l", "right":
				m.currentFocus = FocusStage
				m.updateComponentFocus()
			}
			return m, nil
		}

		// Current focus is FocusStage: route to active page
		if m.activeDestination == 0 {
			var pageCmd tea.Cmd
			m.overviewPage, pageCmd = m.overviewPage.Update(msg)
			if pageCmd != nil {
				cmds = append(cmds, pageCmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *AppModel) setDestination(index int) {
	m.activeDestination = index
	m.rail.ActiveIndex = index
	m.header.Breadcrumb = components.DefaultDestinations[index].Title
	m.updateComponentFocus()
}

func (m *AppModel) updateComponentFocus() {
	m.rail.IsFocused = (m.currentFocus == FocusRail)
	stageFocused := (m.currentFocus == FocusStage)

	m.overviewPage.IsFocused = stageFocused
	for i := range m.placeholders {
		m.placeholders[i].IsFocused = stageFocused
	}
}

// View produces the composite terminal rendering of the entire SetupForge workspace.
func (m AppModel) View() string {
	if m.metrics.TotalWidth <= 0 || m.metrics.TotalHeight <= 0 {
		return ""
	}

	// 1. Render Row 0: Header
	headerView := m.header.Render(m.metrics, m.styles)

	// 2. Render Stage Content
	var stageContent string
	if m.activeDestination == 0 {
		stageContent = m.overviewPage.Render(m.metrics, m.styles)
	} else if m.activeDestination-1 < len(m.placeholders) {
		stageContent = m.placeholders[m.activeDestination-1].Render(m.metrics, m.styles)
	}

	// 3. Render Middle Rows: Rail + Stage
	var middleView string
	if m.metrics.RailWidth > 0 {
		railView := m.rail.Render(m.metrics, m.styles)
		middleView = lipgloss.JoinHorizontal(lipgloss.Top, railView, stageContent)
	} else {
		// S breakpoint: rail is collapsed
		middleView = stageContent
	}

	// 4. Render Last Row: Dock
	dockView := m.dock.Render(m.metrics, m.styles)

	// Combine rows vertically
	workspace := lipgloss.JoinVertical(lipgloss.Left, headerView, middleView, dockView)

	// If Update Review modal is active, overlay it with centered placement
	if m.reviewModalOpen {
		return m.renderReviewOverlay(workspace)
	}

	// If Help modal is active, overlay it
	if m.helpModalOpen {
		return m.renderHelpOverlay(workspace)
	}

	return workspace
}

// ReviewModalOpen reports whether the update review modal overlay is open.
func (m AppModel) ReviewModalOpen() bool {
	return m.reviewModalOpen
}

// ReviewModel returns the current UpdateReviewModel.
func (m AppModel) ReviewModel() update.UpdateReviewModel {
	return m.reviewModel
}

// LastDecision returns the last recorded confirmation decision.
func (m AppModel) LastDecision() domain.ConfirmationDecision {
	return m.lastDecision
}

func (m AppModel) reviewDialogWidth() int {
	if m.metrics.IsSmall {
		return max(76, min(m.metrics.TotalWidth-2, 80))
	} else if m.metrics.IsMedium {
		return min(96, m.metrics.TotalWidth-4)
	}
	return min(120, m.metrics.TotalWidth-6)
}

func (m AppModel) reviewDialogHeight() int {
	if m.metrics.IsSmall {
		return max(20, min(m.metrics.TotalHeight-2, 22))
	} else if m.metrics.IsMedium {
		return min(28, m.metrics.TotalHeight-2)
	}
	return min(32, m.metrics.TotalHeight-2)
}

func (m AppModel) renderReviewOverlay(background string) string {
	dialog := m.reviewModel.Render()
	return lipgloss.Place(
		m.metrics.TotalWidth,
		m.metrics.TotalHeight,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars(" "),
	)
}

func (m AppModel) planUpdateCmd(opID string) tea.Cmd {
	if m.updateService == nil {
		return func() tea.Msg {
			return update.PlanLoadedMsg{Err: fmt.Errorf("no update service available")}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		artifact, plan, err := m.updateService.PlanUpdate(ctx, opID)
		if err != nil {
			return update.PlanLoadedMsg{Err: err}
		}

		if err := m.updateService.ValidateArtifact(ctx, artifact); err != nil {
			return update.PlanLoadedMsg{Err: err}
		}

		return update.PlanLoadedMsg{Plan: plan}
	}
}

// renderHelpOverlay renders a centered modal dialog with keybinding documentation.
func (m AppModel) renderHelpOverlay(background string) string {
	dialogW := 60
	dialogH := 15
	if m.metrics.IsSmall {
		dialogW = min(m.metrics.TotalWidth-4, 50)
		dialogH = 13
	}

	var helpLines []string
	helpLines = append(helpLines, m.styles.Heading.Render("SetupForge Core Navigation"))
	helpLines = append(helpLines, "")
	helpLines = append(helpLines, fmt.Sprintf("%-12s %s", m.styles.Key.Render("1–7"), m.styles.Body.Render("Jump directly to rail destination")))
	helpLines = append(helpLines, fmt.Sprintf("%-12s %s", m.styles.Key.Render("Tab"), m.styles.Body.Render("Cycle focus between Rail and Stage")))
	helpLines = append(helpLines, fmt.Sprintf("%-12s %s", m.styles.Key.Render("j/k or ↑/↓"), m.styles.Body.Render("Move selection / navigate items")))
	helpLines = append(helpLines, fmt.Sprintf("%-12s %s", m.styles.Key.Render("Enter"), m.styles.Body.Render("Activate selection / jump to section")))
	helpLines = append(helpLines, fmt.Sprintf("%-12s %s", m.styles.Key.Render("Ctrl+B"), m.styles.Body.Render("Toggle rail sidebar (compact mode)")))
	helpLines = append(helpLines, fmt.Sprintf("%-12s %s", m.styles.Key.Render("r"), m.styles.Body.Render("Refresh host system status")))
	helpLines = append(helpLines, fmt.Sprintf("%-12s %s", m.styles.Key.Render("?"), m.styles.Body.Render("Toggle this help modal")))
	helpLines = append(helpLines, fmt.Sprintf("%-12s %s", m.styles.Key.Render("Ctrl+Q"), m.styles.Body.Render("Quit SetupForge")))
	helpLines = append(helpLines, "")
	helpLines = append(helpLines, m.styles.Hint.Render("Press Esc or Enter to close"))

	content := strings.Join(helpLines, "\n")
	dialog := components.RenderPane(components.PaneConfig{
		Title:     "Keyboard Help",
		Width:     dialogW,
		Height:    dialogH,
		IsFocused: true,
	}, content, m.styles)

	// Center dialog on the screen
	return lipgloss.Place(
		m.metrics.TotalWidth,
		m.metrics.TotalHeight,
		lipgloss.Center,
		lipgloss.Center,
		dialog,
		lipgloss.WithWhitespaceChars(" "),
	)
}
