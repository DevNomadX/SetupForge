package update

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/ui/components"
	"github.com/setupforge/setupforge/internal/ui/theme"
)

// ReviewState represents the lifecycle phase of the update review UI.
type ReviewState int

const (
	ReviewStateIdle ReviewState = iota
	ReviewStateLoading
	ReviewStateReady
	ReviewStateConfirmed
	ReviewStateCancelled
	ReviewStateError
	ReviewStateEmpty
)

func (s ReviewState) String() string {
	switch s {
	case ReviewStateIdle:
		return "Idle"
	case ReviewStateLoading:
		return "Loading"
	case ReviewStateReady:
		return "Ready"
	case ReviewStateConfirmed:
		return "Confirmed"
	case ReviewStateCancelled:
		return "Cancelled"
	case ReviewStateError:
		return "Error"
	case ReviewStateEmpty:
		return "Empty"
	default:
		return "Unknown"
	}
}

// ReviewControl represents the interactive control currently focused within ReviewStateReady.
type ReviewControl int

const (
	ControlTable ReviewControl = iota
	ControlCancelButton
	ControlConfirmButton
)

// RequestUpdateReviewMsg is dispatched when a user requests review of pending system updates.
type RequestUpdateReviewMsg struct{}

// PlanLoadedMsg arrives when an UpdatePlan has been generated and validated asynchronously.
type PlanLoadedMsg struct {
	Plan domain.UpdatePlan
	Err  error
}

// UpdateConfirmedMsg is emitted when the user explicitly activates the Confirm Update button.
// Invariant: This message does NOT execute any transaction. Execution is Phase 2B.3+.
type UpdateConfirmedMsg struct {
	Decision domain.ConfirmationDecision
}

// UpdateCancelledMsg is emitted when the user explicitly cancels or presses Esc.
type UpdateCancelledMsg struct {
	Decision domain.ConfirmationDecision
}

// UpdateReviewClosedMsg is emitted when the review interface is dismissed.
type UpdateReviewClosedMsg struct{}

// ReviewItem is a presentation-ready row representing a logical package operation.
type ReviewItem struct {
	Action        domain.PackageAction
	Name          string
	OldEVR        string
	NewEVR        string
	Arch          string
	Repo          string
	SizeBytes     int64
	FormattedSize string
	ChangeSummary string
}

// FormatBytes formats byte sizes into readable decimal binary units (B, kB, MB, GB).
func FormatBytes(b int64) string {
	if b <= 0 {
		return "0 B"
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

// BuildReviewItems transforms a slice of domain PlannedPackages into clean presentation items.
// Passive duplicate 'Replaced' rows are excluded so each logical change has exactly one row.
func BuildReviewItems(packages []domain.PlannedPackage) []ReviewItem {
	var items []ReviewItem
	for _, pkg := range packages {
		if pkg.Action == domain.ActionReplaced {
			continue
		}

		var changeSummary string
		switch pkg.Action {
		case domain.ActionUpgrade:
			if pkg.OldEVR != "" && pkg.EVR != "" {
				changeSummary = fmt.Sprintf("%s → %s", pkg.OldEVR, pkg.EVR)
			} else if pkg.EVR != "" {
				changeSummary = fmt.Sprintf("→ %s", pkg.EVR)
			} else {
				changeSummary = "Upgrade"
			}
		case domain.ActionDowngrade:
			if pkg.OldEVR != "" && pkg.EVR != "" {
				changeSummary = fmt.Sprintf("%s → %s", pkg.OldEVR, pkg.EVR)
			} else {
				changeSummary = "Downgrade"
			}
		case domain.ActionInstall:
			changeSummary = fmt.Sprintf("Install %s", pkg.EVR)
		case domain.ActionRemove:
			changeSummary = fmt.Sprintf("Remove %s", pkg.OldEVR)
		default:
			changeSummary = string(pkg.Action)
		}

		items = append(items, ReviewItem{
			Action:        pkg.Action,
			Name:          pkg.Name,
			OldEVR:        pkg.OldEVR,
			NewEVR:        pkg.EVR,
			Arch:          pkg.Arch,
			Repo:          pkg.RepoID,
			SizeBytes:     pkg.SizeBytes,
			FormattedSize: FormatBytes(pkg.SizeBytes),
			ChangeSummary: changeSummary,
		})
	}
	return items
}

// UpdateReviewModel coordinates the presentation, responsive layout, navigation,
// and explicit confirmation of an UpdatePlan.
type UpdateReviewModel struct {
	state          ReviewState
	focusedControl ReviewControl
	plan           domain.UpdatePlan
	decision       domain.ConfirmationDecision
	items          []ReviewItem
	table          table.Model
	err            error
	width          int
	height         int
	styles         theme.Styles
	theme          theme.Theme
}

// NewUpdateReviewModel initializes an UpdateReviewModel.
func NewUpdateReviewModel(th theme.Theme, st theme.Styles) UpdateReviewModel {
	tbl := table.New(
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = st.Meta.Bold(true).BorderBottom(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(th.BorderMuted)
	s.Selected = st.Selection.Bold(true)
	s.Cell = st.Body
	tbl.SetStyles(s)

	return UpdateReviewModel{
		state:          ReviewStateIdle,
		focusedControl: ControlTable,
		table:          tbl,
		theme:          th,
		styles:         st,
		width:          80,
		height:         24,
	}
}

// State returns the current lifecycle state.
func (m UpdateReviewModel) State() ReviewState {
	return m.state
}

// Decision returns the user's recorded confirmation decision.
func (m UpdateReviewModel) Decision() domain.ConfirmationDecision {
	return m.decision
}

// Plan returns the currently loaded UpdatePlan.
func (m UpdateReviewModel) Plan() domain.UpdatePlan {
	return m.plan
}

// FocusedControl returns the active interactive control.
func (m UpdateReviewModel) FocusedControl() ReviewControl {
	return m.focusedControl
}

// Items returns the presentation items shown in the review table.
func (m UpdateReviewModel) Items() []ReviewItem {
	return m.items
}

// SetPlan populates the model with a verified UpdatePlan and transitions to ReviewStateReady or ReviewStateEmpty.
func (m *UpdateReviewModel) SetPlan(plan domain.UpdatePlan) {
	m.plan = plan
	m.err = nil
	m.decision = domain.ConfirmationDecision{}
	m.items = BuildReviewItems(plan.Packages)

	if len(m.items) == 0 {
		m.state = ReviewStateEmpty
		return
	}

	m.state = ReviewStateReady
	m.focusedControl = ControlTable
	m.table.Focus()
	m.rebuildTable()
}

// SetError transitions the model to ReviewStateError.
func (m *UpdateReviewModel) SetError(err error) {
	m.err = err
	m.state = ReviewStateError
}

// SetLoading transitions the model to ReviewStateLoading.
func (m *UpdateReviewModel) SetLoading() {
	m.state = ReviewStateLoading
	m.err = nil
}

// SetDimensions updates the layout geometry and adjusts table columns to match responsive width.
func (m *UpdateReviewModel) SetDimensions(w, h int) {
	m.width = w
	m.height = h
	if m.state == ReviewStateReady {
		m.rebuildTable()
	}
}

func (m *UpdateReviewModel) rebuildTable() {
	interiorW := m.width - 2 - (theme.SpaceSM * 2)
	if interiorW < 10 {
		interiorW = 10
	}

	// Calculate table height: interiorHeight minus top banner and bottom controls
	interiorH := m.height - 4
	topBannerRows := 4
	if m.plan.HasKernelUpdate || m.plan.HasSystemdUpdate {
		topBannerRows++
	}
	bottomControlRows := 3
	tableH := interiorH - topBannerRows - bottomControlRows
	if tableH < 3 {
		tableH = 3
	}

	m.table.SetWidth(interiorW)
	m.table.SetHeight(tableH)

	var cols []table.Column
	var rows []table.Row

	if interiorW < 75 {
		// Small breakpoint (S)
		actionW := 10
		versionW := 18
		nameW := max(16, interiorW-actionW-versionW-4)
		cols = []table.Column{
			{Title: "Action", Width: actionW},
			{Title: "Package", Width: nameW},
			{Title: "Version", Width: versionW},
		}
		for _, it := range m.items {
			rows = append(rows, table.Row{
				string(it.Action),
				it.Name,
				it.NewEVR,
			})
		}
	} else if interiorW < 100 {
		// Medium breakpoint (M)
		actionW := 10
		nameW := 22
		sizeW := 10
		changeW := max(22, interiorW-actionW-nameW-sizeW-6)
		cols = []table.Column{
			{Title: "Action", Width: actionW},
			{Title: "Package", Width: nameW},
			{Title: "Change", Width: changeW},
			{Title: "Size", Width: sizeW},
		}
		for _, it := range m.items {
			rows = append(rows, table.Row{
				string(it.Action),
				it.Name,
				it.ChangeSummary,
				it.FormattedSize,
			})
		}
	} else {
		// Large / Extra-Large breakpoint (L/XL)
		actionW := 10
		nameW := 24
		repoW := 12
		sizeW := 10
		rem := interiorW - actionW - nameW - repoW - sizeW - 8
		evrW := max(18, rem/2)
		cols = []table.Column{
			{Title: "Action", Width: actionW},
			{Title: "Package", Width: nameW},
			{Title: "Old EVR", Width: evrW},
			{Title: "New EVR", Width: evrW},
			{Title: "Repo", Width: repoW},
			{Title: "Size", Width: sizeW},
		}
		for _, it := range m.items {
			rows = append(rows, table.Row{
				string(it.Action),
				it.Name,
				it.OldEVR,
				it.NewEVR,
				it.Repo,
				it.FormattedSize,
			})
		}
	}

	m.table.SetColumns(cols)
	m.table.SetRows(rows)
}

// Update processes Bubble Tea events and orchestrates review navigation.
func (m UpdateReviewModel) Update(msg tea.Msg) (UpdateReviewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case PlanLoadedMsg:
		if msg.Err != nil {
			m.SetError(msg.Err)
			return m, nil
		}
		m.SetPlan(msg.Plan)
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case ReviewStateReady:
			switch msg.String() {
			case "tab":
				switch m.focusedControl {
				case ControlTable:
					m.focusedControl = ControlCancelButton
					m.table.Blur()
				case ControlCancelButton:
					m.focusedControl = ControlConfirmButton
				case ControlConfirmButton:
					m.focusedControl = ControlTable
					m.table.Focus()
				}
				return m, nil

			case "shift+tab":
				switch m.focusedControl {
				case ControlTable:
					m.focusedControl = ControlConfirmButton
					m.table.Blur()
				case ControlCancelButton:
					m.focusedControl = ControlTable
					m.table.Focus()
				case ControlConfirmButton:
					m.focusedControl = ControlCancelButton
				}
				return m, nil

			case "left", "h":
				if m.focusedControl == ControlConfirmButton {
					m.focusedControl = ControlCancelButton
					return m, nil
				}
			case "right", "l":
				if m.focusedControl == ControlCancelButton {
					m.focusedControl = ControlConfirmButton
					return m, nil
				}

			case "enter":
				if m.focusedControl == ControlCancelButton {
					m.state = ReviewStateCancelled
					m.decision = domain.ConfirmationDecision{
						Status:              domain.ConfirmationStatusCancelled,
						OperationID:         m.plan.OperationID,
						ArtifactDirectory:   m.plan.Artifact.Directory,
						ArtifactFingerprint: m.plan.Artifact.Fingerprint,
						DecidedAt:           time.Now(),
					}
					dec := m.decision
					return m, func() tea.Msg {
						return UpdateCancelledMsg{Decision: dec}
					}
				} else if m.focusedControl == ControlConfirmButton {
					m.state = ReviewStateConfirmed
					m.decision = domain.ConfirmationDecision{
						Status:              domain.ConfirmationStatusConfirmed,
						OperationID:         m.plan.OperationID,
						ArtifactDirectory:   m.plan.Artifact.Directory,
						ArtifactFingerprint: m.plan.Artifact.Fingerprint,
						DecidedAt:           time.Now(),
					}
					dec := m.decision
					return m, func() tea.Msg {
						return UpdateConfirmedMsg{Decision: dec}
					}
				}
				// MANDATORY SAFETY INVARIANT: If focusedControl == ControlTable, Enter does NOT confirm.
				return m, nil

			case "esc":
				m.state = ReviewStateCancelled
				m.decision = domain.ConfirmationDecision{
					Status:              domain.ConfirmationStatusCancelled,
					OperationID:         m.plan.OperationID,
					ArtifactDirectory:   m.plan.Artifact.Directory,
					ArtifactFingerprint: m.plan.Artifact.Fingerprint,
					DecidedAt:           time.Now(),
				}
				dec := m.decision
				return m, func() tea.Msg {
					return UpdateCancelledMsg{Decision: dec}
				}

			default:
				if m.focusedControl == ControlTable {
					var cmd tea.Cmd
					m.table, cmd = m.table.Update(msg)
					return m, cmd
				}
			}

		case ReviewStateConfirmed, ReviewStateCancelled, ReviewStateError, ReviewStateEmpty:
			switch msg.String() {
			case "esc", "enter", "q":
				return m, func() tea.Msg {
					return UpdateReviewClosedMsg{}
				}
			}

		case ReviewStateLoading:
			if msg.String() == "esc" {
				return m, func() tea.Msg {
					return UpdateReviewClosedMsg{}
				}
			}
		}
	}

	return m, nil
}

// Render produces the visual representation of the UpdateReviewModel framed in a standardized Pane.
func (m UpdateReviewModel) Render() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}

	interiorW := m.width - 2 - (theme.SpaceSM * 2)
	if interiorW < 10 {
		interiorW = 10
	}

	var content string
	var countStr string
	var posStr string

	switch m.state {
	case ReviewStateIdle:
		countStr = "Idle"
		content = strings.Join([]string{
			m.styles.Heading.Render("Update Transaction Review"),
			"",
			m.styles.Hint.Render("No update transaction is currently active."),
			"",
			m.styles.Hint.Render("Press [Esc] or [Enter] to return."),
		}, "\n")

	case ReviewStateLoading:
		countStr = "Planning…"
		content = strings.Join([]string{
			m.styles.Heading.Render("Resolving System Update Transaction"),
			"",
			fmt.Sprintf("%s %s", m.styles.Brand.Render(theme.GlyphRunning), m.styles.Body.Render("Querying package manager dependencies and staging transaction artifact…")),
			"",
			m.styles.Meta.Render("This involves read-only DNF5 transaction resolution and local RPM staging."),
			m.styles.Hint.Render("No changes have been made to the system."),
			"",
			m.styles.Hint.Render("Press [Esc] to cancel."),
		}, "\n")

	case ReviewStateError:
		countStr = "Error"
		errMsg := "Unknown error occurred"
		if m.err != nil {
			errMsg = m.err.Error()
		}
		content = strings.Join([]string{
			m.styles.StatusError.Render(fmt.Sprintf("%s Update Planning Failed", theme.GlyphError)),
			"",
			m.styles.Body.Render(errMsg),
			"",
			m.styles.Meta.Render("The system was not modified."),
			"",
			m.styles.Hint.Render("Press [Esc] or [Enter] to return."),
		}, "\n")

	case ReviewStateEmpty:
		countStr = "0 updates"
		content = strings.Join([]string{
			m.styles.StatusSuccess.Render(fmt.Sprintf("%s System is Up to Date", theme.GlyphSuccess)),
			"",
			m.styles.Body.Render("DNF5 confirmed no packages require updating or installation."),
			"",
			m.styles.Hint.Render("Press [Esc] or [Enter] to return."),
		}, "\n")

	case ReviewStateConfirmed:
		countStr = "Confirmed"
		fpShort := m.decision.ArtifactFingerprint
		if len(fpShort) > 16 {
			fpShort = fpShort[:8] + "…" + fpShort[len(fpShort)-8:]
		}
		content = strings.Join([]string{
			m.styles.StatusSuccess.Render(fmt.Sprintf("%s Update Plan Confirmed by User", theme.GlyphSuccess)),
			"",
			fmt.Sprintf("%s  %s", m.styles.Meta.Render("Operation ID:"), m.styles.Body.Render(m.decision.OperationID)),
			fmt.Sprintf("%s  %s", m.styles.Meta.Render("Fingerprint: "), m.styles.Data.Render(fpShort)),
			fmt.Sprintf("%s  %s", m.styles.Meta.Render("Packages:    "), m.styles.Body.Render(fmt.Sprintf("%d packages approved", m.plan.TotalPackageCount()))),
			fmt.Sprintf("%s  %s", m.styles.Meta.Render("Download:    "), m.styles.Body.Render(FormatBytes(m.plan.TotalDownloadBytes))),
			"",
			m.styles.InnerRule.Render(strings.Repeat("─", max(1, interiorW))),
			"",
			m.styles.Brand.Render("Phase 2B.3 Boundary Invariant:"),
			m.styles.Body.Render("The user confirmed this plan, but NO execution has taken place."),
			m.styles.Meta.Render("Execution authorization (Phase 2B.3) is not started."),
			"",
			m.styles.Hint.Render("Press [Esc] or [Enter] to close."),
		}, "\n")

	case ReviewStateCancelled:
		countStr = "Cancelled"
		content = strings.Join([]string{
			m.styles.StatusWarning.Render(fmt.Sprintf("%s Update Transaction Cancelled", theme.GlyphWarning)),
			"",
			m.styles.Body.Render("The transaction plan review was cancelled by the user."),
			m.styles.Meta.Render("No packages were installed or modified. System state is unchanged."),
			"",
			m.styles.Hint.Render("Press [Esc] or [Enter] to close."),
		}, "\n")

	case ReviewStateReady:
		countStr = fmt.Sprintf("%d packages", m.plan.TotalPackageCount())
		if len(m.items) > 0 {
			posStr = fmt.Sprintf("%d of %d", m.table.Cursor()+1, len(m.items))
		}

		var lines []string

		// 1. Metrics summary row
		summaryParts := []string{
			fmt.Sprintf("%s: %s", m.styles.Meta.Render("Upgrades"), m.styles.Body.Render(fmt.Sprintf("%d", m.plan.UpgradeCount))),
		}
		if m.plan.InstallCount > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%s: %s", m.styles.Meta.Render("Installs"), m.styles.Body.Render(fmt.Sprintf("%d", m.plan.InstallCount))))
		}
		if m.plan.RemoveCount > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%s: %s", m.styles.Meta.Render("Removals"), m.styles.Body.Render(fmt.Sprintf("%d", m.plan.RemoveCount))))
		}
		if m.plan.DowngradeCount > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%s: %s", m.styles.Meta.Render("Downgrades"), m.styles.Body.Render(fmt.Sprintf("%d", m.plan.DowngradeCount))))
		}
		summaryParts = append(summaryParts, fmt.Sprintf("%s: %s", m.styles.Meta.Render("Download"), m.styles.Data.Render(FormatBytes(m.plan.TotalDownloadBytes))))
		lines = append(lines, strings.Join(summaryParts, "  ·  "))

		// 2. High-impact warnings (Kernel / Systemd)
		if m.plan.HasKernelUpdate || m.plan.HasSystemdUpdate {
			var flags []string
			if m.plan.HasKernelUpdate {
				flags = append(flags, fmt.Sprintf("%s Kernel update (reboot required after application)", theme.GlyphWarning))
			}
			if m.plan.HasSystemdUpdate {
				flags = append(flags, fmt.Sprintf("%s Systemd core update", theme.GlyphWarning))
			}
			lines = append(lines, m.styles.StatusWarning.Render(strings.Join(flags, "  ·  ")))
		}

		// 3. Artifact verification line
		fpShort := m.plan.Artifact.Fingerprint
		if len(fpShort) > 16 {
			fpShort = fpShort[:8] + "…" + fpShort[len(fpShort)-8:]
		}
		lines = append(lines, fmt.Sprintf("%s %s  ·  %s %s",
			m.styles.Meta.Render("Artifact:"),
			m.styles.Body.Render(m.plan.OperationID),
			m.styles.Meta.Render("Digest:"),
			m.styles.Data.Render(fpShort),
		))

		// 4. Divider above table
		lines = append(lines, m.styles.InnerRule.Render(strings.Repeat("─", max(1, interiorW))))

		// 5. Table rows
		lines = append(lines, m.table.View())

		// 6. Divider below table
		lines = append(lines, m.styles.InnerRule.Render(strings.Repeat("─", max(1, interiorW))))

		// 7. Interactive action buttons
		cancelStyle := lipgloss.NewStyle().Foreground(m.theme.Fg)
		confirmStyle := lipgloss.NewStyle().Foreground(m.theme.Accent).Bold(true)

		switch m.focusedControl {
		case ControlCancelButton:
			cancelStyle = lipgloss.NewStyle().Foreground(m.theme.Focus).Bold(true).Underline(true)
			confirmStyle = lipgloss.NewStyle().Foreground(m.theme.FgMuted)
		case ControlConfirmButton:
			cancelStyle = lipgloss.NewStyle().Foreground(m.theme.FgMuted)
			confirmStyle = lipgloss.NewStyle().Foreground(m.theme.SelectionFg).Background(m.theme.Focus).Bold(true)
		}

		btnCancel := cancelStyle.Render("[ Cancel ]")
		btnConfirm := confirmStyle.Render("[ Confirm Update ]")

		lines = append(lines, fmt.Sprintf("  %s      %s", btnCancel, btnConfirm))

		// 8. Navigation help
		lines = append(lines, m.styles.Hint.Render("Tab: switch focus · Enter: activate focused button · Esc: cancel · ↑/↓: browse table"))

		content = strings.Join(lines, "\n")
	}

	return components.RenderPane(components.PaneConfig{
		Title:     "Update Transaction Review",
		Count:     countStr,
		Position:  posStr,
		Width:     m.width,
		Height:    m.height,
		IsFocused: true,
	}, content, m.styles)
}
