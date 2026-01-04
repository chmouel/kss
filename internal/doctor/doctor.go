package doctor

import (
	"fmt"
	"strings"

	"github.com/chmouel/kss/internal/kube"
	"github.com/chmouel/kss/internal/model"
	"github.com/chmouel/kss/internal/util"
)

// AnalyzeContainerState checks the container status for known failure patterns
func AnalyzeContainerState(container model.ContainerStatus) []string {
	var issues []string

	checkTerminated := func(state *model.TerminatedState) {
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

	if container.State.Terminated != nil {
		checkTerminated(container.State.Terminated)
	} else if container.LastState != nil && container.LastState.Terminated != nil {
		checkTerminated(container.LastState.Terminated)
	}

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

	return issues
}

// AnalyzeLogs checks log content for common error patterns
func AnalyzeLogs(logs string) []string {
	var issues []string
	if logs == "" {
		return issues
	}

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
	return issues
}

// DiagnosePod analyzes the pod status and logs to find potential issues
func DiagnosePod(podObj model.Pod, kctl, podName string, args model.Args) {
	fmt.Println()
	fmt.Println(util.ColorText("  🩺 Doctor Analysis", "cyan"))
	fmt.Println(util.ColorText("  ──────────────────────────────────────────────────────────────", "dim"))

	foundIssue := false
	diagnoseContainer := func(container model.ContainerStatus, isInit bool) {
		issues := AnalyzeContainerState(container)

		if container.State.Terminated != nil || container.RestartCount > 0 || (container.State.Waiting != nil && container.State.Waiting.Reason == "CrashLoopBackOff") {
			// Get some logs to check for common patterns
			origMaxLines := args.MaxLines
			args.MaxLines = "100" // Get enough context for diagnosis

			// Try to get previous logs if restarting, otherwise current logs
			usePrevious := container.RestartCount > 0
			logs, err := kube.ShowLog(kctl, args, container.Name, podName, usePrevious)
			// If previous logs failed (maybe not available yet) or empty, try current
			if (err != nil || logs == "") && usePrevious {
				logs, err = kube.ShowLog(kctl, args, container.Name, podName, false)
			}
			args.MaxLines = origMaxLines

			if err == nil && logs != "" {
				issues = append(issues, AnalyzeLogs(logs)...)
			}
		}

		if len(issues) > 0 {
			foundIssue = true
			prefix := ""
			if isInit {
				prefix = "(Init) "
			}
			fmt.Printf("    %s %s\n", util.ColorText(fmt.Sprintf("%sContainer %s:", prefix, container.Name), "white_bold"), util.ColorText("Diagnosis", "yellow"))
			for _, issue := range issues {
				fmt.Printf("    • %s\n", issue)
			}
		} else if args.Doctor && container.State.Running != nil && container.RestartCount > 0 {
			// Explicit Doctor check for "Running but suspicious"
			foundIssue = true
			fmt.Printf("    %s %s\n", util.ColorText(fmt.Sprintf("Container %s:", container.Name), "white_bold"), util.ColorText("Stability Warning", "yellow"))
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
		fmt.Printf("    %s\n", util.ColorText("No obvious issues detected by heuristics.", "dim"))
	}
	fmt.Println()
}
