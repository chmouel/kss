package tui

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chmouel/kss/internal/doctor"
	"github.com/chmouel/kss/internal/kube"
	"github.com/chmouel/kss/internal/model"
	"github.com/chmouel/kss/internal/tekton"
	"github.com/chmouel/kss/internal/util"
)

// Model state
type Model struct {
	list            list.Model
	viewport        viewport.Model
	eventsViewport  viewport.Model
	doctorViewport  viewport.Model
	ready           bool
	width, height   int
	activeTab       int
	resourceType    string // "pod" or "pipelinerun"
	namespace       string
	kubectlArgs     []string
	err             error
	ChosenItem      list.Item
	doctorResults   *DoctorResults
	isDoctorLoading bool
	focusedPane     int
}

const (
	paneList    = 0
	paneDetails = 1
)

const (
	tabOverview = iota
	tabLogs
	tabEvents
	tabDoctor
)

const (
	minListWidth    = 30
	minDetailsWidth = 40
	minHeight       = 10
)

var (
	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	// Overview Styles
	overviewHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")). // Cyan
				Bold(true).
				MarginBottom(1)

	sectionHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")). // Cyan
				Bold(true).
				MarginTop(1).
				MarginBottom(0)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")). // Gray
			Width(12)

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")) // White/Gray

	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // Green
	failedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	runningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("220")) // Yellow
	waitingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // Gray

	// Logs Styles
	logHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")). // Cyan
			Bold(true).
			MarginTop(1).
			MarginBottom(1)

	logContentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")) // Light Gray

	// Events Styles
	eventTimeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")) // Dark Gray

	eventReasonStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")) // White

	eventMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")). // Light Gray
				MarginLeft(4)

	// Doctor Styles
	doctorHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")). // Cyan
				Bold(true).
				MarginBottom(1)

	doctorRemediationStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("242")). // Gray
				Italic(true).
				MarginLeft(4)

	// Layout Styles
	appStyle = lipgloss.NewStyle().Margin(0, 0) // No margin to maximize space

	detailsStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("86")). // Cyan border
			Padding(0, 1)
)

// ResourceMsg carries the result of fetching resources (pods or pipelineruns).
type ResourceMsg struct {
	items []list.Item
	err   error
}

// LogsMsg carries the result of fetching logs for a resource.
type LogsMsg struct {
	content string
	err     error
}

// EventsMsg carries the result of fetching events for a resource.
type EventsMsg struct {
	content string
	err     error
}

// Severity levels for doctor findings
type Severity int

// Severity constants define the severity levels for doctor findings
const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityCritical
)

// DoctorFinding represents a single diagnostic finding
type DoctorFinding struct {
	Severity        Severity
	Message         string
	Remediation     string
	ContainerName   string
	IsInitContainer bool
}

// DoctorResults aggregates all findings for a resource
type DoctorResults struct {
	ResourceName string
	ResourceType string
	Findings     []DoctorFinding
	AnalyzedAt   time.Time
}

// DoctorMsg carries the result of doctor analysis
type DoctorMsg struct {
	results *DoctorResults
	err     error
}

// FetchResources returns a command that fetches the list of resources (pods or pipelineruns).
func (m Model) FetchResources() tea.Cmd {
	return func() tea.Msg {
		if m.resourceType == "pod" {
			pods, err := kube.ListPods(m.kubectlArgs)
			if err != nil {
				return ResourceMsg{err: err}
			}
			items := make([]list.Item, len(pods))
			for i := range pods {
				items[i] = NewPodItem(pods[i])
			}
			return ResourceMsg{items: items}
		} else {
			prs, err := tekton.ListPipelineRuns(m.kubectlArgs)
			if err != nil {
				return ResourceMsg{err: err}
			}
			items := make([]list.Item, len(prs))
			for i := range prs {
				items[i] = NewPipelineRunItem(prs[i])
			}
			return ResourceMsg{items: items}
		}
	}
}

// Event represents a Kubernetes event.
type Event struct {
	LastTimestamp string `json:"lastTimestamp"`
	Type          string `json:"type"`
	Reason        string `json:"reason"`
	Message       string `json:"message"`
	Count         int32  `json:"count"`
}

// EventList wraps a list of events.
type EventList struct {
	Items []Event `json:"items"`
}

// FetchEvents returns a command that fetches events for the given item.
func (m Model) FetchEvents(item list.Item) tea.Cmd {
	return func() tea.Msg {
		if podItem, ok := item.(PodItem); ok {
			pod := podItem.pod
			return m.fetchPodEvents(pod.Metadata.Name)
		}

		if prItem, ok := item.(PipelineRunItem); ok {
			pr := prItem.pr
			return m.fetchPipelineRunEvents(pr.Metadata.Name)
		}

		return EventsMsg{content: "Events not available for this resource type."}
	}
}

func (m Model) fetchPodEvents(podName string) EventsMsg {
	// Build kubectl command - multiple field selectors must be comma-separated
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName)
	cmdArgs := append(append([]string{}, m.kubectlArgs...),
		"get", "events",
		"--field-selector", fieldSelector,
		"-o", "json")

	cmd := exec.Command("kubectl", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return EventsMsg{err: fmt.Errorf("failed to fetch events: %w", err)}
	}

	var eventList EventList
	if err := json.Unmarshal(output, &eventList); err != nil {
		return EventsMsg{err: fmt.Errorf("failed to parse events: %w", err)}
	}

	if len(eventList.Items) == 0 {
		return EventsMsg{content: "No events found for this pod."}
	}

	// Filter and sort events
	var validEvents []Event
	for _, e := range eventList.Items {
		if e.LastTimestamp != "" {
			validEvents = append(validEvents, e)
		}
	}

	if len(validEvents) == 0 {
		return EventsMsg{content: "No events with timestamps found."}
	}

	// Sort by timestamp
	slices.SortFunc(validEvents, func(a, b Event) int {
		return strings.Compare(a.LastTimestamp, b.LastTimestamp)
	})

	// Format events
	var eventBuilder strings.Builder
	eventBuilder.WriteString(overviewHeaderStyle.Render(fmt.Sprintf("📅 Events for Pod: %s", podName)) + "\n\n")

	for _, event := range validEvents {
		// Parse timestamp
		eventTime, err := time.Parse(time.RFC3339, event.LastTimestamp)
		var timeStr string
		if err == nil {
			timeStr = eventTime.Format("15:04:05")
		} else {
			timeStr = event.LastTimestamp
		}

		// Format based on event type
		typeIcon := "•"
		var typeStyle lipgloss.Style
		if event.Type == "Warning" {
			typeIcon = "⚠"
			typeStyle = failedStyle
		} else {
			typeStyle = successStyle
		}

		eventBuilder.WriteString(fmt.Sprintf("%s %s %s\n",
			eventTimeStyle.Render(timeStr),
			typeStyle.Render(typeIcon),
			eventReasonStyle.Render(event.Reason)))

		eventBuilder.WriteString(eventMessageStyle.Render(event.Message) + "\n")

		if event.Count > 1 {
			eventBuilder.WriteString(eventMessageStyle.Render(fmt.Sprintf("(occurred %d times)", event.Count)) + "\n")
		}
		eventBuilder.WriteString("\n")
	}

	return EventsMsg{content: eventBuilder.String()}
}

func (m Model) fetchPipelineRunEvents(prName string) EventsMsg {
	// For PipelineRuns, we fetch events for the TaskRuns
	// First, fetch all TaskRuns for this PipelineRun
	taskRuns, err := tekton.FetchTaskRunsForPipelineRun(m.kubectlArgs, prName)
	if err != nil {
		return EventsMsg{err: fmt.Errorf("failed to fetch TaskRuns: %w", err)}
	}

	if len(taskRuns) == 0 {
		return EventsMsg{content: "No TaskRuns found for this PipelineRun."}
	}

	// Collect all events from all TaskRuns (both TaskRun events and Pod events)
	var allEvents []Event
	var eventBuilder strings.Builder
	eventBuilder.WriteString(overviewHeaderStyle.Render(fmt.Sprintf("📅 Events for PipelineRun: %s", prName)) + "\n\n")

	for i := range taskRuns {
		tr := &taskRuns[i]
		taskName := tekton.TaskRunDisplayName(*tr)
		trName := tr.Metadata.Name

		// Fetch TaskRun events
		fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=TaskRun", trName)
		cmdArgs := append(append([]string{}, m.kubectlArgs...),
			"get", "events",
			"--field-selector", fieldSelector,
			"-o", "json")

		cmd := exec.Command("kubectl", cmdArgs...)
		output, err := cmd.CombinedOutput()

		var taskRunEvents []Event
		if err == nil {
			var eventList EventList
			if err := json.Unmarshal(output, &eventList); err == nil {
				for _, e := range eventList.Items {
					if e.LastTimestamp != "" {
						taskRunEvents = append(taskRunEvents, e)
					}
				}
			}
		}

		// Also fetch Pod events for this TaskRun
		podName, err := tekton.PodNameForTaskRun(m.kubectlArgs, *tr)
		var podEvents []Event
		if err == nil {
			fieldSelector = fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName)
			cmdArgs = append(append([]string{}, m.kubectlArgs...),
				"get", "events",
				"--field-selector", fieldSelector,
				"-o", "json")

			cmd = exec.Command("kubectl", cmdArgs...)
			output, err = cmd.CombinedOutput()

			if err == nil {
				var eventList EventList
				if err := json.Unmarshal(output, &eventList); err == nil {
					for _, e := range eventList.Items {
						if e.LastTimestamp != "" {
							podEvents = append(podEvents, e)
						}
					}
				}
			}
		}

		// Combine TaskRun and Pod events
		combinedEvents := make([]Event, 0, len(taskRunEvents)+len(podEvents))
		combinedEvents = append(combinedEvents, taskRunEvents...)
		combinedEvents = append(combinedEvents, podEvents...)

		if len(combinedEvents) > 0 {
			// Sort by timestamp
			slices.SortFunc(combinedEvents, func(a, b Event) int {
				return strings.Compare(a.LastTimestamp, b.LastTimestamp)
			})

			// Add header for this TaskRun
			eventBuilder.WriteString(sectionHeaderStyle.Render(fmt.Sprintf("--- TaskRun: %s ---", taskName)) + "\n")

			for _, event := range combinedEvents {
				// Parse timestamp
				eventTime, err := time.Parse(time.RFC3339, event.LastTimestamp)
				var timeStr string
				if err == nil {
					timeStr = eventTime.Format("15:04:05")
				} else {
					timeStr = event.LastTimestamp
				}

				// Format based on event type
				typeIcon := "•"
				var typeStyle lipgloss.Style
				if event.Type == "Warning" {
					typeIcon = "⚠"
					typeStyle = failedStyle
				} else {
					typeStyle = successStyle
				}

				eventBuilder.WriteString(fmt.Sprintf("%s %s %s\n",
					eventTimeStyle.Render(timeStr),
					typeStyle.Render(typeIcon),
					eventReasonStyle.Render(event.Reason)))

				eventBuilder.WriteString(eventMessageStyle.Render(event.Message) + "\n")

				if event.Count > 1 {
					eventBuilder.WriteString(eventMessageStyle.Render(fmt.Sprintf("(occurred %d times)", event.Count)) + "\n")
				}
			}
			eventBuilder.WriteString("\n")

			allEvents = append(allEvents, combinedEvents...)
		}
	}

	if len(allEvents) == 0 {
		return EventsMsg{content: fmt.Sprintf("No events found for PipelineRun: %s", prName)}
	}

	return EventsMsg{content: eventBuilder.String()}
}

// FetchLogs returns a command that fetches logs for the given item.
func (m Model) FetchLogs(item list.Item) tea.Cmd {
	return func() tea.Msg {
		if podItem, ok := item.(PodItem); ok {
			pod := podItem.pod
			if len(pod.Status.ContainerStatuses) == 0 {
				return LogsMsg{err: fmt.Errorf("no containers in pod")}
			}
			// Just fetch logs for the first container for now
			container := pod.Status.ContainerStatuses[0].Name
			logs, err := kube.ShowLog("kubectl", model.Args{MaxLines: "100"}, container, pod.Metadata.Name, false)

			// Simple style for single pod logs
			var logBuilder strings.Builder
			logBuilder.WriteString(overviewHeaderStyle.Render(fmt.Sprintf("📜 Logs for Pod: %s", pod.Metadata.Name)) + "\n\n")
			// Force wrap logs to viewport width to prevent layout breaking
			logBuilder.WriteString(logContentStyle.Width(m.viewport.Width).Render(logs))

			return LogsMsg{content: logBuilder.String(), err: err}
		}

		if prItem, ok := item.(PipelineRunItem); ok {
			pr := prItem.pr
			// Fetch TaskRuns for this PipelineRun
			taskRuns, err := tekton.FetchTaskRunsForPipelineRun(m.kubectlArgs, pr.Metadata.Name)
			if err != nil {
				return LogsMsg{err: fmt.Errorf("failed to fetch TaskRuns: %w", err)}
			}

			if len(taskRuns) == 0 {
				return LogsMsg{content: "No TaskRuns found for this PipelineRun."}
			}

			// Build a summary of logs from all TaskRuns
			var logBuilder strings.Builder
			logBuilder.WriteString(overviewHeaderStyle.Render(fmt.Sprintf("📜 Logs for PipelineRun: %s", pr.Metadata.Name)) + "\n\n")

			for i := range taskRuns {
				tr := &taskRuns[i]
				taskName := tekton.TaskRunDisplayName(*tr)

				// Get the pod for this TaskRun
				podName, err := tekton.PodNameForTaskRun(m.kubectlArgs, *tr)
				if err != nil {
					logBuilder.WriteString(fmt.Sprintf("[%s] Error: %v\n\n", taskName, err))
					continue
				}

				// Fetch the pod to get container names
				pod, err := kube.FetchPod(m.kubectlArgs, podName)
				if err != nil {
					logBuilder.WriteString(fmt.Sprintf("[%s] Error fetching pod: %v\n\n", taskName, err))
					continue
				}

				// Get logs from the first step container (usually the main task)
				if len(pod.Spec.Containers) > 0 {
					container := pod.Spec.Containers[0].Name
					header := fmt.Sprintf("--- TaskRun: %s (Container: %s) ---", taskName, container)
					logBuilder.WriteString(logHeaderStyle.Render(header) + "\n")

					logs, err := kube.ShowLog("kubectl", model.Args{MaxLines: "50"}, container, podName, false)
					if err != nil {
						logBuilder.WriteString(fmt.Sprintf("Error fetching logs: %v\n", err))
					} else {
						logBuilder.WriteString(logContentStyle.Render(logs))
					}
					logBuilder.WriteString("\n\n")
				}
			}

			return LogsMsg{content: logBuilder.String()}
		}

		return LogsMsg{content: "Logs not available for this resource type."}
	}
}

// shouldAnalyzeLogs determines if logs should be fetched for analysis
func shouldAnalyzeLogs(container model.ContainerStatus) bool {
	return container.State.Terminated != nil ||
		container.RestartCount > 0 ||
		(container.State.Waiting != nil && container.State.Waiting.Reason == "CrashLoopBackOff")
}

// FetchDoctor returns a command that performs doctor analysis on the given item
func (m Model) FetchDoctor(item list.Item) tea.Cmd {
	return func() tea.Msg {
		if podItem, ok := item.(PodItem); ok {
			return m.fetchPodDoctor(podItem.pod)
		}

		if prItem, ok := item.(PipelineRunItem); ok {
			return m.fetchPipelineRunDoctor(prItem.pr)
		}

		return DoctorMsg{err: fmt.Errorf("doctor not available for this resource type")}
	}
}

// fetchPodDoctor analyzes a single pod
func (m Model) fetchPodDoctor(pod model.Pod) DoctorMsg {
	results := &DoctorResults{
		ResourceName: pod.Metadata.Name,
		ResourceType: "pod",
		Findings:     []DoctorFinding{},
		AnalyzedAt:   time.Now(),
	}

	// Analyze init containers
	for _, container := range pod.Status.InitContainerStatuses {
		findings := m.analyzeContainer(container, pod, true)
		results.Findings = append(results.Findings, findings...)
	}

	// Analyze regular containers
	for _, container := range pod.Status.ContainerStatuses {
		findings := m.analyzeContainer(container, pod, false)
		results.Findings = append(results.Findings, findings...)
	}

	return DoctorMsg{results: results}
}

// fetchPipelineRunDoctor analyzes all TaskRun pods in a PipelineRun
func (m Model) fetchPipelineRunDoctor(pr tekton.PipelineRun) DoctorMsg {
	results := &DoctorResults{
		ResourceName: pr.Metadata.Name,
		ResourceType: "pipelinerun",
		Findings:     []DoctorFinding{},
		AnalyzedAt:   time.Now(),
	}

	// Fetch TaskRuns for this PipelineRun
	taskRuns, err := tekton.FetchTaskRunsForPipelineRun(m.kubectlArgs, pr.Metadata.Name)
	if err != nil {
		return DoctorMsg{err: fmt.Errorf("failed to fetch TaskRuns: %w", err)}
	}

	if len(taskRuns) == 0 {
		return DoctorMsg{results: results} // No TaskRuns = no findings
	}

	// Analyze each TaskRun's pod
	for i := range taskRuns {
		tr := &taskRuns[i]
		podName, err := tekton.PodNameForTaskRun(m.kubectlArgs, *tr)
		if err != nil {
			// Add finding about unable to get pod
			results.Findings = append(results.Findings, DoctorFinding{
				Severity:      SeverityWarning,
				Message:       fmt.Sprintf("TaskRun %s: Unable to fetch pod", tekton.TaskRunDisplayName(*tr)),
				Remediation:   "Check TaskRun status for pod scheduling issues",
				ContainerName: tekton.TaskRunDisplayName(*tr),
			})
			continue
		}

		pod, err := kube.FetchPod(m.kubectlArgs, podName)
		if err != nil {
			results.Findings = append(results.Findings, DoctorFinding{
				Severity:      SeverityWarning,
				Message:       fmt.Sprintf("TaskRun %s: Unable to fetch pod %s", tekton.TaskRunDisplayName(*tr), podName),
				Remediation:   "Verify pod exists and is accessible",
				ContainerName: tekton.TaskRunDisplayName(*tr),
			})
			continue
		}

		// Analyze containers in this TaskRun's pod
		for _, container := range pod.Status.ContainerStatuses {
			findings := m.analyzeContainer(container, pod, false)
			// Prefix container name with TaskRun name for context
			for i := range findings {
				findings[i].ContainerName = fmt.Sprintf("%s/%s", tekton.TaskRunDisplayName(*tr), findings[i].ContainerName)
			}
			results.Findings = append(results.Findings, findings...)
		}
	}

	return DoctorMsg{results: results}
}

// analyzeContainer performs doctor analysis on a single container
func (m Model) analyzeContainer(container model.ContainerStatus, pod model.Pod, isInit bool) []DoctorFinding {
	findings := make([]DoctorFinding, 0, 5) // Pre-allocate with capacity

	// 1. Analyze container state using existing doctor functions
	stateIssues := doctor.AnalyzeContainerState(container)
	for _, issue := range stateIssues {
		finding := DoctorFinding{
			Message:         issue,
			ContainerName:   container.Name,
			IsInitContainer: isInit,
		}

		// Categorize severity and add remediation
		finding.Severity, finding.Remediation = categorizeAndRemediate(issue, container)
		findings = append(findings, finding)
	}

	// 2. Check for high restart count (stability warning)
	if container.State.Running != nil && container.RestartCount > 3 {
		findings = append(findings, DoctorFinding{
			Severity:        SeverityWarning,
			Message:         fmt.Sprintf("Container has restarted %d times", container.RestartCount),
			Remediation:     "Check logs for intermittent crashes. Consider increasing memory/CPU limits or reviewing application stability.",
			ContainerName:   container.Name,
			IsInitContainer: isInit,
		})
	}

	// 3. Analyze logs if container is problematic
	if shouldAnalyzeLogs(container) {
		logs, err := kube.ShowLog("kubectl", model.Args{MaxLines: "100"}, container.Name, pod.Metadata.Name, container.RestartCount > 0)
		if err == nil && logs != "" {
			logIssues := doctor.AnalyzeLogs(logs)
			for _, issue := range logIssues {
				finding := DoctorFinding{
					Message:         issue,
					ContainerName:   container.Name,
					IsInitContainer: isInit,
				}
				finding.Severity, finding.Remediation = categorizeLogIssue(issue)
				findings = append(findings, finding)
			}
		}
	}

	return findings
}

// categorizeAndRemediate assigns severity and remediation based on container state issue
func categorizeAndRemediate(issue string, container model.ContainerStatus) (Severity, string) {
	issueLower := strings.ToLower(issue)

	// Critical issues (container cannot run)
	if strings.Contains(issueLower, "oomkilled") {
		return SeverityCritical, "Increase memory limits in pod spec. Check resource usage patterns in monitoring."
	}
	if strings.Contains(issueLower, "imagepullbackoff") || strings.Contains(issueLower, "errimagepull") {
		return SeverityCritical, "Verify image name/tag. Check imagePullSecrets if registry requires authentication."
	}
	if strings.Contains(issueLower, "crashloopbackoff") {
		return SeverityCritical, "Review application logs for startup errors. Check readiness/liveness probe configuration."
	}
	if strings.Contains(issueLower, "createcontainerconfigerror") {
		return SeverityCritical, "Verify referenced ConfigMaps and Secrets exist. Check volume mount configurations."
	}

	// Warning issues (container unstable or degraded)
	if strings.Contains(issueLower, "exit code") {
		return SeverityWarning, "Application exited with error. Review logs and check application health."
	}
	if strings.Contains(issueLower, "command not found") {
		return SeverityWarning, "Verify container entrypoint/command in pod spec. Check binary exists in container image."
	}

	// Default to info
	return SeverityInfo, "Review container status and logs for additional context."
}

// loadTabContent returns the command to fetch content for the active tab
func (m Model) loadTabContent(item list.Item) tea.Cmd {
	switch m.activeTab {
	case tabLogs:
		return m.FetchLogs(item)
	case tabEvents:
		return m.FetchEvents(item)
	case tabDoctor:
		m.isDoctorLoading = true
		m.doctorResults = nil
		return m.FetchDoctor(item)
	}
	return nil
}

// categorizeLogIssue assigns severity and remediation based on log analysis
func categorizeLogIssue(issue string) (Severity, string) {
	issueLower := strings.ToLower(issue)

	if strings.Contains(issueLower, "connection refused") || strings.Contains(issueLower, "dial tcp") {
		return SeverityWarning, "Ensure dependent services are running and accessible. Check service DNS and network policies."
	}
	if strings.Contains(issueLower, "timeout") || strings.Contains(issueLower, "deadline exceeded") {
		return SeverityWarning, "Increase timeout values if appropriate. Check service responsiveness and network latency."
	}
	if strings.Contains(issueLower, "permission denied") || strings.Contains(issueLower, "forbidden") {
		return SeverityWarning, "Review Pod SecurityContext, serviceAccount permissions, and file ownership."
	}
	if strings.Contains(issueLower, "missing file") || strings.Contains(issueLower, "not found") {
		return SeverityWarning, "Verify volume mounts and ConfigMap/Secret contents. Check file paths in application config."
	}

	return SeverityInfo, "Review logs for additional context and error patterns."
}

// NewModel creates a new TUI model for the given resource type.
func NewModel(resourceType, namespace string, kubectlArgs []string) Model {
	// Initialize with dummy data, will be populated on init
	l := list.New([]list.Item{}, itemDelegate{}, 0, 0)
	l.Title = fmt.Sprintf("KSS Dashboard - %s", strings.ToUpper(resourceType[:1])+resourceType[1:])
	l.Styles.Title = titleStyle
	l.SetShowHelp(false)

	vp := viewport.New(0, 0)
	// No styling - we draw our own borders in View()
	vp.Style = lipgloss.NewStyle()

	evp := viewport.New(0, 0)
	evp.Style = lipgloss.NewStyle()

	dvp := viewport.New(0, 0)
	dvp.Style = lipgloss.NewStyle()

	return Model{
		list:            l,
		viewport:        vp,
		eventsViewport:  evp,
		doctorViewport:  dvp,
		resourceType:    resourceType,
		namespace:       namespace,
		kubectlArgs:     kubectlArgs,
		isDoctorLoading: false,
		doctorResults:   nil,
	}
}

// Init initializes the model and returns the initial command.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.FetchResources(), tea.EnterAltScreen)
}

// Update handles messages and updates the model state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case ResourceMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.list.SetItems(msg.items)
		}

	case LogsMsg:
		if msg.err != nil {
			m.viewport.SetContent(fmt.Sprintf("Error fetching logs: %v", msg.err))
		} else {
			m.viewport.SetContent(msg.content)
		}

	case EventsMsg:
		if msg.err != nil {
			m.eventsViewport.SetContent(fmt.Sprintf("Error fetching events: %v", msg.err))
		} else {
			m.eventsViewport.SetContent(msg.content)
		}

	case DoctorMsg:
		m.isDoctorLoading = false
		if msg.err != nil {
			m.doctorViewport.SetContent(fmt.Sprintf("Error performing doctor analysis: %v", msg.err))
		} else {
			m.doctorResults = msg.results
			content := m.renderDoctorResults(msg.results, m.doctorViewport.Width)
			m.doctorViewport.SetContent(content)
		}

	case tea.KeyMsg:
		// Global Navigation (Tabs)
		switch msg.String() {
		case "tab":
			m.activeTab = (m.activeTab + 1) % 4
			item := m.list.SelectedItem()
			if item != nil {
				cmds = append(cmds, m.loadTabContent(item))
			}
			return m, tea.Batch(cmds...)
		case "1", "2", "3", "4":
			m.activeTab = int(msg.Runes[0] - '1')
			item := m.list.SelectedItem()
			if item != nil {
				cmds = append(cmds, m.loadTabContent(item))
			}
			return m, tea.Batch(cmds...)
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if m.focusedPane == paneList {
				m.ChosenItem = m.list.SelectedItem()
				return m, tea.Quit
			}
		}

		// Pane Navigation
		switch msg.String() {
		case "left":
			m.focusedPane = paneList
			return m, nil
		case "right":
			m.focusedPane = paneDetails
			return m, nil
		}

		// Routed Input
		if m.focusedPane == paneList {
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)

			// If selection changed, reload tab content
			// We can't easily detect selection change without storing prev selection,
			// but triggering load is safe (it's async).
			// Actually, list update might not have changed selection yet.
			// But for simplicity, we can let the user press enter or just re-fetch on tab switch.
			// Better: Check if list item changed?
			// For now, let's just update the list. The viewport content updates when switching tabs
			// or when explicitly requested.
			// To make it responsive, we should update content on selection change.
			// But list.Update doesn't return "selection changed" event.
			// We can check m.list.SelectedItem() before and after.

			// However, keeping it simple: selecting an item is "Enter" or just navigating?
			// The original code updated content on tab switch or init.
			// Let's stick to that for now to avoid excessive fetches while scrolling.

		} else {
			// Details Pane Input
			switch m.activeTab {
			case tabLogs:
				m.viewport, cmd = m.viewport.Update(msg)
			case tabEvents:
				m.eventsViewport, cmd = m.eventsViewport.Update(msg)
			case tabDoctor:
				m.doctorViewport, cmd = m.doctorViewport.Update(msg)
			}
			cmds = append(cmds, cmd)
		}

		return m, tea.Batch(cmds...)

	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.width = msg.Width - h
		m.height = msg.Height - v

		// Ensure minimum dimensions
		if m.width < minListWidth+minDetailsWidth {
			m.width = minListWidth + minDetailsWidth
		}
		if m.height < minHeight {
			m.height = minHeight
		}

		// Calculate list width (1/3 of total)
		listWidth := m.width / 3
		if listWidth < minListWidth {
			listWidth = minListWidth
		}

		// Calculate details width (remaining space minus borders)
		// Border takes 2 width (left+right)
		detailsWidth := m.width - listWidth - 2
		if detailsWidth < minDetailsWidth {
			detailsWidth = minDetailsWidth
		}

		// Set list dimensions
		m.list.SetSize(listWidth, m.height)

		// Set viewport dimensions
		// Tab bar takes 1 line
		// Border takes 2 height (top+bottom)
		// Extra 1 line for safety against wrapping
		viewportHeight := m.height - 4
		if viewportHeight < 5 {
			viewportHeight = 5
		}

		m.viewport.Width = detailsWidth - 2 // Content padding
		m.viewport.Height = viewportHeight
		m.eventsViewport.Width = detailsWidth - 2
		m.eventsViewport.Height = viewportHeight
		m.doctorViewport.Width = detailsWidth - 2
		m.doctorViewport.Height = viewportHeight

		m.ready = true
	}

	return m, tea.Batch(cmds...)
}

// visibleLength calculates the visible length of a string, excluding ANSI escape codes.
func visibleLength(s string) int {
	return lipgloss.Width(s)
}

// View renders the TUI to a string.
func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing Dashboard..."
	}

	// Defensive check: ensure we have valid dimensions
	if m.width <= 0 || m.height <= 0 {
		return "\n  Terminal too small. Please resize."
	}

	listView := m.list.View()

	// Determine border color based on focus
	var borderColor lipgloss.Color
	if m.focusedPane == paneDetails {
		borderColor = lipgloss.Color("86") // Cyan (active)
	} else {
		borderColor = lipgloss.Color("240") // Dim gray (inactive)
	}

	tabs := []string{"Overview", "Logs", "Events", "Doctor"}
	var renderedTabs []string

	for i, t := range tabs {
		var style lipgloss.Style
		isFirst := i == 0
		isActive := i == m.activeTab

		if isActive {
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")).
				Background(borderColor). // Matches border color dynamically
				Bold(true).
				Padding(0, 1)
		} else {
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Padding(0, 1)
		}

		// Add number prefix
		content := fmt.Sprintf("%d:%s", i+1, t)

		if isFirst {
			renderedTabs = append(renderedTabs, style.Render(content))
		} else {
			renderedTabs = append(renderedTabs, style.Render(" "+content))
		}
	}

		// Create the tab bar row
		tabBar := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
		
		detailsContent := m.renderDetails()
		
		// Wrap details in the rounded border box	// Ensure the box fills the remaining height
	// Available height = m.height
	// Tab bar = 1 line
	// Border = 2 lines (top/bottom)
	// Content height should be: m.height - 1 (tabs) - 2 (borders) - 1 (safety) = m.height - 4
	detailsBox := detailsStyle.
		Width(m.viewport.Width). // viewport width already accounts for border padding in Update
		Height(m.height - 4).
		BorderForeground(borderColor).
		Render(detailsContent)

	rightSide := lipgloss.JoinVertical(lipgloss.Left, tabBar, detailsBox)

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, listView, rightSide)

	return appStyle.Render(mainView)
}

func (m Model) renderDetails() string {
	if m.err != nil {
		return fmt.Sprintf("\033[38;5;196mError: %v\033[0m", m.err)
	}

	item := m.list.SelectedItem()
	if item == nil {
		return "No item selected."
	}

	switch m.activeTab {
	case tabOverview:
		return m.renderOverview(item)
	case tabLogs:
		// Viewport handles scrolling and returns visible content
		return m.viewport.View()
	case tabEvents:
		// Events viewport handles scrolling and returns visible content
		return m.eventsViewport.View()
	case tabDoctor:
		if m.isDoctorLoading {
			return "\033[38;5;242mAnalyzing containers...\033[0m"
		}
		return m.doctorViewport.View()
	}

	return ""
}

func (m Model) renderOverview(item list.Item) string {
	var sb strings.Builder

	if podItem, ok := item.(PodItem); ok {
		pod := podItem.pod

		// Header
		sb.WriteString(overviewHeaderStyle.Render(fmt.Sprintf("  Pod: %s", pod.Metadata.Name)) + "\n")

		// Details
		sb.WriteString(renderRow("🔖 Namespace", pod.Metadata.Namespace))

		// Status with color
		status := pod.Status.Phase
		var style lipgloss.Style
		switch status {
		case "Running", "Succeeded":
			style = successStyle
		case "Failed", "Error":
			style = failedStyle
		case "Pending":
			style = runningStyle
		default:
			style = waitingStyle
		}
		sb.WriteString(renderRow("🚥 Phase", style.Render(status)))

		sb.WriteString(renderRow("🕒 Age", util.FormatDuration(pod.Status.StartTime)))

		if len(pod.Status.ContainerStatuses) > 0 {
			sb.WriteString(sectionHeaderStyle.Render("\n🐳 Containers") + "\n")
			for _, c := range pod.Status.ContainerStatuses {
				status, _ := c.StateLabel()

				// Style the container status
				var cStyle lipgloss.Style
				if strings.HasPrefix(status, "Running") || strings.HasPrefix(status, "Completed") {
					cStyle = successStyle
				} else if strings.Contains(status, "Error") || strings.Contains(status, "BackOff") {
					cStyle = failedStyle
				} else {
					cStyle = waitingStyle
				}

				sb.WriteString(fmt.Sprintf("  %s %s: %s\n", cStyle.Render("•"), c.Name, cStyle.Render(status)))
			}
		}
		return sb.String()
	}

	if prItem, ok := item.(PipelineRunItem); ok {
		pr := prItem.pr

		// Header
		sb.WriteString(overviewHeaderStyle.Render(fmt.Sprintf("🚀 PipelineRun: %s", pr.Metadata.Name)) + "\n")

		// Details
		sb.WriteString(renderRow("🔖 Namespace", pr.Metadata.Namespace))

		label, color, reason, _ := tekton.StatusLabel(pr.Status.Conditions)

		// Map text color to lipgloss style
		var style lipgloss.Style
		switch color {
		case "green":
			style = successStyle
		case "red", "magenta":
			style = failedStyle
		case "yellow":
			style = runningStyle
		default:
			style = waitingStyle
		}

		statusText := label
		if reason != "" && label != "Succeeded" {
			statusText = fmt.Sprintf("%s (%s)", label, reason)
		}
		sb.WriteString(renderRow("🚥 Status", style.Render(statusText)))

		sb.WriteString(renderRow("🕒 Age", util.FormatDuration(pr.Metadata.CreationTimestamp)))

		return sb.String()
	}

	return "Unknown item type"
}

func renderRow(label, value string) string {
	return fmt.Sprintf("%s %s\n", labelStyle.Render(label), valueStyle.Render(value))
}

// renderDoctorResults formats doctor analysis results for display
func (m Model) renderDoctorResults(results *DoctorResults, viewportWidth int) string {
	if results == nil {
		return "No doctor analysis available."
	}

	var sb strings.Builder

	// Header
	sb.WriteString(overviewHeaderStyle.Render(fmt.Sprintf("🚑 Doctor Analysis: %s", results.ResourceName)) + "\n")
	sb.WriteString(fmt.Sprintf("Analyzed at: %s\n\n", results.AnalyzedAt.Format("15:04:05")))

	if len(results.Findings) == 0 {
		sb.WriteString(successStyle.Render("✨ No issues detected. All containers appear healthy.") + "\n")
		return sb.String()
	}

	// Group findings by container
	containerFindings := make(map[string][]DoctorFinding)
	for _, finding := range results.Findings {
		containerFindings[finding.ContainerName] = append(containerFindings[finding.ContainerName], finding)
	}

	// Sort containers for consistent display
	containers := make([]string, 0, len(containerFindings))
	for name := range containerFindings {
		containers = append(containers, name)
	}
	slices.Sort(containers)

	// Render findings by container
	for _, containerName := range containers {
		findings := containerFindings[containerName]

		// Container header
		isInit := findings[0].IsInitContainer
		prefix := ""
		if isInit {
			prefix = "(Init) "
		}

		sb.WriteString(sectionHeaderStyle.Render(fmt.Sprintf("\n📦 %sContainer: %s", prefix, containerName)) + "\n")

		// Group by severity
		critical := []DoctorFinding{}
		warnings := []DoctorFinding{}
		info := []DoctorFinding{}

		for _, f := range findings {
			switch f.Severity {
			case SeverityCritical:
				critical = append(critical, f)
			case SeverityWarning:
				warnings = append(warnings, f)
			case SeverityInfo:
				info = append(info, f)
			}
		}

		// Render critical findings
		for _, f := range critical {
			sb.WriteString(renderFinding(f, "✖", failedStyle)) // Red bold X
		}

		// Render warnings
		for _, f := range warnings {
			sb.WriteString(renderFinding(f, "⚠", runningStyle)) // Yellow bold warning
		}

		// Render info
		for _, f := range info {
			sb.WriteString(renderFinding(f, "ℹ", lipgloss.NewStyle().Foreground(lipgloss.Color("39")))) // Blue info
		}

		sb.WriteString("\n")
	}

	// Summary statistics
	criticalCount := 0
	warningCount := 0
	infoCount := 0
	for _, f := range results.Findings {
		switch f.Severity {
		case SeverityCritical:
			criticalCount++
		case SeverityWarning:
			warningCount++
		case SeverityInfo:
			infoCount++
		}
	}

	sb.WriteString(doctorHeaderStyle.Render("\n📊 Summary:") + " ")
	if criticalCount > 0 {
		sb.WriteString(failedStyle.Render(fmt.Sprintf("%d critical", criticalCount)) + "  ")
	}
	if warningCount > 0 {
		sb.WriteString(runningStyle.Render(fmt.Sprintf("%d warnings", warningCount)) + "  ")
	}
	if infoCount > 0 {
		sb.WriteString(fmt.Sprintf("%d info", infoCount))
	}
	sb.WriteString("\n")

	return sb.String()
}

// wrapText wraps text to the specified width, preserving indentation
func wrapText(text string, width, indent int) []string {
	if width <= indent {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	var currentLine strings.Builder
	lineWidth := 0
	indentStr := strings.Repeat(" ", indent)

	for i, word := range words {
		wordLen := len(word)

		// Add space before word (except first word on a line)
		spaceLen := 0
		if lineWidth > 0 {
			spaceLen = 1
		}

		// Check if adding this word would exceed width
		if lineWidth > 0 && lineWidth+spaceLen+wordLen > width-indent {
			// Finish current line and start new one
			lines = append(lines, indentStr+currentLine.String())
			currentLine.Reset()
			lineWidth = 0
		}

		// Add space if not first word on line
		if lineWidth > 0 {
			currentLine.WriteString(" ")
			lineWidth++
		}

		currentLine.WriteString(word)
		lineWidth += wordLen

		// Add last line
		if i == len(words)-1 {
			lines = append(lines, indentStr+currentLine.String())
		}
	}

	if len(lines) == 0 {
		lines = append(lines, indentStr)
	}

	return lines
}

// renderFinding formats a single finding with icon, color, and remediation
func renderFinding(f DoctorFinding, icon string, style lipgloss.Style) string {
	var sb strings.Builder

	// Icon + Message - wrap at reasonable width (80 chars)
	messageLines := wrapText(f.Message, 80, 4)
	for i, line := range messageLines {
		if i == 0 {
			// First line with icon
			sb.WriteString(fmt.Sprintf("  %s %s\n", style.Render(icon), strings.TrimSpace(line)))
		} else {
			// Continuation lines
			sb.WriteString(fmt.Sprintf("  %s   %s\n", style.Render(" "), strings.TrimSpace(line)))
		}
	}

	// Remediation (indented, dimmed, wrapped)
	if f.Remediation != "" {
		remediationLines := wrapText(f.Remediation, 80, 6)
		for i, line := range remediationLines {
			if i == 0 {
				sb.WriteString(doctorRemediationStyle.Render(fmt.Sprintf("→ %s", strings.TrimSpace(line))) + "\n")
			} else {
				sb.WriteString(doctorRemediationStyle.Render(fmt.Sprintf("  %s", strings.TrimSpace(line))) + "\n")
			}
		}
	}

	sb.WriteString("\n")
	return sb.String()
}
