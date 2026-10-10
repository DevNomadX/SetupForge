package pages

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/setupforge/setupforge/internal/domain"
	"github.com/setupforge/setupforge/internal/ui/components"
	"github.com/setupforge/setupforge/internal/ui/layout"
	"github.com/setupforge/setupforge/internal/ui/theme"
)

// LaunchpadItem represents a destination reachable from Overview.
type LaunchpadItem struct {
	DestinationIndex int
	Title            string
	Description      string
	Key              string
}

// OverviewModel represents the Overview (destination 1) workbench screen.
type OverviewModel struct {
	SystemInfo     domain.SystemInfo
	IsLoaded       bool
	IsFocused      bool
	SelectedIndex  int
	LaunchpadItems []LaunchpadItem

	UpdateStatus   domain.UpdateStatus
	UpdateLoaded   bool
	UpdateChecking bool
	UpdateErr      error
}

// NewOverviewModel constructs an OverviewModel with canonical launchpad entries.
func NewOverviewModel() OverviewModel {
	return OverviewModel{
		IsLoaded:       false,
		IsFocused:      true,
		SelectedIndex:  0,
		UpdateLoaded:   false,
		UpdateChecking: false,
		LaunchpadItems: []LaunchpadItem{
			{DestinationIndex: 1, Title: "Software", Description: "Browse packages, Flatpaks, and repositories", Key: "2"},
			{DestinationIndex: 2, Title: "Workstation", Description: "Configure desktop, fonts, and shell environment", Key: "3"},
			{DestinationIndex: 3, Title: "Development", Description: "Install language toolchains and build tools", Key: "4"},
			{DestinationIndex: 4, Title: "System", Description: "Check updates, system services, and health", Key: "5"},
			{DestinationIndex: 5, Title: "Journal", Description: "Review execution logs and operation history", Key: "6"},
			{DestinationIndex: 6, Title: "Settings", Description: "Configure SetupForge behavior and theme", Key: "7"},
		},
	}
}

// SetSystemInfo updates the model with detected host data.
func (m *OverviewModel) SetSystemInfo(info domain.SystemInfo) {
	m.SystemInfo = info
	m.IsLoaded = true
}

// SetUpdateStatus updates the model with the latest update availability status.
func (m *OverviewModel) SetUpdateStatus(status domain.UpdateStatus, err error) {
	m.UpdateStatus = status
	m.UpdateErr = err
	m.UpdateLoaded = true
	m.UpdateChecking = false
}

// SetCheckingUpdates transitions the model into the active checking state.
func (m *OverviewModel) SetCheckingUpdates() {
	m.UpdateChecking = true
}

// JumpDestinationMsg is emitted when a user selects a launchpad item in Overview.
type JumpDestinationMsg struct {
	DestinationIndex int
}

// RequestUpdateReviewMsg is emitted when the user requests update plan review.
type RequestUpdateReviewMsg struct{}

// Update handles key navigation within the Overview stage.
func (m OverviewModel) Update(msg tea.Msg) (OverviewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.SelectedIndex > 0 {
				m.SelectedIndex--
			}
		case "down", "j":
			if m.SelectedIndex < len(m.LaunchpadItems)-1 {
				m.SelectedIndex++
			}
		case "u":
			if m.UpdateStatus.IsAvailable() {
				return m, func() tea.Msg {
					return RequestUpdateReviewMsg{}
				}
			}
		case "enter":
			if m.SelectedIndex >= 0 && m.SelectedIndex < len(m.LaunchpadItems) {
				dest := m.LaunchpadItems[m.SelectedIndex].DestinationIndex
				return m, func() tea.Msg {
					return JumpDestinationMsg{DestinationIndex: dest}
				}
			}
		}
	}
	return m, nil
}

// Render renders the Overview stage based on responsive layout metrics.
func (m OverviewModel) Render(metrics layout.Metrics, styles theme.Styles) string {
	if metrics.StageWidth <= 0 || metrics.StageHeight <= 0 {
		return ""
	}

	// 1. Prepare Host Environment summary text
	var envLines []string
	if !m.IsLoaded {
		envLines = append(envLines, styles.Hint.Render("Detecting host environment…"))
	} else {
		statusGlyph := theme.GlyphSuccess
		statusText := "Fedora Verified"
		statusStyle := styles.StatusSuccess

		if !m.SystemInfo.IsSupported {
			statusGlyph = theme.GlyphWarning
			statusText = "Non-Fedora Host"
			statusStyle = styles.StatusWarning
		}

		envLines = append(envLines, statusStyle.Render(fmt.Sprintf("%s %s", statusGlyph, statusText)))
		envLines = append(envLines, "")
		envLines = append(envLines, fmt.Sprintf("%s  %s", styles.Meta.Render("OS:"), styles.Body.Render(m.SystemInfo.OS.PrettyName)))
		envLines = append(envLines, fmt.Sprintf("%s  %s", styles.Meta.Render("Kernel:"), styles.Data.Render(m.SystemInfo.Kernel)))
		envLines = append(envLines, fmt.Sprintf("%s  %s", styles.Meta.Render("Arch:"), styles.Data.Render(m.SystemInfo.Architecture)))
		if m.SystemInfo.Hostname != "" {
			envLines = append(envLines, fmt.Sprintf("%s  %s", styles.Meta.Render("Host:"), styles.Body.Render(m.SystemInfo.Hostname)))
		}
		if m.SystemInfo.OS.Variant != "" {
			envLines = append(envLines, fmt.Sprintf("%s  %s", styles.Meta.Render("Variant:"), styles.Body.Render(m.SystemInfo.OS.Variant)))
		}

		if m.UpdateChecking {
			envLines = append(envLines, fmt.Sprintf("%s  %s", styles.Meta.Render("Updates:"), styles.Hint.Render("Checking…")))
		} else if m.UpdateLoaded {
			if m.UpdateErr != nil || m.UpdateStatus.State == domain.UpdateStateFailed {
				envLines = append(envLines, fmt.Sprintf("%s  %s", styles.Meta.Render("Updates:"), styles.StatusWarning.Render(fmt.Sprintf("%s Check warning", theme.GlyphWarning))))
			} else if m.UpdateStatus.IsAvailable() {
				envLines = append(envLines, fmt.Sprintf("%s  %s  %s",
					styles.Meta.Render("Updates:"),
					styles.StatusWarning.Render(fmt.Sprintf("%d available %s", m.UpdateStatus.UpdateCount, theme.GlyphMarked)),
					styles.Key.Render("[u] Review plan"),
				))
			} else {
				envLines = append(envLines, fmt.Sprintf("%s  %s", styles.Meta.Render("Updates:"), styles.StatusSuccess.Render(fmt.Sprintf("%s Up to date", theme.GlyphSuccess))))
			}
		}
	}
	envContent := strings.Join(envLines, "\n")

	// 2. Determine pane widths for L/XL dual-pane vs S/M single-pane
	var leftW, rightW int
	if metrics.IsLarge || metrics.IsExtraLarge {
		// Balanced content-aware allocation: Host System needs ~44-52 cols.
		// Launchpad receives remaining width so descriptions never clip on wide screens.
		leftW = max(44, min(52, (metrics.StageWidth*35)/100))
		rightW = metrics.StageWidth - leftW
	} else {
		leftW = metrics.StageWidth
		rightW = metrics.StageWidth
	}

	interiorLaunchpadW := rightW - 2 - (theme.SpaceSM * 2)
	availDescWidth := interiorLaunchpadW - 19

	// 3. Prepare Launchpad list text
	var launchpadLines []string
	launchpadLines = append(launchpadLines, styles.Heading.Render("Workbench Quick Access"))
	launchpadLines = append(launchpadLines, "")

	for i, item := range m.LaunchpadItems {
		isSelected := m.IsFocused && (i == m.SelectedIndex)
		prefix := "  "
		if isSelected {
			prefix = theme.GlyphSelect + " "
		}

		keyBadge := styles.Key.Render(fmt.Sprintf("[%s]", item.Key))
		title := styles.Body.Render(item.Title)

		var descStr string
		if availDescWidth > 4 {
			descRaw := "— " + item.Description
			if lipgloss.Width(descRaw) > availDescWidth {
				descStr = ansi.Truncate(descRaw, availDescWidth, "…")
			} else {
				descStr = descRaw
			}
		}
		desc := styles.Meta.Render(descStr)

		line := fmt.Sprintf("%s%s %-12s %s", prefix, keyBadge, title, desc)
		if isSelected {
			line = styles.Selection.Render(line)
		}
		launchpadLines = append(launchpadLines, line)
	}
	launchpadContent := strings.Join(launchpadLines, "\n")

	// 4. Layout composition: Dual-pane for L/XL, Whole pane for S/M
	if metrics.IsLarge || metrics.IsExtraLarge {
		h := metrics.StageHeight

		leftPane := components.RenderPane(components.PaneConfig{
			Title:     "Host System",
			Count:     m.SystemInfo.OS.ID,
			Width:     leftW,
			Height:    h,
			IsFocused: false,
		}, envContent, styles)

		rightPane := components.RenderPane(components.PaneConfig{
			Title:     "Launchpad",
			Position:  fmt.Sprintf("%d of %d", m.SelectedIndex+1, len(m.LaunchpadItems)),
			Width:     rightW,
			Height:    h,
			IsFocused: m.IsFocused,
		}, launchpadContent, styles)

		return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	}

	// Single pane composition for S and M breakpoints
	var combined []string
	combined = append(combined, envLines...)
	combined = append(combined, "")
	combined = append(combined, styles.InnerRule.Render(strings.Repeat("─", max(1, metrics.StageWidth-6))))
	combined = append(combined, "")
	combined = append(combined, launchpadLines...)

	return components.RenderPane(components.PaneConfig{
		Title:     "Overview",
		Count:     m.SystemInfo.OS.ID,
		Width:     metrics.StageWidth,
		Height:    metrics.StageHeight,
		IsFocused: m.IsFocused,
	}, strings.Join(combined, "\n"), styles)
}
