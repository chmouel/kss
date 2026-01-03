// Package main implements KSS - Enhanced Kubernetes Pod Inspection.
// A beautiful and feature-rich tool to show the current status of pods
// and their associated containers and initContainers.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mattn/go-runewidth"
)

// failedContainers lists Kubernetes container states that indicate failure
var failedContainers = []string{
	"ImagePullBackOff",
	"CrashLoopBackOff",
	"ErrImagePull",
	"CreateContainerConfigError",
	"InvalidImageName",
}

// Args holds command-line arguments and options
type Args struct {
	Namespace     string   // Kubernetes namespace to use
	Restrict      string   // Regex pattern to restrict container display
	ShowLog       bool     // Whether to show container logs
	MaxLines      string   // Maximum number of log lines to show
	Labels        bool     // Whether to show pod labels
	Annotations   bool     // Whether to show pod annotations
	Events        bool     // Whether to show pod events
	Watch         bool     // Whether to enable watch mode
	WatchInterval int      // Watch refresh interval in seconds
	Preview       bool     // Whether to use compact preview mode (for fzf)
	Pods          []string // List of pod names to display
	Shell         bool     // Whether to open an interactive shell
	Doctor        bool     // Enable heuristic analysis
	Explain       bool     // Enable AI explanation
	Model         string   // AI Model to use
	Persona       string   // AI Persona to use
}

type ContainerState struct {
	Waiting    *WaitingState    `json:"waiting,omitempty"`
	Running    *RunningState    `json:"running,omitempty"`
	Terminated *TerminatedState `json:"terminated,omitempty"`
}

type WaitingState struct {
	Reason  string `json:"reason"`
	Message string `json:"message,omitempty"`
}

type RunningState struct {
	StartedAt string `json:"startedAt,omitempty"`
}

type TerminatedState struct {
	ExitCode   int    `json:"exitCode"`
	Message    string `json:"message,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
}

type ContainerStatus struct {
	Name         string          `json:"name"`
	State        ContainerState  `json:"state"`
	LastState    *ContainerState `json:"lastState,omitempty"`
	Ready        bool            `json:"ready"`
	RestartCount int             `json:"restartCount"`
	Image        string          `json:"image,omitempty"`
	ImageID      string          `json:"imageID,omitempty"`
}

type PodStatus struct {
	InitContainerStatuses []ContainerStatus `json:"initContainerStatuses,omitempty"`
	ContainerStatuses     []ContainerStatus `json:"containerStatuses,omitempty"`
	Phase                 string            `json:"phase"`
	StartTime             string            `json:"startTime,omitempty"`
	PodIP                 string            `json:"podIP,omitempty"`
	HostIP                string            `json:"hostIP,omitempty"`
	NodeName              string            `json:"nodeName,omitempty"`
	QOSClass              string            `json:"qosClass,omitempty"`
	Conditions            []PodCondition    `json:"conditions,omitempty"`
}

type PodCondition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
}

type PodMetadata struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	CreationTimestamp string            `json:"creationTimestamp,omitempty"`
}

type Pod struct {
	Metadata PodMetadata `json:"metadata"`
	Status   PodStatus   `json:"status"`
	Spec     PodSpec     `json:"spec,omitempty"`
}

type PodSpec struct {
	Containers         []ContainerSpec `json:"containers,omitempty"`
	InitContainers     []ContainerSpec `json:"initContainers,omitempty"`
	NodeName           string          `json:"nodeName,omitempty"`
	ServiceAccountName string          `json:"serviceAccountName,omitempty"`
	PriorityClassName  string          `json:"priorityClassName,omitempty"`
}

type ResourceList map[string]string

type Resources struct {
	Requests ResourceList `json:"requests,omitempty"`
	Limits   ResourceList `json:"limits,omitempty"`
}

type ContainerSpec struct {
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Resources Resources `json:"resources,omitempty"`
}

// colorText applies color formatting to text based on the color name
func colorText(text, colorName string) string {
	switch colorName {
	case "red":
		return color.New(color.FgRed, color.Bold).Sprint(text)
	case "yellow":
		return color.New(color.FgYellow, color.Bold).Sprint(text)
	case "blue":
		return color.New(color.FgBlue, color.Bold).Sprint(text)
	case "cyan":
		return color.New(color.FgCyan, color.Bold).Sprint(text)
	case "green":
		return color.New(color.FgGreen, color.Bold).Sprint(text)
	case "magenta":
		return color.New(color.FgMagenta, color.Bold).Sprint(text)
	case "white":
		return color.New(color.FgWhite).Sprint(text)
	case "white_bold":
		return color.New(color.FgWhite, color.Bold).Sprint(text)
	case "dim":
		return color.New(color.FgWhite, color.Faint).Sprint(text)
	default:
		return text
	}
}

// formatDuration converts a Kubernetes timestamp to a human-readable duration
// (e.g., "5m", "2h", "3d")
func formatDuration(timestamp string) string {
	if timestamp == "" {
		return "N/A"
	}
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return timestamp
	}
	duration := time.Since(t)
	switch {
	case duration < time.Minute:
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	case duration < time.Hour:
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%dh", int(duration.Hours()))
	default:
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
}

// getStateIcon returns a Unicode icon for the container state
func getStateIcon(state string) string {
	switch state {
	case "Running", "RUNNING":
		return "✓"
	case "FAIL", "FAILED":
		return "✗"
	case "SUCCESS":
		return "✓"
	case "Waiting":
		return "⏳"
	default:
		return "•"
	}
}

// showLog retrieves and returns container logs using kubectl
func showLog(kctl string, args Args, container, pod string, previous bool) (string, error) {
	cmdArgs := fmt.Sprintf("%s logs --tail=%s %s -c%s", kctl, args.MaxLines, pod, container)
	if previous {
		cmdArgs += " -p"
	}
	cmd := exec.Command("sh", "-c", cmdArgs)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not run '%s': %v", cmd.String(), err)
	}
	return strings.TrimSpace(string(output)), nil
}

// overCnt displays container information in a formatted, color-coded manner
func overCnt(containers []ContainerStatus, kctl, pod string, args Args, podObj Pod) {
	for _, container := range containers {
		errmsg := ""

		if args.Restrict != "" {
			matched, err := regexp.MatchString(args.Restrict, container.Name)
			if err != nil || !matched {
				continue
			}
		}

		state := ""
		var stateColor string
		age := ""

		switch {
		case container.State.Running != nil:
			state = "Running"
			stateColor = "blue"
			if container.State.Running.StartedAt != "" {
				age = formatDuration(container.State.Running.StartedAt)
			}
		case container.State.Terminated != nil:
			if container.State.Terminated.ExitCode != 0 {
				state = fmt.Sprintf("FAIL (exit: %d)", container.State.Terminated.ExitCode)
				stateColor = "red"
			} else {
				state = "SUCCESS"
				stateColor = "green"
			}
			if container.State.Terminated.FinishedAt != "" {
				age = formatDuration(container.State.Terminated.FinishedAt)
			}
		case container.State.Waiting != nil:
			reason := container.State.Waiting.Reason
			if contains(failedContainers, reason) {
				state = reason
				stateColor = "red"
				if container.LastState != nil && container.LastState.Terminated != nil {
					errmsg = container.LastState.Terminated.Message
				} else if container.State.Waiting != nil {
					errmsg = container.State.Waiting.Message
				}
			} else {
				state = fmt.Sprintf("Waiting: %s", reason)
				stateColor = "yellow"
			}
		}

		icon := getStateIcon(state)
		cname := container.Name

		// Simple, clean format: icon name status (age) [restarts]
		statusLine := fmt.Sprintf("  %s %s", icon, colorText(cname, "white_bold"))

		// Status with age and restarts
		statusParts := []string{colorText(state, stateColor)}
		if age != "" {
			statusParts = append(statusParts, colorText(fmt.Sprintf("(%s)", age), "dim"))
		}
		if container.RestartCount > 0 {
			statusParts = append(statusParts, colorText(fmt.Sprintf("[%d restarts]", container.RestartCount), "yellow"))
		}

		// Align status to a fixed column (50 chars for name)
		namePadding := 50
		if len(cname) > namePadding {
			namePadding = len(cname) + 2
		}
		fmt.Printf("%-*s %s\n", namePadding+4, statusLine, strings.Join(statusParts, " "))

		// Image on separate line
		if container.Image != "" {
			image := container.Image
			maxImageLen := 70
			if len(image) > maxImageLen {
				image = image[:maxImageLen-3] + "..."
			}
			fmt.Printf("     %s %s\n", colorText("Image:", "dim"), image)
		}

		// Show resources if available (from spec)
		if len(podObj.Spec.Containers) > 0 {
			for _, spec := range podObj.Spec.Containers {
				if spec.Name == container.Name {
					resources := []string{}
					if len(spec.Resources.Requests) > 0 {
						for k, v := range spec.Resources.Requests {
							resources = append(resources, fmt.Sprintf("%s:%s", k, v))
						}
					}
					if len(spec.Resources.Limits) > 0 {
						limits := []string{}
						for k, v := range spec.Resources.Limits {
							limits = append(limits, fmt.Sprintf("%s:%s", k, v))
						}
						if len(limits) > 0 {
							resources = append(resources, fmt.Sprintf("limits:[%s]", strings.Join(limits, ",")))
						}
					}
					if len(resources) > 0 {
						fmt.Printf("     %s %s\n", colorText("Resources:", "dim"), strings.Join(resources, " "))
					}
					break
				}
			}
		}

		// Show ready status
		readyStatus := colorText("No", "red")
		if container.Ready {
			readyStatus = colorText("Yes", "green")
		}
		fmt.Printf("     %s %s\n", colorText("Ready:", "dim"), readyStatus)

		if errmsg != "" {
			fmt.Println()
			fmt.Printf("    %s\n", colorText("Error:", "red"))
			for _, line := range strings.Split(errmsg, "\n") {
				if strings.TrimSpace(line) != "" {
					fmt.Printf("      %s\n", colorText(line, "dim"))
				}
			}
			fmt.Println()
		}

		if args.ShowLog {
			outputlog, err := showLog(kctl, args, container.Name, pod, false)
			if err == nil && outputlog != "" {
				fmt.Println()
				fmt.Printf("    %s\n", colorText(fmt.Sprintf("Logs for %s:", container.Name), "cyan"))
				fmt.Println(colorText("    ──────────────────────────────────────────────────────────────", "dim"))
				for _, line := range strings.Split(outputlog, "\n") {
					fmt.Printf("    %s\n", line)
				}
				fmt.Println(colorText("    ──────────────────────────────────────────────────────────────", "dim"))
				fmt.Println()
			}
		}
	}
}

// lensc counts the number of successful or failed containers
func lensc(containers []ContainerStatus) int {
	s := 0
	for _, c := range containers {
		if c.State.Waiting != nil && contains(failedContainers, c.State.Waiting.Reason) {
			s++
		}
		if c.State.Terminated != nil && c.State.Terminated.ExitCode == 0 {
			s++
		}
	}
	return s
}

// hasFailure checks if any container in the list has failed
func hasFailure(containers []ContainerStatus) bool {
	for _, c := range containers {
		if c.State.Waiting != nil && contains(failedContainers, c.State.Waiting.Reason) {
			return true
		}
		if c.State.Terminated != nil && c.State.Terminated.ExitCode != 0 {
			return true
		}
	}
	return false
}

// getStatus determines the overall pod status color and text based on container states
func getStatus(hasFailures bool, allc, allf int) (string, string) {
	if hasFailures {
		return "red", "❌ FAIL"
	} else if allc != allf {
		return "blue", "🔄 RUNNING"
	}
	return "green", "✅ SUCCESS"
}

// which finds the full path of an executable in PATH
func which(program string) string {
	if filepath.IsAbs(program) {
		if _, err := os.Stat(program); err == nil {
			return program
		}
		return ""
	}

	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		fullPath := filepath.Join(dir, program)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}
	}
	return ""
}

// printLabelsAnnotations displays pod labels or annotations in a formatted table
func printLabelsAnnotations(pod Pod, key, label string) {
	var items map[string]string
	if key == "labels" {
		items = pod.Metadata.Labels
	} else {
		items = pod.Metadata.Annotations
	}

	if len(items) == 0 {
		return
	}

	fmt.Println()
	fmt.Println(colorText(fmt.Sprintf("  %s", label), "cyan"))
	fmt.Println(colorText("  ──────────────────────────────────────────────────────────────", "dim"))

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = false

	for k, v := range items {
		t.AppendRow(table.Row{
			fmt.Sprintf("    %s", colorText(k, "white")),
			fmt.Sprintf(": %s", v),
		})
	}
	t.Render()
	fmt.Println()
}

// contains checks if a string slice contains a specific item
func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}

// stripANSI removes ANSI color codes from a string to get actual display width
func stripANSI(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return re.ReplaceAllString(s, "")
}

// getDisplayWidth calculates the actual display width of a string,
// accounting for ANSI codes and wide characters (emojis)
func getDisplayWidth(s string) int {
	// Remove ANSI codes first
	clean := stripANSI(s)
	// Use runewidth to get proper display width (handles emojis correctly)
	return runewidth.StringWidth(clean)
}

// padToWidth pads a string to a specific display width, accounting for ANSI codes and emojis
func padToWidth(s string, width int) string {
	actualWidth := getDisplayWidth(s)
	if actualWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-actualWidth)
}

// printPodPreview displays a compact preview optimized for fzf preview window
func printPodPreview(podObj Pod, pod string, args Args) {
	if podObj.Status.InitContainerStatuses == nil {
		podObj.Status.InitContainerStatuses = []ContainerStatus{}
	}

	cntFailicontainers := lensc(podObj.Status.InitContainerStatuses)
	cntAllicontainers := len(podObj.Status.InitContainerStatuses)
	cntFailcontainers := lensc(podObj.Status.ContainerStatuses)
	cntAllcontainers := len(podObj.Status.ContainerStatuses)

	// Compact header
	colour, text := getStatus(
		hasFailure(podObj.Status.InitContainerStatuses) || hasFailure(podObj.Status.ContainerStatuses),
		cntAllcontainers+cntAllicontainers,
		cntFailcontainers+cntFailicontainers,
	)

	fmt.Printf("%s %s\n", colorText("Pod:", "cyan"), colorText(pod, "white_bold"))
	fmt.Printf("%s %s\n", colorText("Status:", "cyan"), colorText(text, colour))

	if podObj.Metadata.Namespace != "" {
		fmt.Printf("%s %s\n", colorText("Namespace:", "cyan"), podObj.Metadata.Namespace)
	}
	if podObj.Status.Phase != "" {
		fmt.Printf("%s %s\n", colorText("Phase:", "cyan"), podObj.Status.Phase)
	}
	if podObj.Status.StartTime != "" {
		fmt.Printf("%s %s\n", colorText("Age:", "cyan"), formatDuration(podObj.Status.StartTime))
	}
	fmt.Println()

	// Containers - compact format
	if len(podObj.Status.InitContainerStatuses) > 0 {
		fmt.Println(colorText("Init Containers:", "cyan"))
		for _, container := range podObj.Status.InitContainerStatuses {
			if args.Restrict != "" {
				matched, err := regexp.MatchString(args.Restrict, container.Name)
				if err != nil || !matched {
					continue
				}
			}
			printContainerPreview(container)
		}
		fmt.Println()
	}

	fmt.Println(colorText("Containers:", "cyan"))
	for _, container := range podObj.Status.ContainerStatuses {
		if args.Restrict != "" {
			matched, err := regexp.MatchString(args.Restrict, container.Name)
			if err != nil || !matched {
				continue
			}
		}
		printContainerPreview(container)
	}
}

// printContainerPreview displays container info in compact format for fzf preview
func printContainerPreview(container ContainerStatus) {
	state := ""
	var stateColor string
	age := ""

	switch {
	case container.State.Running != nil:
		state = "Running"
		stateColor = "blue"
		if container.State.Running.StartedAt != "" {
			age = formatDuration(container.State.Running.StartedAt)
		}
	case container.State.Terminated != nil:
		if container.State.Terminated.ExitCode != 0 {
			state = fmt.Sprintf("Failed(%d)", container.State.Terminated.ExitCode)
			stateColor = "red"
		} else {
			state = "Succeeded"
			stateColor = "green"
		}
		if container.State.Terminated.FinishedAt != "" {
			age = formatDuration(container.State.Terminated.FinishedAt)
		}
	case container.State.Waiting != nil:
		reason := container.State.Waiting.Reason
		if contains(failedContainers, reason) {
			state = reason
			stateColor = "red"
		} else {
			state = fmt.Sprintf("Waiting:%s", reason)
			stateColor = "yellow"
		}
	}

	icon := getStateIcon(state)
	cname := container.Name

	// Very simple format for preview - one line, no fancy alignment
	statusInfo := colorText(state, stateColor)
	if age != "" {
		statusInfo += " " + colorText(fmt.Sprintf("(%s)", age), "dim")
	}
	if container.RestartCount > 0 {
		statusInfo += " " + colorText(fmt.Sprintf("[%d]", container.RestartCount), "yellow")
	}

	fmt.Printf("  %s %s  %s\n", icon, colorText(cname, "white_bold"), statusInfo)

	// Image on separate line, truncated
	if container.Image != "" {
		image := container.Image
		maxImageLen := 50
		if len(image) > maxImageLen {
			image = image[:maxImageLen-3] + "..."
		}
		fmt.Printf("    %s\n", image)
	}
}

type Event struct {
	LastTimestamp string `json:"lastTimestamp"`
	Type          string `json:"type"`
	Reason        string `json:"reason"`
	Message       string `json:"message"`
	Count         int32  `json:"count"`
}

type EventList struct {
	Items []Event `json:"items"`
}

// printEventsTimeline displays pod events in a relative timeline format
func printEventsTimeline(pod Pod, kctl, podName string) {
	fmt.Println()
	fmt.Println(colorText("  Events", "cyan"))
	fmt.Println(colorText("  ──────────────────────────────────────────────────────────────", "dim"))

	cmdStr := fmt.Sprintf("%s get events --field-selector involvedObject.name=%s --field-selector involvedObject.kind=Pod -o json", kctl, podName)
	output, err := exec.Command("sh", "-c", cmdStr).Output()
	if err != nil {
		fmt.Printf("    %s\n", colorText("Error fetching events", "red"))
		return
	}

	var eventList EventList
	if err := json.Unmarshal(output, &eventList); err != nil {
		fmt.Printf("    %s: %v\n", colorText("Error parsing events", "red"), err)
		return
	}

	if len(eventList.Items) == 0 {
		fmt.Printf("    %s\n", colorText("No events found", "dim"))
		return
	}

	// Filter out events without timestamp
	var validEvents []Event
	for _, e := range eventList.Items {
		if e.LastTimestamp != "" {
			validEvents = append(validEvents, e)
		}
	}
	eventList.Items = validEvents

	if len(eventList.Items) == 0 {
		fmt.Printf("    %s\n", colorText("No events with timestamps found", "dim"))
		return
	}

	// Sort events by timestamp
	slices.SortFunc(eventList.Items, func(a, b Event) int {
		return strings.Compare(a.LastTimestamp, b.LastTimestamp)
	})

	podCreationTime, err := time.Parse(time.RFC3339, pod.Metadata.CreationTimestamp)
	// If pod creation time failed, use the first event time
	if err != nil {
		t, err2 := time.Parse(time.RFC3339, eventList.Items[0].LastTimestamp)
		if err2 == nil {
			podCreationTime = t
		}
	}

	for _, event := range eventList.Items {
		eventTime, err := time.Parse(time.RFC3339, event.LastTimestamp)
		if err != nil {
			continue
		}

		diff := max(eventTime.Sub(podCreationTime), 0)

		// Format diff
		var timeStr string
		totalSeconds := int(diff.Seconds())
		minutes := totalSeconds / 60
		seconds := totalSeconds % 60
		hours := minutes / 60
		minutes %= minutes

		if hours > 0 {
			timeStr = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
		} else {
			timeStr = fmt.Sprintf("%02d:%02d", minutes, seconds)
		}

		// Color based on Type and Reason
		reasonColor := "white"
		if event.Type == "Warning" {
			reasonColor = "yellow"
			if strings.Contains(strings.ToLower(event.Reason), "failed") || strings.Contains(strings.ToLower(event.Reason), "backoff") {
				reasonColor = "red"
			}
		}

		// Print: Time Reason (Message)
		// Truncate message if too long? For now let it wrap or just show as is.
		fmt.Printf("    %s %s %s\n",
			colorText(timeStr, "dim"),
			colorText(event.Reason, reasonColor),
			colorText(fmt.Sprintf("(%s)", event.Message), "dim"))
	}
}

// printPodInfo displays comprehensive pod information including containers, labels, and events
func printPodInfo(podObj Pod, kctl, pod string, args Args) {
	// Use compact preview if in preview mode
	if args.Preview {
		printPodPreview(podObj, pod, args)
		return
	}
	if podObj.Status.InitContainerStatuses == nil {
		podObj.Status.InitContainerStatuses = []ContainerStatus{}
	}

	cntFailicontainers := lensc(podObj.Status.InitContainerStatuses)
	cntAllicontainers := len(podObj.Status.InitContainerStatuses)
	cntFailcontainers := lensc(podObj.Status.ContainerStatuses)
	cntAllcontainers := len(podObj.Status.ContainerStatuses)

	// Print header with box - fixed width of 76 characters
	boxWidth := 76
	boxTop := strings.Repeat("═", boxWidth-2)
	boxBottom := strings.Repeat("═", boxWidth-2)

	fmt.Println()
	fmt.Printf("%s%s%s\n",
		colorText("╔", "cyan"),
		colorText(boxTop, "cyan"),
		colorText("╗", "cyan"))

	// Pod name
	podLine := fmt.Sprintf("%s %s", colorText("Pod:", "cyan"), colorText(pod, "white_bold"))
	fmt.Printf("%s %s %s\n",
		colorText("║", "cyan"),
		padToWidth(podLine, boxWidth-4),
		colorText("║", "cyan"))

	// Status
	colour, text := getStatus(
		hasFailure(podObj.Status.InitContainerStatuses) || hasFailure(podObj.Status.ContainerStatuses),
		cntAllcontainers+cntAllicontainers,
		cntFailcontainers+cntFailicontainers,
	)
	statusLine := fmt.Sprintf("%s %s", colorText("Status:", "cyan"), colorText(text, colour))
	fmt.Printf("%s %s %s\n",
		colorText("║", "cyan"),
		padToWidth(statusLine, boxWidth-4),
		colorText("║", "cyan"))

	// Add namespace and phase info
	if podObj.Metadata.Namespace != "" {
		nsLine := fmt.Sprintf("%s %s", colorText("Namespace:", "cyan"), podObj.Metadata.Namespace)
		fmt.Printf("%s %s %s\n",
			colorText("║", "cyan"),
			padToWidth(nsLine, boxWidth-4),
			colorText("║", "cyan"))
	}
	if podObj.Status.Phase != "" {
		phaseLine := fmt.Sprintf("%s %s", colorText("Phase:", "cyan"), podObj.Status.Phase)
		fmt.Printf("%s %s %s\n",
			colorText("║", "cyan"),
			padToWidth(phaseLine, boxWidth-4),
			colorText("║", "cyan"))
	}
	if podObj.Status.StartTime != "" {
		ageLine := fmt.Sprintf("%s %s", colorText("Age:", "cyan"), formatDuration(podObj.Status.StartTime))
		fmt.Printf("%s %s %s\n",
			colorText("║", "cyan"),
			padToWidth(ageLine, boxWidth-4),
			colorText("║", "cyan"))
	}
	if podObj.Status.PodIP != "" {
		ipLine := fmt.Sprintf("%s %s", colorText("Pod IP:", "cyan"), podObj.Status.PodIP)
		fmt.Printf("%s %s %s\n",
			colorText("║", "cyan"),
			padToWidth(ipLine, boxWidth-4),
			colorText("║", "cyan"))
	}
	if podObj.Status.NodeName != "" {
		nodeLine := fmt.Sprintf("%s %s", colorText("Node:", "cyan"), podObj.Status.NodeName)
		fmt.Printf("%s %s %s\n",
			colorText("║", "cyan"),
			padToWidth(nodeLine, boxWidth-4),
			colorText("║", "cyan"))
	}
	if podObj.Status.QOSClass != "" {
		qosLine := fmt.Sprintf("%s %s", colorText("QOS:", "cyan"), podObj.Status.QOSClass)
		fmt.Printf("%s %s %s\n",
			colorText("║", "cyan"),
			padToWidth(qosLine, boxWidth-4),
			colorText("║", "cyan"))
	}
	if podObj.Spec.ServiceAccountName != "" {
		saLine := fmt.Sprintf("%s %s", colorText("ServiceAccount:", "cyan"), podObj.Spec.ServiceAccountName)
		fmt.Printf("%s %s %s\n",
			colorText("║", "cyan"),
			padToWidth(saLine, boxWidth-4),
			colorText("║", "cyan"))
	}
	if podObj.Spec.PriorityClassName != "" {
		priorityLine := fmt.Sprintf("%s %s", colorText("Priority:", "cyan"), podObj.Spec.PriorityClassName)
		fmt.Printf("%s %s %s\n",
			colorText("║", "cyan"),
			padToWidth(priorityLine, boxWidth-4),
			colorText("║", "cyan"))
	}

	// Show pod conditions
	if len(podObj.Status.Conditions) > 0 {
		fmt.Printf("%s %s %s\n",
			colorText("║", "cyan"),
			padToWidth(colorText("Conditions:", "cyan"), boxWidth-4),
			colorText("║", "cyan"))
		for _, condition := range podObj.Status.Conditions {
			statusColor := "red"
			switch condition.Status {
			case "True":
				statusColor = "green"
			case "False":
				statusColor = "yellow"
			}
			condLine := fmt.Sprintf("  %s: %s", condition.Type, colorText(condition.Status, statusColor))
			if condition.Reason != "" {
				condLine += fmt.Sprintf(" (%s)", condition.Reason)
			}
			fmt.Printf("%s %s %s\n",
				colorText("║", "cyan"),
				padToWidth(condLine, boxWidth-4),
				colorText("║", "cyan"))
		}
	}

	fmt.Printf("%s%s%s\n",
		colorText("╚", "cyan"),
		colorText(boxBottom, "cyan"),
		colorText("╝", "cyan"))
	fmt.Println()

	if args.Labels {
		printLabelsAnnotations(podObj, "labels", "Labels")
	}
	if args.Annotations {
		printLabelsAnnotations(podObj, "annotations", "Annotations")
	}

	if len(podObj.Status.InitContainerStatuses) > 0 {
		colour, _ := getStatus(
			hasFailure(podObj.Status.InitContainerStatuses),
			cntAllicontainers,
			cntFailicontainers,
		)
		s := fmt.Sprintf("%d/%d", cntFailicontainers, cntAllicontainers)
		fmt.Printf("%s %s %s\n",
			colorText("Init Containers:", "cyan"),
			colorText(s, colour),
			colorText(fmt.Sprintf("(%d total)", cntAllicontainers), "dim"))
		overCnt(podObj.Status.InitContainerStatuses, kctl, pod, args, podObj)
		fmt.Println()
	}

	colour, text = getStatus(
		hasFailure(podObj.Status.ContainerStatuses),
		cntAllcontainers,
		cntFailcontainers,
	)
	var s string
	if text == "🔄 RUNNING" {
		s = fmt.Sprintf("%d", cntAllcontainers)
	} else {
		s = fmt.Sprintf("%d/%d", cntFailcontainers, cntAllcontainers)
	}
	fmt.Printf("%s %s %s\n",
		colorText("Containers:", "cyan"),
		colorText(s, colour),
		colorText(fmt.Sprintf("(%d total)", cntAllcontainers), "dim"))
	overCnt(podObj.Status.ContainerStatuses, kctl, pod, args, podObj)

	if args.Events {
		printEventsTimeline(podObj, kctl, pod)
	}

	if args.Doctor || hasFailure(podObj.Status.ContainerStatuses) || hasFailure(podObj.Status.InitContainerStatuses) {
		diagnosePod(podObj, kctl, pod, args)
	}

	if args.Explain {
		explainPod(podObj, kctl, pod, args)
	}
}

// explainPod gathers context and asks AI for an explanation
func explainPod(podObj Pod, kctl, podName string, args Args) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println()
		fmt.Printf("    %s %s\n", colorText("🧠 AI Explanation:", "cyan"), colorText("GEMINI_API_KEY not set. Cannot provide AI analysis.", "red"))
		return
	}

	personaDisplay := map[string]string{
		"butler":   "🤵 Alfred",
		"sergeant": "🪖 The Drill Sergeant",
		"hacker":   "⌨️ The Cyberpunk Hacker",
		"pirate":   "🏴‍☠️ The Pirate",
		"genz":     "✨ The Gen Z Influencer",
	}
	displayName := personaDisplay[args.Persona]
	if displayName == "" {
		displayName = args.Persona
	}

	fmt.Println()
	fmt.Printf("    %s %s", colorText("🧠 AI Explanation:", "cyan"), colorText(fmt.Sprintf("%s is investigating...", displayName), "dim"))

	// 1. Gather Context
	podJSON, _ := json.MarshalIndent(podObj, "", "  ")

	// 2. Gather Events
	cmdStr := fmt.Sprintf("%s get events --field-selector involvedObject.name=%s --field-selector involvedObject.kind=Pod -o json", kctl, podName)
	eventsOutput, _ := exec.Command("sh", "-c", cmdStr).Output()

	// 3. Gather Logs (from failing containers)
	var logs strings.Builder
	collectLogs := func(containers []ContainerStatus) {
		for _, c := range containers {
			if hasFailure([]ContainerStatus{c}) || c.RestartCount > 0 {
				origMaxLines := args.MaxLines
				args.MaxLines = "100" // Increased context for AI

				// Try previous logs first if there are restarts
				if c.RestartCount > 0 {
					l, err := showLog(kctl, args, c.Name, podName, true)
					if err == nil && l != "" {
						logs.WriteString(fmt.Sprintf("\n--- Previous Logs for container %s (Crashed Instance) ---\n%s\n", c.Name, l))
					}
				}

				// Always get current logs too
				l, err := showLog(kctl, args, c.Name, podName, false)
				args.MaxLines = origMaxLines
				if err == nil && l != "" {
					logs.WriteString(fmt.Sprintf("\n--- Current Logs for container %s ---\n%s\n", c.Name, l))
				}
			}
		}
	}
	collectLogs(podObj.Status.InitContainerStatuses)
	collectLogs(podObj.Status.ContainerStatuses)

	// 4. Construct Persona-specific instructions
	personaInstructions := ""
	switch args.Persona {
	case "sergeant":
		personaInstructions = "Speak in the persona of a stern Drill Sergeant. Be demanding and direct, but keep it professional. Use caps for emphasis."
	case "hacker":
		personaInstructions = "Speak in the persona of an edgy cyberpunk hacker. Use technical slang like 'glitch', 'patching the ghost', 'zero-day', and 'mainframe'. Be cool and efficient."
	case "pirate":
		personaInstructions = "Speak in the persona of a rough pirate. Use 'Arrgh', 'matey', and nautical terms. Be gritty but helpful."
	case "genz":
		personaInstructions = "Speak in the persona of a Gen Z influencer. Use 'no cap', 'it's giving', 'shook', and 'vibe check'. Use plenty of emojis."
	default:
		personaInstructions = "Speak in the persona of Alfred, a refined British butler. Be polite, formal, but efficient. Address the user as 'sir'. Never use the word 'master'."
	}

	// 5. Construct Prompt
	prompt := fmt.Sprintf(`%s
Your task is to diagnose a pod failure.

Context:
- Pod Name: %s
- Namespace: %s
- Phase: %s

Pod Status (JSON):
%s

Events:
%s

Logs:
%s

Instructions:
1. Adhere strictly to your persona.
2. Analyze the logs and events to identify the root cause.
3. If "Previous Logs" are present, prioritize them.
4. Provide a VERY CONCISE explanation of the failure (1-2 sentences max).
5. Provide a specific *kubectl* command or YAML fix to resolve it.
6. Use clear Markdown formatting. Use bolding and code blocks effectively.
7. Do not waste words. Get straight to the point.`,
		personaInstructions, podName, podObj.Metadata.Namespace, podObj.Status.Phase,
		string(podJSON), string(eventsOutput), logs.String())

	// 6. Call Gemini API
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", args.Model, apiKey)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
		"safetySettings": []map[string]interface{}{
			{"category": "HARM_CATEGORY_HARASSMENT", "threshold": "BLOCK_NONE"},
			{"category": "HARM_CATEGORY_HATE_SPEECH", "threshold": "BLOCK_NONE"},
			{"category": "HARM_CATEGORY_SEXUALLY_EXPLICIT", "threshold": "BLOCK_NONE"},
			{"category": "HARM_CATEGORY_DANGEROUS_CONTENT", "threshold": "BLOCK_NONE"},
		},
	}
	jsonData, _ := json.Marshal(reqBody)

	// #nosec G107
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("\r    %s %s\n", colorText("🧠 AI Explanation:", "cyan"), colorText("Error calling AI API: "+err.Error(), "red"))
		return
	}
	defer func() { _ = resp.Body.Close() }()

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		fmt.Printf("\r    %s %s\n", colorText("🧠 AI Explanation:", "cyan"), colorText("Error decoding AI response.", "red"))
		return
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		// Dump the raw body for debugging if we get an empty candidate list (often means a safety block or 400 error that wasn't caught)
		// We re-read the body which is a bit tricky with http.Response, but for now let's just print a generic error with a tip.
		fmt.Printf("\r    %s %s\n", colorText("🧠 AI Explanation:", "cyan"), colorText("AI returned no candidates. This might be due to Safety Settings or an invalid prompt.", "yellow"))
		return
	}

	explanation := geminiResp.Candidates[0].Content.Parts[0].Text

	// Debug: Print raw explanation length
	// fmt.Printf("\n[Debug] Received explanation of length: %d\n", len(explanation))

	// Clear the "Consulting..." line
	fmt.Print("\r")

	// Render Markdown using glamour
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	out, err := r.Render(explanation)
	if err != nil {
		fmt.Println("Error rendering markdown:", err)
		fmt.Println(explanation) // Fallback
	} else {
		if out == "" {
			fmt.Println("Markdown renderer returned empty string. Fallback to raw text:")
			fmt.Println(explanation)
		} else {
			fmt.Print(out)
		}
	}
	fmt.Println()
}

// clearScreen clears the terminal screen (used in watch mode)
func diagnosePod(podObj Pod, kctl, podName string, args Args) {
	fmt.Println()
	fmt.Println(colorText("  🩺 Doctor Analysis", "cyan"))
	fmt.Println(colorText("  ──────────────────────────────────────────────────────────────", "dim"))

	foundIssue := false
	diagnoseContainer := func(container ContainerStatus, isInit bool) {
		issues := []string{}

		checkTerminated := func(state *TerminatedState) {
			if state == nil {
				return
			}
			exitCode := state.ExitCode
			switch exitCode {
			case 137:
				issues = append(issues, "Likely OOMKilled (Out of Memory). Your container exceeded its memory limits.")
			case 1, 2:
				issues = append(issues, fmt.Sprintf("Application crashed (Exit Code %d). This is usually an internal application error.", exitCode))
			case 127:
				issues = append(issues, "Command not found. Check your container's entrypoint or command.")
			}
		}

		// 1. Check Exit Codes (Current or Last)
		if container.State.Terminated != nil {
			checkTerminated(container.State.Terminated)
		} else if container.LastState != nil && container.LastState.Terminated != nil {
			checkTerminated(container.LastState.Terminated)
		}

		// 2. Check Waiting Reasons
		if container.State.Waiting != nil {
			reason := container.State.Waiting.Reason
			switch reason {
			case "ImagePullBackOff", "ErrImagePull":
				issues = append(issues, "Failed to pull image. Check if the image name/tag is correct and if the registry requires authentication.")
			case "CrashLoopBackOff":
				issues = append(issues, "Container is crashing repeatedly. Check application logs for errors during startup.")
			case "CreateContainerConfigError":
				issues = append(issues, "Configuration error. Likely a missing ConfigMap or Secret.")
			}
		}

		// 3. Log Analysis (if we can get them)
		if container.State.Terminated != nil || container.RestartCount > 0 || (container.State.Waiting != nil && container.State.Waiting.Reason == "CrashLoopBackOff") {
			// Get some logs to check for common patterns
			origMaxLines := args.MaxLines
			args.MaxLines = "100" // Get enough context for diagnosis

			// Try to get previous logs if restarting, otherwise current logs
			usePrevious := container.RestartCount > 0
			logs, err := showLog(kctl, args, container.Name, podName, usePrevious)
			// If previous logs failed (maybe not available yet) or empty, try current
			if (err != nil || logs == "") && usePrevious {
				logs, err = showLog(kctl, args, container.Name, podName, false)
			}
			args.MaxLines = origMaxLines

			if err == nil && logs != "" {
				lowerLogs := strings.ToLower(logs)
				if strings.Contains(lowerLogs, "connection refused") || strings.Contains(lowerLogs, "dial tcp") {
					issues = append(issues, "Network error detected (Connection Refused). Check if dependent services are reachable.")
				}
				if strings.Contains(lowerLogs, "timeout") || strings.Contains(lowerLogs, "deadline exceeded") {
					issues = append(issues, "Timeout detected. A service or resource might be slow or unreachable.")
				}
				if strings.Contains(lowerLogs, "permission denied") || strings.Contains(lowerLogs, "forbidden") {
					issues = append(issues, "Permission denied. Check the Pod's SecurityContext or file system permissions.")
				}
				if strings.Contains(lowerLogs, "not found") && (strings.Contains(lowerLogs, "config") || strings.Contains(lowerLogs, "file")) {
					issues = append(issues, "Missing file or configuration. Check your volume mounts and ConfigMaps.")
				}
			}
		}

		if len(issues) > 0 {
			foundIssue = true
			prefix := ""
			if isInit {
				prefix = "(Init) "
			}
			fmt.Printf("    %s %s\n", colorText(fmt.Sprintf("%sContainer %s:", prefix, container.Name), "white_bold"), colorText("Diagnosis", "yellow"))
			for _, issue := range issues {
				fmt.Printf("    • %s\n", issue)
			}
		} else if args.Doctor && container.State.Running != nil && container.RestartCount > 0 {
			// Explicit Doctor check for "Running but suspicious"
			foundIssue = true
			fmt.Printf("    %s %s\n", colorText(fmt.Sprintf("Container %s:", container.Name), "white_bold"), colorText("Stability Warning", "yellow"))
			fmt.Printf("    • Container is running but has restarted %d times. Check logs for intermittent crashes.\n", container.RestartCount)
		}
	}

	for _, c := range podObj.Status.InitContainerStatuses {
		diagnoseContainer(c, true)
	}
	for _, c := range podObj.Status.ContainerStatuses {
		diagnoseContainer(c, false)
	}

	if !foundIssue {
		fmt.Printf("    %s\n", colorText("No obvious issues detected by heuristics.", "dim"))
	}
	fmt.Println()
}

// clearScreen clears the terminal screen (used in watch mode)
func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func kubectlArgs(args Args) []string {
	if args.Namespace == "" {
		return nil
	}
	return []string{"-n", args.Namespace}
}

func fetchPod(kubectlArgs []string, pod string) (Pod, error) {
	cmdArgs := append(append([]string{}, kubectlArgs...), "get", "pod", pod, "-ojson")
	cmd := exec.Command("kubectl", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return Pod{}, fmt.Errorf("could not fetch pod %s: %s", pod, strings.TrimSpace(string(output)))
	}

	var podObj Pod
	if err := json.Unmarshal(output, &podObj); err != nil {
		return Pod{}, fmt.Errorf("could not parse pod data for %s: %w", pod, err)
	}

	return podObj, nil
}

type containerInfo struct {
	Name    string
	State   string
	Running bool
}

func containerStateLabel(status ContainerStatus) (string, bool) {
	switch {
	case status.State.Running != nil:
		return "running", true
	case status.State.Waiting != nil:
		reason := status.State.Waiting.Reason
		if reason == "" {
			reason = "waiting"
		}
		return "waiting: " + reason, false
	case status.State.Terminated != nil:
		reason := status.State.Terminated.Message
		if reason == "" {
			reason = fmt.Sprintf("exit %d", status.State.Terminated.ExitCode)
		}
		return "terminated: " + reason, false
	default:
		return "unknown", false
	}
}

func containerInfoForPod(kubectlArgs []string, pod string) ([]containerInfo, error) {
	podObj, err := fetchPod(kubectlArgs, pod)
	if err != nil {
		return nil, err
	}

	statusByName := make(map[string]containerInfo, len(podObj.Status.ContainerStatuses))
	for _, status := range podObj.Status.ContainerStatuses {
		state, running := containerStateLabel(status)
		statusByName[status.Name] = containerInfo{
			Name:    status.Name,
			State:   state,
			Running: running,
		}
	}

	infos := make([]containerInfo, 0, len(podObj.Spec.Containers))
	for _, container := range podObj.Spec.Containers {
		if info, ok := statusByName[container.Name]; ok {
			infos = append(infos, info)
			continue
		}
		infos = append(infos, containerInfo{Name: container.Name, State: "unknown"})
	}

	return infos, nil
}

func filterContainersByRestrict(containers []string, restrict string) ([]string, error) {
	if restrict == "" {
		return containers, nil
	}

	re, err := regexp.Compile(restrict)
	if err != nil {
		return nil, fmt.Errorf("invalid restrict regex: %w", err)
	}

	var filtered []string
	for _, container := range containers {
		if re.MatchString(container) {
			filtered = append(filtered, container)
		}
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("no containers matched restrict %q", restrict)
	}

	return filtered, nil
}

func selectContainerWithFzf(containers []string) (string, error) {
	cmd := exec.Command("fzf", "-0", "-1", "--prompt", "container> ")
	cmd.Stdin = strings.NewReader(strings.Join(containers, "\n"))
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("fzf selection failed: %w", err)
	}

	selection := strings.TrimSpace(output.String())
	if selection == "" {
		return "", fmt.Errorf("no container selected")
	}

	return selection, nil
}

func selectContainerByPrompt(containers []string) (string, error) {
	fmt.Println("Select a container:")
	for i, name := range containers {
		fmt.Printf("  %d) %s\n", i+1, name)
	}
	fmt.Print("Enter number: ")

	reader := bufio.NewReader(os.Stdin)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read selection: %w", err)
		}
		input = strings.TrimSpace(input)
		if input == "" {
			fmt.Print("Enter number: ")
			continue
		}

		index, err := strconv.Atoi(input)
		if err != nil || index < 1 || index > len(containers) {
			fmt.Printf("Please enter a number between 1 and %d: ", len(containers))
			continue
		}

		return containers[index-1], nil
	}
}

func selectContainer(containers []string, restrict string) (string, error) {
	filtered, err := filterContainersByRestrict(containers, restrict)
	if err != nil {
		return "", err
	}

	if len(filtered) == 1 {
		return filtered[0], nil
	}

	if which("fzf") != "" {
		return selectContainerWithFzf(filtered)
	}

	return selectContainerByPrompt(filtered)
}

var shellCandidates = []string{
	"/bin/bash",
	"/usr/bin/bash",
	"/bin/sh",
	"/usr/bin/sh",
	"/bin/ash",
	"/busybox/sh",
	"/bin/dash",
}

func isBashShell(shell string) bool {
	return filepath.Base(shell) == "bash"
}

func probeShell(kubectlArgs []string, pod, container, shell string) bool {
	cmdArgs := append(append([]string{}, kubectlArgs...), "exec", pod, "-c", container, "--", shell, "-c", "exit 0")
	cmd := exec.Command("kubectl", cmdArgs...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func pickShell(kubectlArgs []string, pod, container string) (string, error) {
	for _, shell := range shellCandidates {
		if probeShell(kubectlArgs, pod, container, shell) {
			return shell, nil
		}
	}
	return "", fmt.Errorf("no usable shell found in pod %s (tried: %s)", pod, strings.Join(shellCandidates, ", "))
}

func runShell(args Args, kubectlArgs []string, pod string) error {
	containers, err := containerInfoForPod(kubectlArgs, pod)
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		return fmt.Errorf("pod %s has no containers to shell into", pod)
	}

	allNames := make([]string, 0, len(containers))
	runningNames := make([]string, 0, len(containers))
	stateByName := make(map[string]string, len(containers))
	runningByName := make(map[string]bool, len(containers))
	for _, container := range containers {
		allNames = append(allNames, container.Name)
		stateByName[container.Name] = container.State
		runningByName[container.Name] = container.Running
		if container.Running {
			runningNames = append(runningNames, container.Name)
		}
	}

	candidates := allNames
	if args.Restrict == "" {
		if len(runningNames) == 0 {
			var details []string
			for _, name := range allNames {
				details = append(details, fmt.Sprintf("%s (%s)", name, stateByName[name]))
			}
			return fmt.Errorf("no running containers in pod %s (%s)", pod, strings.Join(details, ", "))
		}
		candidates = runningNames
	}

	container, err := selectContainer(candidates, args.Restrict)
	if err != nil {
		return err
	}
	if !runningByName[container] {
		state := stateByName[container]
		if state == "" {
			state = "unknown"
		}
		return fmt.Errorf("container %s is not running (%s)", container, state)
	}

	shell, err := pickShell(kubectlArgs, pod, container)
	if err != nil {
		return err
	}
	if !isBashShell(shell) {
		fmt.Printf("Using %s (bash unavailable)\n", shell)
	}

	cmdArgs := append(append([]string{}, kubectlArgs...), "exec", "-it", pod, "-c", container, "--", shell)
	cmd := exec.Command("kubectl", cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func main() {
	args := parseArgs()

	kctl := "kubectl"
	if args.Namespace != "" {
		kctl = fmt.Sprintf("kubectl -n %s", args.Namespace)
	}
	kubectlBaseArgs := kubectlArgs(args)

	myself := which("kss")
	preview := fmt.Sprintf("%s describe {}", kctl)
	if myself != "" {
		// Use preview mode for fzf - compact and clean
		preview = myself + " --preview"
		if args.Namespace != "" {
			preview += fmt.Sprintf(" -n %s", args.Namespace)
		}
		if args.Annotations {
			preview += " -A"
		}
		if args.Labels {
			preview += " -L"
		}
		preview += " {}"
	}

	if args.Shell {
		pod := ""
		if len(args.Pods) > 0 {
			pod = args.Pods[0]
		} else {
			queryArgs := ""
			if len(args.Pods) > 0 {
				queryArgs = fmt.Sprintf("-q '%s'", strings.Join(args.Pods, " "))
			}
			runcmd := fmt.Sprintf("%s get pods -o name|fzf -0 -n 1 -m -1 %s --preview-window 'right:60%%:wrap' --preview='%s'", kctl, queryArgs, preview)
			cmd := exec.Command("sh", "-c", runcmd)
			output, err := cmd.Output()
			if err != nil {
				fmt.Println("No pods is no news which is arguably no worries.")
				os.Exit(1)
			}
			podNames := strings.TrimSpace(string(output))
			if podNames == "" {
				fmt.Println("No pods is no news which is arguably no worries.")
				os.Exit(1)
			}
			pods := strings.Split(strings.ReplaceAll(podNames, "pod/", ""), "\n")
			pod = strings.TrimSpace(pods[0])
		}

		if pod == "" {
			fmt.Println("No pods is no news which is arguably no worries.")
			os.Exit(1)
		}

		if err := runShell(args, kubectlBaseArgs, pod); err != nil {
			fmt.Printf("Unable to start shell in pod %s: %v\n", pod, err)
			os.Exit(1)
		}
		return
	}

	queryArgs := ""
	if len(args.Pods) > 0 {
		queryArgs = fmt.Sprintf("-q '%s'", strings.Join(args.Pods, " "))
	}

	runcmd := fmt.Sprintf("%s get pods -o name|fzf -0 -n 1 -m -1 %s --preview-window 'right:60%%:wrap' --preview='%s'", kctl, queryArgs, preview)

	cmd := exec.Command("sh", "-c", runcmd)
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("No pods is no news which is arguably no worries.")
		os.Exit(1)
	}

	podNames := strings.TrimSpace(string(output))
	if podNames == "" {
		fmt.Println("No pods is no news which is arguably no worries.")
		os.Exit(1)
	}

	pods := strings.Split(strings.ReplaceAll(podNames, "pod/", ""), "\n")

	// Handle watch mode
	if args.Watch {
		interval := time.Duration(args.WatchInterval) * time.Second
		if interval == 0 {
			interval = 2 * time.Second
		}

		// Handle Ctrl+C gracefully
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		for {
			select {
			case <-sigChan:
				fmt.Println("\nExiting watch mode...")
				os.Exit(0)
			default:
				clearScreen()
				fmt.Printf("%s Watching pods (refresh every %v, press Ctrl+C to exit)\n\n",
					colorText("⏱", "cyan"), interval)

				for _, pod := range pods {
					pod = strings.TrimSpace(pod)
					if pod == "" {
						continue
					}

					cmdline := fmt.Sprintf("%s get pod %s -ojson", kctl, pod)
					cmd := exec.Command("sh", "-c", cmdline)
					output, err := cmd.CombinedOutput()
					if err != nil {
						fmt.Printf("Error running '%s'\n", cmdline)
						continue
					}

					var podObj Pod
					if err := json.Unmarshal(output, &podObj); err != nil {
						fmt.Printf("Error parsing JSON: %v\n", err)
						continue
					}

					printPodInfo(podObj, kctl, pod, args)
				}

				time.Sleep(interval)
			}
		}
	}

	// Normal mode
	for _, pod := range pods {
		pod = strings.TrimSpace(pod)
		if pod == "" {
			continue
		}

		cmdline := fmt.Sprintf("%s get pod %s -ojson", kctl, pod)
		cmd := exec.Command("sh", "-c", cmdline)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("There was some problem running '%s'\n", cmdline)
			os.Exit(1)
		}

		var podObj Pod
		if err := json.Unmarshal(output, &podObj); err != nil {
			fmt.Printf("Error parsing JSON: %v\n", err)
			os.Exit(1)
		}

		printPodInfo(podObj, kctl, pod, args)

		if len(pods) > 1 {
			fmt.Println()
		}
	}
}

// parseArgs parses command-line arguments and returns an Args struct
func parseArgs() Args {
	args := Args{
		MaxLines:      "-1",
		WatchInterval: 2,
		Model:         "gemini-2.5-flash-lite",
	}

	personas := []string{"butler", "sergeant", "hacker", "pirate", "genz"}

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "-n", "--namespace":
			if i+1 < len(os.Args) {
				args.Namespace = os.Args[i+1]
				i++
			}
		case "-r", "--restrict":
			if i+1 < len(os.Args) {
				args.Restrict = os.Args[i+1]
				i++
			}
		case "-l", "--showlog":
			args.ShowLog = true
		case "--maxlines":
			if i+1 < len(os.Args) {
				args.MaxLines = os.Args[i+1]
				i++
			}
		case "-L", "--labels":
			args.Labels = true
		case "-A", "--annotations":
			args.Annotations = true
		case "-E", "--events":
			args.Events = true
		case "-w", "--watch":
			args.Watch = true
		case "--watch-interval":
			if i+1 < len(os.Args) {
				var interval int
				_, _ = fmt.Sscanf(os.Args[i+1], "%d", &interval)
				args.WatchInterval = interval
				i++
			}
		case "--preview":
			args.Preview = true
		case "-d", "--doctor":
			args.Doctor = true
		case "-s", "--shell":
			args.Shell = true
		case "--explain":
			args.Explain = true
		case "--model":
			if i+1 < len(os.Args) {
				args.Model = os.Args[i+1]
				i++
			}
		case "-p", "--persona":
			if i+1 < len(os.Args) {
				args.Persona = os.Args[i+1]
				i++
			}
		case "-h", "--help":
			printHelp()
			os.Exit(0)
		default:
			if !strings.HasPrefix(arg, "-") {
				args.Pods = append(args.Pods, arg)
			}
		}
	}

	// Pick a random persona if not specified
	if args.Persona == "" {
		// #nosec G404
		args.Persona = personas[rand.Intn(len(personas))]
	}

	return args
}

// printHelp displays the help message with usage examples
func printHelp() {
	helpText := `
╔════════════════════════════════════════════════════════════════════════╗
║                    KSS - Kubernetes Pod Status                         ║
║                 Enhanced Kubernetes Pod Inspection                     ║
╚════════════════════════════════════════════════════════════════════════╝

Usage: kss [OPTIONS] [POD...]

Options:
  -n, --namespace NAMESPACE    Use namespace
  -r, --restrict REGEXP        Restrict to show only those containers (regexp)
  -l, --showlog                Show logs of containers
  --maxlines INT               Maximum line when showing logs (default: -1)
  -L, --labels                 Show labels
  -A, --annotations            Show annotations
  -E, --events                 Show events
  -w, --watch                  Watch mode (auto-refresh)
  --watch-interval SECONDS     Watch refresh interval in seconds (default: 2)
  -d, --doctor                 Enable heuristic analysis (Doctor mode)
  -s, --shell                  Open an interactive shell in the selected pod
  --explain                    Enable AI explanation for pod failures
  --model MODEL                AI Model to use (default: gemini-2.5-flash-lite)
  -p, --persona PERSONA        AI Persona: butler, sergeant, hacker, pirate, genz (default: random)
  -h, --help                   Display this help message
`
	fmt.Println(helpText)
}
