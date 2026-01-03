package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"

	"github.com/chmouel/kss/internal/ai"
	"github.com/chmouel/kss/internal/doctor"
	"github.com/chmouel/kss/internal/kube"
	"github.com/chmouel/kss/internal/model"
	"github.com/chmouel/kss/internal/util"
)

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

// overCnt displays container information in a formatted, color-coded manner
func overCnt(containers []model.ContainerStatus, kctl, pod string, args model.Args, podObj model.Pod) {
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
				age = util.FormatDuration(container.State.Running.StartedAt)
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
				age = util.FormatDuration(container.State.Terminated.FinishedAt)
			}
		case container.State.Waiting != nil:
			reason := container.State.Waiting.Reason
			if util.Contains(model.FailedContainers, reason) {
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
		statusLine := fmt.Sprintf("  %s %s", icon, util.ColorText(cname, "white_bold"))

		// Status with age and restarts
		statusParts := []string{util.ColorText(state, stateColor)}
		if age != "" {
			statusParts = append(statusParts, util.ColorText(fmt.Sprintf("(%s)", age), "dim"))
		}
		if container.RestartCount > 0 {
			statusParts = append(statusParts, util.ColorText(fmt.Sprintf("[%d restarts]", container.RestartCount), "yellow"))
		}

		// Align status to a fixed column (50 chars for name)
		namePadding := 50
		if len(cname) > namePadding {
			namePadding = len(cname) + 2
		}
		fmt.Printf("% -*s %s\n", namePadding+4, statusLine, strings.Join(statusParts, " "))

		// Image on separate line
		if container.Image != "" {
			image := container.Image
			maxImageLen := 70
			if len(image) > maxImageLen {
				image = image[:maxImageLen-3] + "..."
			}
			fmt.Printf("     %s %s\n", util.ColorText("Image:", "dim"), image)
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
						fmt.Printf("     %s %s\n", util.ColorText("Resources:", "dim"), strings.Join(resources, " "))
					}
					break
				}
			}
		}

		// Show ready status
		readyStatus := util.ColorText("No", "red")
		if container.Ready {
			readyStatus = util.ColorText("Yes", "green")
		}
		fmt.Printf("     %s %s\n", util.ColorText("Ready:", "dim"), readyStatus)

		if errmsg != "" {
			fmt.Println()
			fmt.Printf("    %s\n", util.ColorText("Error:", "red"))
			for _, line := range strings.Split(errmsg, "\n") {
				if strings.TrimSpace(line) != "" {
					fmt.Printf("      %s\n", util.ColorText(line, "dim"))
				}
			}
			fmt.Println()
		}

		if args.ShowLog {
			outputlog, err := kube.ShowLog(kctl, args, container.Name, pod, false)
			if err == nil && outputlog != "" {
				fmt.Println()
				fmt.Printf("    %s\n", util.ColorText(fmt.Sprintf("Logs for %s:", container.Name), "cyan"))
				fmt.Println(util.ColorText("    ──────────────────────────────────────────────────────────────", "dim"))
				for _, line := range strings.Split(outputlog, "\n") {
					fmt.Printf("    %s\n", line)
				}
				fmt.Println(util.ColorText("    ──────────────────────────────────────────────────────────────", "dim"))
				fmt.Println()
			}
		}
	}
}

// lensc counts the number of successful or failed containers
func lensc(containers []model.ContainerStatus) int {
	s := 0
	for _, c := range containers {
		if c.State.Waiting != nil && util.Contains(model.FailedContainers, c.State.Waiting.Reason) {
			s++
		}
		if c.State.Terminated != nil && c.State.Terminated.ExitCode == 0 {
			s++
		}
	}
	return s
}

// hasFailure checks if any container in the list has failed
func hasFailure(containers []model.ContainerStatus) bool {
	for _, c := range containers {
		if c.State.Waiting != nil && util.Contains(model.FailedContainers, c.State.Waiting.Reason) {
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

// printLabelsAnnotations displays pod labels or annotations in a formatted table
func printLabelsAnnotations(pod model.Pod, key, label string) {
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
	fmt.Println(util.ColorText(fmt.Sprintf("  %s", label), "cyan"))
	fmt.Println(util.ColorText("  ──────────────────────────────────────────────────────────────", "dim"))

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetStyle(table.StyleColoredBright)
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateColumns = false

	for k, v := range items {
		t.AppendRow(table.Row{
			fmt.Sprintf("    %s", util.ColorText(k, "white")),
			fmt.Sprintf(": %s", v),
		})
	}
	t.Render()
	fmt.Println()
}

// printPodPreview displays a compact preview optimized for fzf preview window
func printPodPreview(podObj model.Pod, pod string, args model.Args) {
	if podObj.Status.InitContainerStatuses == nil {
		podObj.Status.InitContainerStatuses = []model.ContainerStatus{}
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

	fmt.Printf("%s %s\n", util.ColorText("Pod:", "cyan"), util.ColorText(pod, "white_bold"))
	fmt.Printf("%s %s\n", util.ColorText("Status:", "cyan"), util.ColorText(text, colour))

	if podObj.Metadata.Namespace != "" {
		fmt.Printf("%s %s\n", util.ColorText("Namespace:", "cyan"), podObj.Metadata.Namespace)
	}
	if podObj.Status.Phase != "" {
		fmt.Printf("%s %s\n", util.ColorText("Phase:", "cyan"), podObj.Status.Phase)
	}
	if podObj.Status.StartTime != "" {
		fmt.Printf("%s %s\n", util.ColorText("Age:", "cyan"), util.FormatDuration(podObj.Status.StartTime))
	}
	fmt.Println()

	// Containers - compact format
	if len(podObj.Status.InitContainerStatuses) > 0 {
		fmt.Println(util.ColorText("Init Containers:", "cyan"))
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

	fmt.Println(util.ColorText("Containers:", "cyan"))
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
func printContainerPreview(container model.ContainerStatus) {
	state := ""
	var stateColor string
	age := ""

	switch {
	case container.State.Running != nil:
		state = "Running"
		stateColor = "blue"
		if container.State.Running.StartedAt != "" {
			age = util.FormatDuration(container.State.Running.StartedAt)
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
			age = util.FormatDuration(container.State.Terminated.FinishedAt)
		}
	case container.State.Waiting != nil:
		reason := container.State.Waiting.Reason
		if util.Contains(model.FailedContainers, reason) {
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
	statusInfo := util.ColorText(state, stateColor)
	if age != "" {
		statusInfo += " " + util.ColorText(fmt.Sprintf("(%s)", age), "dim")
	}
	if container.RestartCount > 0 {
		statusInfo += " " + util.ColorText(fmt.Sprintf("[%d]", container.RestartCount), "yellow")
	}

	fmt.Printf("  %s %s  %s\n", icon, util.ColorText(cname, "white_bold"), statusInfo)

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

// Event represents a Kubernetes event emitted for a pod.
type Event struct {
	LastTimestamp string `json:"lastTimestamp"`
	Type          string `json:"type"`
	Reason        string `json:"reason"`
	Message       string `json:"message"`
	Count         int32  `json:"count"`
}

// EventList wraps a list of pod events returned by `kubectl get events`.
type EventList struct {
	Items []Event `json:"items"`
}

// printEventsTimeline displays pod events in a relative timeline format
func printEventsTimeline(pod model.Pod, kctl, podName string) {
	fmt.Println()
	fmt.Println(util.ColorText("  Events", "cyan"))
	fmt.Println(util.ColorText("  ──────────────────────────────────────────────────────────────", "dim"))

	cmdStr := fmt.Sprintf("%s get events --field-selector involvedObject.name=%s --field-selector involvedObject.kind=Pod -o json", kctl, podName)
	output, err := exec.Command("sh", "-c", cmdStr).Output()
	if err != nil {
		fmt.Printf("    %s\n", util.ColorText("Error fetching events", "red"))
		return
	}

	var eventList EventList
	if err := json.Unmarshal(output, &eventList); err != nil {
		fmt.Printf("    %s: %v\n", util.ColorText("Error parsing events", "red"), err)
		return
	}

	if len(eventList.Items) == 0 {
		fmt.Printf("    %s\n", util.ColorText("No events found", "dim"))
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
		fmt.Printf("    %s\n", util.ColorText("No events with timestamps found", "dim"))
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
			util.ColorText(timeStr, "dim"),
			util.ColorText(event.Reason, reasonColor),
			util.ColorText(fmt.Sprintf("(%s)", event.Message), "dim"))
	}
}

// PrintPodInfo displays comprehensive pod information including containers, labels, and events
func PrintPodInfo(podObj model.Pod, kctl, pod string, args model.Args) {
	// Use compact preview if in preview mode
	if args.Preview {
		printPodPreview(podObj, pod, args)
		return
	}
	if podObj.Status.InitContainerStatuses == nil {
		podObj.Status.InitContainerStatuses = []model.ContainerStatus{}
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
		util.ColorText("╔", "cyan"),
		util.ColorText(boxTop, "cyan"),
		util.ColorText("╗", "cyan"))

	// Pod name
	podLine := fmt.Sprintf("%s %s", util.ColorText("Pod:", "cyan"), util.ColorText(pod, "white_bold"))
	fmt.Printf("%s %s %s\n",
		util.ColorText("║", "cyan"),
		util.PadToWidth(podLine, boxWidth-4),
		util.ColorText("║", "cyan"))

	// Status
	colour, text := getStatus(
		hasFailure(podObj.Status.InitContainerStatuses) || hasFailure(podObj.Status.ContainerStatuses),
		cntAllcontainers+cntAllicontainers,
		cntFailcontainers+cntFailicontainers,
	)
	statusLine := fmt.Sprintf("%s %s", util.ColorText("Status:", "cyan"), util.ColorText(text, colour))
	fmt.Printf("%s %s %s\n",
		util.ColorText("║", "cyan"),
		util.PadToWidth(statusLine, boxWidth-4),
		util.ColorText("║", "cyan"))

	// Add namespace and phase info
	if podObj.Metadata.Namespace != "" {
		nsLine := fmt.Sprintf("%s %s", util.ColorText("Namespace:", "cyan"), podObj.Metadata.Namespace)
		fmt.Printf("%s %s %s\n",
			util.ColorText("║", "cyan"),
			util.PadToWidth(nsLine, boxWidth-4),
			util.ColorText("║", "cyan"))
	}
	if podObj.Status.Phase != "" {
		phaseLine := fmt.Sprintf("%s %s", util.ColorText("Phase:", "cyan"), podObj.Status.Phase)
		fmt.Printf("%s %s %s\n",
			util.ColorText("║", "cyan"),
			util.PadToWidth(phaseLine, boxWidth-4),
			util.ColorText("║", "cyan"))
	}
	if podObj.Status.StartTime != "" {
		ageLine := fmt.Sprintf("%s %s", util.ColorText("Age:", "cyan"), util.FormatDuration(podObj.Status.StartTime))
		fmt.Printf("%s %s %s\n",
			util.ColorText("║", "cyan"),
			util.PadToWidth(ageLine, boxWidth-4),
			util.ColorText("║", "cyan"))
	}
	if podObj.Status.PodIP != "" {
		ipLine := fmt.Sprintf("%s %s", util.ColorText("Pod IP:", "cyan"), podObj.Status.PodIP)
		fmt.Printf("%s %s %s\n",
			util.ColorText("║", "cyan"),
			util.PadToWidth(ipLine, boxWidth-4),
			util.ColorText("║", "cyan"))
	}
	if podObj.Status.NodeName != "" {
		nodeLine := fmt.Sprintf("%s %s", util.ColorText("Node:", "cyan"), podObj.Status.NodeName)
		fmt.Printf("%s %s %s\n",
			util.ColorText("║", "cyan"),
			util.PadToWidth(nodeLine, boxWidth-4),
			util.ColorText("║", "cyan"))
	}
	if podObj.Status.QOSClass != "" {
		qosLine := fmt.Sprintf("%s %s", util.ColorText("QOS:", "cyan"), podObj.Status.QOSClass)
		fmt.Printf("%s %s %s\n",
			util.ColorText("║", "cyan"),
			util.PadToWidth(qosLine, boxWidth-4),
			util.ColorText("║", "cyan"))
	}
	if podObj.Spec.ServiceAccountName != "" {
		saLine := fmt.Sprintf("%s %s", util.ColorText("ServiceAccount:", "cyan"), podObj.Spec.ServiceAccountName)
		fmt.Printf("%s %s %s\n",
			util.ColorText("║", "cyan"),
			util.PadToWidth(saLine, boxWidth-4),
			util.ColorText("║", "cyan"))
	}
	if podObj.Spec.PriorityClassName != "" {
		priorityLine := fmt.Sprintf("%s %s", util.ColorText("Priority:", "cyan"), podObj.Spec.PriorityClassName)
		fmt.Printf("%s %s %s\n",
			util.ColorText("║", "cyan"),
			util.PadToWidth(priorityLine, boxWidth-4),
			util.ColorText("║", "cyan"))
	}

	// Show pod conditions
	if len(podObj.Status.Conditions) > 0 {
		fmt.Printf("%s %s %s\n",
			util.ColorText("║", "cyan"),
			util.PadToWidth(util.ColorText("Conditions:", "cyan"), boxWidth-4),
			util.ColorText("║", "cyan"))
		for _, condition := range podObj.Status.Conditions {
			statusColor := "red"
			switch condition.Status {
			case "True":
				statusColor = "green"
			case "False":
				statusColor = "yellow"
			}
			condLine := fmt.Sprintf("  %s: %s", condition.Type, util.ColorText(condition.Status, statusColor))
			if condition.Reason != "" {
				condLine += fmt.Sprintf(" (%s)", condition.Reason)
			}
			fmt.Printf("%s %s %s\n",
				util.ColorText("║", "cyan"),
				util.PadToWidth(condLine, boxWidth-4),
				util.ColorText("║", "cyan"))
		}
	}

	fmt.Printf("%s%s%s\n",
		util.ColorText("╚", "cyan"),
		util.ColorText(boxBottom, "cyan"),
		util.ColorText("╝", "cyan"))
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
			util.ColorText("Init Containers:", "cyan"),
			util.ColorText(s, colour),
			util.ColorText(fmt.Sprintf("(%d total)", cntAllicontainers), "dim"))
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
		util.ColorText("Containers:", "cyan"),
		util.ColorText(s, colour),
		util.ColorText(fmt.Sprintf("(%d total)", cntAllcontainers), "dim"))
	overCnt(podObj.Status.ContainerStatuses, kctl, pod, args, podObj)

	if args.Events {
		printEventsTimeline(podObj, kctl, pod)
	}

	if args.Doctor || hasFailure(podObj.Status.ContainerStatuses) || hasFailure(podObj.Status.InitContainerStatuses) {
		doctor.DiagnosePod(podObj, kctl, pod, args)
	}

	if args.Explain {
		ai.ExplainPod(podObj, kctl, pod, args)
	}
}
