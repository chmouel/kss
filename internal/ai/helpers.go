package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/chmouel/kss/internal/kube"
	"github.com/chmouel/kss/internal/model"
	"github.com/chmouel/kss/internal/util"
)

const aiMaxLogLines = "100"

var (
	errNoCandidates = errors.New("AI returned no candidates. This might be due to Safety Settings or an invalid prompt")
	runner          util.Runner = &util.RealRunner{}
)

func printAIStatus(displayName string) {
	fmt.Println()
	fmt.Printf("    %s %s", util.ColorText("🧠 AI Explanation:", "cyan"), util.ColorText(fmt.Sprintf("%s is investigating...", displayName), "dim"))
}

func printAIError(message, color string) {
	fmt.Printf("\r    %s %s\n", util.ColorText("🧠 AI Explanation:", "cyan"), util.ColorText(message, color))
}

func aiErrorColor(err error) string {
	if errors.Is(err, errNoCandidates) {
		return "yellow"
	}
	return "red"
}

func fetchEventsJSON(kctl, kind, name string) string {
	cmdStr := fmt.Sprintf("%s get events --field-selector involvedObject.name=%s --field-selector involvedObject.kind=%s -o json", kctl, name, kind)
	output, _ := runner.Run("sh", "-c", cmdStr)
	return string(output)
}

func collectPodLogsForAI(kctl string, args model.Args, podObj model.Pod, podName string) string {
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
				args.MaxLines = aiMaxLogLines

				if c.RestartCount > 0 {
					l, err := kube.ShowLog(kctl, args, c.Name, podName, true)
					if err == nil && l != "" {
						logs.WriteString(fmt.Sprintf("\n--- Previous Logs for container %s (Crashed Instance) ---\n%s\n", c.Name, l))
					}
				}

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
	return logs.String()
}

func callGemini(apiKey, modelName, prompt string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", modelName, apiKey)

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
		return "", fmt.Errorf("error calling AI API: %w", err)
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
		return "", fmt.Errorf("error decoding AI response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", errNoCandidates
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

func renderExplanation(explanation string) {
	fmt.Print("\r")

	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	out, err := r.Render(explanation)
	if err != nil {
		fmt.Println("Error rendering markdown:", err)
		fmt.Println(explanation)
		return
	}
	if out == "" {
		fmt.Println("Markdown renderer returned empty string. Fallback to raw text:")
		fmt.Println(explanation)
		return
	}
	fmt.Print(out)
	fmt.Println()
}
