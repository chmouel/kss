package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/kss/internal/tekton"
)

var (
	itemStyle         = lipgloss.NewStyle().PaddingLeft(2)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(0).Foreground(lipgloss.Color("205")).Bold(true) // Pink selection

	// Status Colors
	statusRunning = lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // Yellow
	statusSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // Green
	statusFail    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	statusWaiting = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // Gray

	// Text Colors
	dimmedText = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 2 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	var (
		title, desc string
		statusIcon  string
		statusStyle lipgloss.Style
	)

	switch i := item.(type) {
	case PodItem:
		title = i.title
		desc = i.desc

		// Determine status style and icon for Pod
		switch i.pod.Status.Phase {
		case "Running", "Succeeded":
			statusStyle = statusSuccess
			statusIcon = " " // Box
		case "Failed", "Error":
			statusStyle = statusFail
			statusIcon = "✖ "
		case "Pending":
			statusStyle = statusRunning
			statusIcon = "⏳ "
		default:
			statusStyle = statusWaiting
			statusIcon = "• "
		}
	case PipelineRunItem:
		title = i.title
		desc = i.desc

		// Determine status for PipelineRun
		_, color, _, _ := tekton.StatusLabel(i.pr.Status.Conditions)
		switch color {
		case "green":
			statusStyle = statusSuccess
			statusIcon = "🚀 "
		case "red", "magenta":
			statusStyle = statusFail
			statusIcon = "✖ "
		case "yellow":
			statusStyle = statusRunning
			statusIcon = "🏃 "
		default:
			statusStyle = statusWaiting
			statusIcon = "• "
		}
	default:
		return
	}

	if index == m.Index() {
		// Selected State

		// Border accent on the left for selected item
		selectedBar := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("│ ")

		_, _ = fmt.Fprint(w, selectedBar)
		_, _ = fmt.Fprint(w, selectedItemStyle.Render(statusIcon+title))
		_, _ = fmt.Fprint(w, "\n")
		_, _ = fmt.Fprint(w, selectedBar)
		_, _ = fmt.Fprint(w, dimmedText.Render("  "+desc))
	} else {
		// Normal State
		_, _ = fmt.Fprint(w, itemStyle.Render(statusStyle.Render(statusIcon)+title))
		_, _ = fmt.Fprint(w, "\n")
		_, _ = fmt.Fprint(w, itemStyle.Render(dimmedText.Render("  "+desc)))
	}
}
