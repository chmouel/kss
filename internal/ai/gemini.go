package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/chmouel/kss/internal/kube"
	"github.com/chmouel/kss/internal/model"
	"github.com/chmouel/kss/internal/util"
)

// ExplainPod gathers context and asks AI for an explanation
func ExplainPod(podObj model.Pod, kctl, podName string, args model.Args) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println()
		fmt.Printf("    %s %s\n", util.ColorText("🧠 AI Explanation:", "cyan"), util.ColorText("GEMINI_API_KEY not set. Cannot provide AI analysis.", "red"))
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
	fmt.Printf("    %s %s", util.ColorText("🧠 AI Explanation:", "cyan"), util.ColorText(fmt.Sprintf("%s is investigating...", displayName), "dim"))

	// 1. Gather Context

	podJSON, _ := json.MarshalIndent(podObj, "", "  ")

	// 2. Gather Events
	cmdStr := fmt.Sprintf("%s get events --field-selector involvedObject.name=%s --field-selector involvedObject.kind=Pod -o json", kctl, podName)
	eventsOutput, _ := exec.Command("sh", "-c", cmdStr).Output()

	// 3. Gather Logs (from failing containers)
	var logs strings.Builder
	collectLogs := func(containers []model.ContainerStatus) {
		for _, c := range containers {
			hasFailure := false
			if c.State.Waiting != nil && util.Contains(model.FailedContainers, c.State.Waiting.Reason) {
				hasFailure = true
			}
			if c.State.Terminated != nil && c.State.Terminated.ExitCode != 0 {
				hasFailure = true
			}

			if hasFailure || c.RestartCount > 0 {
				origMaxLines := args.MaxLines
				args.MaxLines = "100" // Increased context for AI

				// Try previous logs first if there are restarts
				if c.RestartCount > 0 {
					l, err := kube.ShowLog(kctl, args, c.Name, podName, true)
					if err == nil && l != "" {
						logs.WriteString(fmt.Sprintf("\n--- Previous Logs for container %s (Crashed Instance) ---\n%s\n", c.Name, l))
					}
				}

				// Always get current logs too
				l, err := kube.ShowLog(kctl, args, c.Name, podName, false)
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
		fmt.Printf("\r    %s %s\n", util.ColorText("🧠 AI Explanation:", "cyan"), util.ColorText("Error calling AI API: "+err.Error(), "red"))
		return
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				}
			}
		}
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		fmt.Printf("\r    %s %s\n", util.ColorText("🧠 AI Explanation:", "cyan"), util.ColorText("Error decoding AI response.", "red"))
		return
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		fmt.Printf("\r    %s %s\n", util.ColorText("🧠 AI Explanation:", "cyan"), util.ColorText("AI returned no candidates. This might be due to Safety Settings or an invalid prompt.", "yellow"))
		return
	}

	explanation := geminiResp.Candidates[0].Content.Parts[0].Text

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
