package ai

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/chmouel/kss/internal/model"
)

// ExplainPod gathers context and asks AI for an explanation.
func ExplainPod(podObj model.Pod, kctl, podName string, args model.Args) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println()
		printAIError("GEMINI_API_KEY not set. Cannot provide AI analysis.", "red")
		return
	}

	displayName := personaDisplayName(args.Persona)
	printAIStatus(displayName)

	// 1. Gather Context

	podJSON, _ := json.MarshalIndent(podObj, "", "  ")

	// 2. Gather Events
	eventsOutput := fetchEventsJSON(kctl, "Pod", podName)

	// 3. Gather Logs (from failing containers)
	logs := collectPodLogsForAI(kctl, args, podObj, podName)

	// 4. Construct Persona-specific instructions
	personaInstructions := personaInstructions(args.Persona)

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
		string(podJSON), eventsOutput, logs)

	// 6. Call Gemini API
	explanation, err := callGemini(apiKey, args.Model, prompt)
	if err != nil {
		printAIError(err.Error(), aiErrorColor(err))
		return
	}

	renderExplanation(explanation)
}
