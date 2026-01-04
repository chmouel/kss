package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/kss/internal/model"
	"github.com/chmouel/kss/internal/tekton"
)

func TestNewModel(t *testing.T) {
	m := NewModel("pod", "default", []string{})

	if m.resourceType != "pod" {
		t.Errorf("Expected resourceType 'pod', got '%s'", m.resourceType)
	}
	if m.namespace != "default" {
		t.Errorf("Expected namespace 'default', got '%s'", m.namespace)
	}
	if m.activeTab != tabOverview {
		t.Errorf("Expected activeTab %d, got %d", tabOverview, m.activeTab)
	}
	if m.focusedPane != paneList {
		t.Errorf("Expected focusedPane %d, got %d", paneList, m.focusedPane)
	}
	if m.ready {
		t.Error("Expected ready to be false")
	}
}

func TestUpdate_Navigation_Tabs(t *testing.T) {
	m := NewModel("pod", "default", []string{})
	m.ready = true

	// Test Tab key cycling
	// Initial: 0 (Overview)
	msg := tea.KeyMsg{Type: tea.KeyTab}

	// 0 -> 1
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(Model)
	if m.activeTab != tabLogs {
		t.Errorf("Expected activeTab %d (Logs), got %d", tabLogs, m.activeTab)
	}

	// 1 -> 2
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(Model)
	if m.activeTab != tabEvents {
		t.Errorf("Expected activeTab %d (Events), got %d", tabEvents, m.activeTab)
	}

	// Test Direct Number keys
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}} // '1' -> index 0
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(Model)
	if m.activeTab != tabOverview {
		t.Errorf("Expected activeTab %d (Overview), got %d", tabOverview, m.activeTab)
	}

	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}} // '4' -> index 3
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(Model)
	if m.activeTab != tabDoctor {
		t.Errorf("Expected activeTab %d (Doctor), got %d", tabDoctor, m.activeTab)
	}
}

func TestUpdate_Navigation_Panes(t *testing.T) {
	m := NewModel("pod", "default", []string{})
	m.ready = true

	// Initial: paneList (0)
	if m.focusedPane != paneList {
		t.Errorf("Expected initial focus on paneList, got %d", m.focusedPane)
	}

	// Right -> paneDetails
	msg := tea.KeyMsg{Type: tea.KeyRight}
	updatedModel, _ := m.Update(msg)
	m = updatedModel.(Model)
	if m.focusedPane != paneDetails {
		t.Errorf("Expected focus on paneDetails, got %d", m.focusedPane)
	}

	// Left -> paneList
	msg = tea.KeyMsg{Type: tea.KeyLeft}
	updatedModel, _ = m.Update(msg)
	m = updatedModel.(Model)
	if m.focusedPane != paneList {
		t.Errorf("Expected focus on paneList, got %d", m.focusedPane)
	}
}

func TestUpdate_WindowResize(t *testing.T) {
	m := NewModel("pod", "default", []string{})

	width := 100
	height := 50
	msg := tea.WindowSizeMsg{Width: width, Height: height}

	updatedModel, _ := m.Update(msg)
	m = updatedModel.(Model)

	if !m.ready {
		t.Error("Expected ready to be true after resize")
	}
	if m.width != width {
		t.Errorf("Expected width %d, got %d", width, m.width)
	}
	if m.height != height {
		t.Errorf("Expected height %d, got %d", height, m.height)
	}

	// Verify Layout Calculations
	// List width is roughly 1/3
	expectedListWidth := width / 3
	if m.list.Width() != expectedListWidth {
		t.Errorf("Expected list width %d, got %d", expectedListWidth, m.list.Width())
	}

	// Viewport Height should be m.height - 4 (The fix we implemented)
	expectedViewportHeight := height - 4
	if m.viewport.Height != expectedViewportHeight {
		t.Errorf("Expected viewport height %d, got %d", expectedViewportHeight, m.viewport.Height)
	}
}

func TestVisibleLength(t *testing.T) {
	// Simple string
	if l := visibleLength("hello"); l != 5 {
		t.Errorf("Expected visibleLength 5, got %d", l)
	}

	// String with ANSI codes
	colored := "\x1b[31mhello\x1b[0m"
	if l := visibleLength(colored); l != 5 {
		t.Errorf("Expected visibleLength 5 for colored string, got %d", l)
	}

	// String with Emoji (width can vary, but lipgloss handles it)
	// 🚀 is usually width 2 in terminals
	emoji := "🚀"
	if l := visibleLength(emoji); l != 2 {
		t.Errorf("Expected visibleLength 2 for emoji, got %d", l)
	}
}

func TestUpdate_Messages(t *testing.T) {
	m := NewModel("pod", "default", []string{})
	// Initialize with size so viewports can render
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updatedModel.(Model)
	m.ready = true

	pod := model.Pod{Metadata: model.PodMetadata{Name: "test-pod"}}
	items := []list.Item{NewPodItem(pod)}
	msg := ResourceMsg{items: items}

	var cmd tea.Cmd
	updatedModel, cmd = m.Update(msg)
	_ = cmd
	m = updatedModel.(Model)

	if len(m.list.Items()) != 1 {
		t.Errorf("Expected 1 item in list, got %d", len(m.list.Items()))
	}

	logsContent := "Log content here"
	logsMsg := LogsMsg{content: logsContent}

	// Switch to Logs tab first to ensure Viewport is updated
	m.activeTab = tabLogs
	updatedModel, _ = m.Update(logsMsg)
	m = updatedModel.(Model)

	// Check content indirectly via View() or directly if accessible?
	// The Viewport doesn't expose Content directly easily, but we can check if View() contains it.
	// Actually Viewport.View() returns the rendered content.
	if !strings.Contains(m.viewport.View(), logsContent) {
		t.Error("Expected logs content in viewport")
	}

	eventsContent := "Events happened"
	eventsMsg := EventsMsg{content: eventsContent}

	m.activeTab = tabEvents
	updatedModel, _ = m.Update(eventsMsg)
	m = updatedModel.(Model)

	if !strings.Contains(m.eventsViewport.View(), eventsContent) {
		t.Error("Expected events content in events viewport")
	}

	doctorResults := &DoctorResults{
		ResourceName: "test-pod",
		Findings: []DoctorFinding{
			{Severity: SeverityCritical, Message: "OOMKilled"},
		},
		AnalyzedAt: time.Now(),
	}
	doctorMsg := DoctorMsg{results: doctorResults}

	m.activeTab = tabDoctor
	updatedModel, _ = m.Update(doctorMsg)
	m = updatedModel.(Model)

	if m.doctorResults != doctorResults {
		t.Error("Expected doctorResults to be set")
	}
	if !strings.Contains(m.doctorViewport.View(), "OOMKilled") {
		t.Error("Expected finding in doctor viewport")
	}
}

func TestView_Rendering(t *testing.T) {
	m := NewModel("pod", "default", []string{})
	m.ready = true
	m.width = 100
	m.height = 30
	// Initialize resizing logic
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updatedModel.(Model)

	// Add an item
	pod := model.Pod{
		Metadata: model.PodMetadata{Name: "test-pod", Namespace: "default"},
		Status:   model.PodStatus{Phase: "Running", StartTime: "2023-01-01T00:00:00Z"},
	}
	m.list.SetItems([]list.Item{NewPodItem(pod)})

	// Test Overview View
	m.activeTab = tabOverview
	view := m.View()
	if !strings.Contains(view, "test-pod") {
		t.Error("Expected pod name in view")
	}
	if !strings.Contains(view, "Running") {
		t.Error("Expected status in view")
	}

	// Test Logs View with content
	m.activeTab = tabLogs
	m.viewport.SetContent("Log data")
	view = m.View()
	if !strings.Contains(view, "Log data") {
		t.Error("Expected log data in view")
	}
}

func TestItemDelegate_Render(t *testing.T) {
	d := itemDelegate{}
	l := list.New([]list.Item{}, d, 0, 0)

	// Test Pod Render
	pod := model.Pod{
		Metadata: model.PodMetadata{Name: "pod-1"},
		Status:   model.PodStatus{Phase: "Running"},
	}
	item := NewPodItem(pod)

	var buf strings.Builder
	d.Render(&buf, l, 0, item)

	output := buf.String()
	if !strings.Contains(output, "pod-1") {
		t.Error("Delegate render should contain pod name")
	}
	// Check for icon (roughly)
	if !strings.Contains(output, "") {
		t.Error("Delegate render should contain pod icon")
	}

	// Test PipelineRun Render
	pr := tekton.PipelineRun{
		Metadata: tekton.Metadata{Name: "pr-1"},
		Status: tekton.PipelineRunStatus{
			Conditions: []tekton.Condition{{Status: "True", Type: "Succeeded"}},
		},
	}
	prItem := NewPipelineRunItem(pr)

	buf.Reset()
	d.Render(&buf, l, 1, prItem)
	output = buf.String()

	if !strings.Contains(output, "pr-1") {
		t.Error("Delegate render should contain pipelinerun name")
	}
}
