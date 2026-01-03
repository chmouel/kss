// Package main implements KSS - Kubernetes pod status on steroid.
// A beautiful and feature-rich tool to show the current status of pods
// and their associated containers and initContainers.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mattn/go-runewidth"
)

var (
	// failedContainers lists Kubernetes container states that indicate failure
	failedContainers = []string{
		"ImagePullBackOff",
		"CrashLoopBackOff",
		"ErrImagePull",
		"CreateContainerConfigError",
		"InvalidImageName",
	}
)

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
	Conditions            []PodCondition   `json:"conditions,omitempty"`
}

type PodCondition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
}

type PodMetadata struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	CreationTimestamp string      `json:"creationTimestamp,omitempty"`
}

type Pod struct {
	Metadata PodMetadata `json:"metadata"`
	Status   PodStatus   `json:"status"`
	Spec     PodSpec     `json:"spec,omitempty"`
}

type PodSpec struct {
	Containers      []ContainerSpec `json:"containers,omitempty"`
	InitContainers  []ContainerSpec `json:"initContainers,omitempty"`
	NodeName        string          `json:"nodeName,omitempty"`
	ServiceAccountName string        `json:"serviceAccountName,omitempty"`
	PriorityClassName string        `json:"priorityClassName,omitempty"`
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
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	return fmt.Sprintf("%dd", int(duration.Hours()/24))
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
func showLog(kctl string, args Args, container, pod string) (string, error) {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s logs --tail=%s %s -c%s", kctl, args.MaxLines, pod, container))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not run '%s': %v", cmd.String(), err)
	}
	return strings.TrimSpace(string(output)), nil
}

func printContainerTable(containers []ContainerStatus, kctl string, pod string, args Args, title string) {
	if len(containers) == 0 {
		return
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Container", "Status", "Image", "Restarts", "Age", "Ready"})
	t.SetStyle(table.StyleColoredBright)
	t.Style().Options.SeparateRows = true

	for _, container := range containers {
		if args.Restrict != "" {
			matched, err := regexp.MatchString(args.Restrict, container.Name)
			if err != nil || !matched {
				continue
			}
		}

		state := ""
		var stateColor string
		age := "N/A"

		if container.State.Running != nil {
			state = "Running"
			stateColor = "blue"
			if container.State.Running.StartedAt != "" {
				age = formatDuration(container.State.Running.StartedAt)
			}
		} else if container.State.Terminated != nil {
			if container.State.Terminated.ExitCode != 0 {
				state = fmt.Sprintf("Failed (exit: %d)", container.State.Terminated.ExitCode)
				stateColor = "red"
			} else {
				state = "Succeeded"
				stateColor = "green"
			}
			if container.State.Terminated.FinishedAt != "" {
				age = formatDuration(container.State.Terminated.FinishedAt)
			}
		} else if container.State.Waiting != nil {
			reason := container.State.Waiting.Reason
			if contains(failedContainers, reason) {
				state = reason
				stateColor = "red"
			} else {
				state = fmt.Sprintf("Waiting: %s", reason)
				stateColor = "yellow"
			}
		}

		icon := getStateIcon(state)
		statusText := fmt.Sprintf("%s %s", icon, state)
		
		image := container.Image
		if image == "" {
			image = colorText("N/A", "dim")
		}

		ready := "No"
		if container.Ready {
			ready = colorText("Yes", "green")
		} else {
			ready = colorText("No", "red")
		}

		restarts := fmt.Sprintf("%d", container.RestartCount)
		if container.RestartCount > 0 {
			restarts = colorText(restarts, "yellow")
		}

		t.AppendRow(table.Row{
			colorText(container.Name, "white_bold"),
			colorText(statusText, stateColor),
			image,
			restarts,
			age,
			ready,
		})
	}

	if t.Length() > 0 {
		fmt.Println()
		fmt.Println(colorText(fmt.Sprintf("  %s", title), "cyan"))
		t.Render()
		fmt.Println()
	}
}

// overCnt displays container information in a formatted, color-coded manner
func overCnt(containers []ContainerStatus, kctl string, pod string, args Args, podObj Pod) {
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

		if container.State.Running != nil {
			state = "Running"
			stateColor = "blue"
			if container.State.Running.StartedAt != "" {
				age = formatDuration(container.State.Running.StartedAt)
			}
		} else if container.State.Terminated != nil {
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
		} else if container.State.Waiting != nil {
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
			outputlog, err := showLog(kctl, args, container.Name, pod)
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
func printLabelsAnnotations(pod Pod, key string, label string) {
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
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
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
func printPodPreview(podObj Pod, kctl string, pod string, args Args) {
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

	if container.State.Running != nil {
		state = "Running"
		stateColor = "blue"
		if container.State.Running.StartedAt != "" {
			age = formatDuration(container.State.Running.StartedAt)
		}
	} else if container.State.Terminated != nil {
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
	} else if container.State.Waiting != nil {
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

// printPodInfo displays comprehensive pod information including containers, labels, and events
func printPodInfo(podObj Pod, kctl string, pod string, args Args) {
	// Use compact preview if in preview mode
	if args.Preview {
		printPodPreview(podObj, kctl, pod, args)
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
			if condition.Status == "True" {
				statusColor = "green"
			} else if condition.Status == "False" {
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
		fmt.Println()
		fmt.Println(colorText("  Events", "cyan"))
		fmt.Println(colorText("  ──────────────────────────────────────────────────────────────", "dim"))
		cmd := fmt.Sprintf("kubectl get events --sort-by='.lastTimestamp' --field-selector involvedObject.name=%s --field-selector involvedObject.kind=Pod", pod)
		if args.Namespace != "" {
			cmd = fmt.Sprintf("kubectl -n %s get events --sort-by='.lastTimestamp' --field-selector involvedObject.name=%s --field-selector involvedObject.kind=Pod", args.Namespace, pod)
		}
		output, _ := exec.Command("sh", "-c", cmd).Output()
		outputStr := strings.TrimSpace(string(output))
		lines := strings.Split(outputStr, "\n")
		if len(lines) > 1 {
			fmt.Printf("    %s\n", colorText(lines[0], "white_bold"))
			for _, line := range lines[1:] {
				if strings.TrimSpace(line) != "" {
					fmt.Printf("    %s\n", line)
				}
			}
		} else {
			fmt.Printf("    %s\n", colorText("No events found", "dim"))
		}
		fmt.Println()
	}
}

// clearScreen clears the terminal screen (used in watch mode)
func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func main() {
	args := parseArgs()

	kctl := "kubectl"
	if args.Namespace != "" {
		kctl = fmt.Sprintf("kubectl -n %s", args.Namespace)
	}

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
		MaxLines:     "-1",
		WatchInterval: 2,
	}

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
				fmt.Sscanf(os.Args[i+1], "%d", &interval)
				args.WatchInterval = interval
				i++
			}
		case "--preview":
			args.Preview = true
		case "-h", "--help":
			printHelp()
			os.Exit(0)
		default:
			if !strings.HasPrefix(arg, "-") {
				args.Pods = append(args.Pods, arg)
			}
		}
	}

	return args
}

// printHelp displays the help message with usage examples
func printHelp() {
	helpText := `
╔════════════════════════════════════════════════════════════════════════╗
║                    KSS - Kubernetes Pod Status                         ║
║                         on Steroid 💉                                  ║
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
  -h, --help                   Display this help message

Examples:
  kss                           # Interactive pod selection with fzf
  kss my-pod                    # Show status for specific pod
  kss -n production             # Select pod from production namespace
  kss -l --maxlines 50          # Show last 50 lines of logs
  kss -r "app" -l               # Show logs only for containers matching "app"
  kss -w --watch-interval 5     # Watch mode, refresh every 5 seconds
`
	fmt.Println(helpText)
}
