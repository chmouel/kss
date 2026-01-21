package ai

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/chmouel/kss/internal/kube"
	"github.com/chmouel/kss/internal/model"
	"github.com/chmouel/kss/internal/tekton"
)

// ExplainPipelineRun gathers context and asks AI for an explanation.
func ExplainPipelineRun(pr tekton.PipelineRun, taskRuns []tekton.TaskRun, kctl string, kubectlArgs []string, args model.Args) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println()
		printAIError("GEMINI_API_KEY not set. Cannot provide AI analysis.", "red")
		return
	}

	displayName := personaDisplayName(args.Persona)
	printAIStatus(displayName)

	pipelineName := "inline"
	if pr.Spec.PipelineRef != nil && pr.Spec.PipelineRef.Name != "" {
		pipelineName = pr.Spec.PipelineRef.Name
	}

	prJSON, _ := json.MarshalIndent(pr, "", "  ")
	taskRunsJSON, _ := json.MarshalIndent(taskRuns, "", "  ")
	prEvents := fetchEventsJSON(kctl, "PipelineRun", pr.Metadata.Name)
	taskEvents := taskRunEvents(kctl, taskRuns)
	taskSummary := taskRunSummary(taskRuns)
	logs := taskRunLogs(taskRuns, kctl, kubectlArgs, args)
	statusLabel, statusReason, statusMessage := pipelineRunStatus(pr)

	prompt := fmt.Sprintf(`%s
Your task is to diagnose a Tekton PipelineRun failure.

Context:
- PipelineRun Name: %s
- Namespace: %s
- Pipeline: %s
- Status: %s
- Reason: %s
- Message: %s

PipelineRun JSON:
%s

TaskRuns JSON:
%s

TaskRun Summary:
%s

PipelineRun Events:
%s

TaskRun Events:
%s

TaskRun Logs:
%s

Instructions:
1. Adhere strictly to your persona.
2. Identify the root cause across TaskRuns and PipelineRun conditions.
3. Prioritize any "Previous Logs" if present.
4. Provide a VERY CONCISE explanation of the failure (1-2 sentences max).
5. Provide a specific *kubectl* command or YAML fix to resolve it.
6. Use clear Markdown formatting. Use bolding and code blocks effectively.
7. Do not waste words. Get straight to the point.`,
		personaInstructions(args.Persona),
		pr.Metadata.Name,
		pr.Metadata.Namespace,
		pipelineName,
		statusLabel,
		statusReason,
		statusMessage,
		string(prJSON),
		string(taskRunsJSON),
		taskSummary,
		prEvents,
		taskEvents,
		logs,
	)

	explanation, err := callGemini(apiKey, args.Model, prompt)
	if err != nil {
		printAIError(err.Error(), aiErrorColor(err))
		return
	}

	renderExplanation(explanation, args.Persona)
}

func taskRunSummary(taskRuns []tekton.TaskRun) string {
	if len(taskRuns) == 0 {
		return "No TaskRuns found."
	}

	lines := make([]string, 0, len(taskRuns))
	for i := range taskRuns {
		tr := &taskRuns[i]
		label, _, reason, message := tekton.StatusLabel(tr.Status.Conditions)
		displayName := tekton.TaskRunDisplayName(*tr)
		line := fmt.Sprintf("- %s (%s): %s", displayName, tr.Metadata.Name, label)
		if reason != "" {
			line += fmt.Sprintf(" (%s)", reason)
		}
		if message != "" {
			line += fmt.Sprintf(" - %s", message)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func taskRunEvents(kctl string, taskRuns []tekton.TaskRun) string {
	if len(taskRuns) == 0 {
		return "No TaskRuns found."
	}

	var events strings.Builder
	for i := range taskRuns {
		tr := &taskRuns[i]
		output := strings.TrimSpace(fetchEventsJSON(kctl, "TaskRun", tr.Metadata.Name))
		if output == "" {
			continue
		}
		events.WriteString(fmt.Sprintf("\n--- Events for TaskRun %s ---\n%s\n", tr.Metadata.Name, output))
	}
	if events.Len() == 0 {
		return "No TaskRun events found."
	}
	return events.String()
}

func taskRunLogs(taskRuns []tekton.TaskRun, kctl string, kubectlArgs []string, args model.Args) string {
	if len(taskRuns) == 0 {
		return "No TaskRuns found."
	}

	var logs strings.Builder
	for i := range taskRuns {
		tr := &taskRuns[i]
		label, _, reason, _ := tekton.StatusLabel(tr.Status.Conditions)
		if label == "Succeeded" {
			continue
		}

		displayName := tekton.TaskRunDisplayName(*tr)
		header := fmt.Sprintf("\n--- TaskRun %s (%s)", displayName, tr.Metadata.Name)
		if reason != "" {
			header += fmt.Sprintf(" - %s", reason)
		}
		header += " ---\n"
		logs.WriteString(header)

		podName, err := tekton.PodNameForTaskRun(kubectlArgs, *tr)
		if err != nil {
			logs.WriteString(fmt.Sprintf("Could not resolve pod: %v\n", err))
			continue
		}

		podObj, err := kube.FetchPod(kubectlArgs, podName)
		if err != nil {
			logs.WriteString(fmt.Sprintf("Could not fetch pod %s: %v\n", podName, err))
			continue
		}

		podLogs := strings.TrimSpace(collectPodLogsForAI(kctl, args, podObj, podName))
		if podLogs == "" {
			logs.WriteString("No failing container logs found.\n")
			continue
		}
		logs.WriteString(podLogs)
		logs.WriteString("\n")
	}

	if logs.Len() == 0 {
		return "No failed TaskRuns found, skipping logs."
	}
	return logs.String()
}

func pipelineRunStatus(pr tekton.PipelineRun) (label, reason, message string) {
	label, _, reason, message = tekton.StatusLabel(pr.Status.Conditions)
	return label, reason, message
}
