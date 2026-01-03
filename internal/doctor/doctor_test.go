package doctor

import (
	"strings"
	"testing"

	"github.com/chmouel/kss/internal/model"
)

func TestAnalyzeContainerState(t *testing.T) {
	cases := []struct {
		name      string
		container model.ContainerStatus
		check     func([]string) bool
	}{
		{
			name: "terminated with OOMKilled",
			container: model.ContainerStatus{
				State: model.ContainerState{
					Terminated: &model.TerminatedState{
						ExitCode: 137,
					},
				},
			},
			check: func(issues []string) bool {
				return len(issues) == 1 && strings.Contains(issues[0], "OOMKilled")
			},
		},
		{
			name: "terminated with crash",
			container: model.ContainerStatus{
				State: model.ContainerState{
					Terminated: &model.TerminatedState{
						ExitCode: 1,
					},
				},
			},
			check: func(issues []string) bool {
				return len(issues) == 1 && strings.Contains(issues[0], "crashed")
			},
		},
		{
			name: "waiting with ImagePullBackOff",
			container: model.ContainerStatus{
				State: model.ContainerState{
					Waiting: &model.WaitingState{
						Reason: "ImagePullBackOff",
					},
				},
			},
			check: func(issues []string) bool {
				return len(issues) == 1 && strings.Contains(issues[0], "pull image")
			},
		},
		{
			name:      "healthy running",
			container: model.ContainerStatus{State: model.ContainerState{Running: &model.RunningState{}}},
			check: func(issues []string) bool {
				return len(issues) == 0
			},
		},
		{
			name: "last state terminated with OOMKilled",
			container: model.ContainerStatus{
				State: model.ContainerState{Running: &model.RunningState{}},
				LastState: &model.ContainerState{
					Terminated: &model.TerminatedState{
						ExitCode: 137,
					},
				},
			},
			check: func(issues []string) bool {
				return len(issues) == 1 && strings.Contains(issues[0], "OOMKilled")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AnalyzeContainerState(tc.container)
			if !tc.check(got) {
				t.Errorf("AnalyzeContainerState() = %v, failed check", got)
			}
		})
	}
}

func TestAnalyzeLogs(t *testing.T) {
	cases := []struct {
		name  string
		logs  string
		check func([]string) bool
	}{
		{
			name:  "empty logs",
			logs:  "",
			check: func(issues []string) bool { return len(issues) == 0 },
		},
		{
			name: "connection refused",
			logs: "Error: connection refused to database",
			check: func(issues []string) bool {
				return len(issues) >= 1 && strings.Contains(issues[0], "Network error")
			},
		},
		{
			name: "timeout",
			logs: "Operation deadline exceeded",
			check: func(issues []string) bool {
				return len(issues) >= 1 && strings.Contains(issues[0], "Timeout")
			},
		},
		{
			name: "permission denied",
			logs: "open /etc/config: permission denied",
			check: func(issues []string) bool {
				return len(issues) >= 1 && strings.Contains(issues[0], "Permission denied")
			},
		},
		{
			name: "file not found",
			logs: "config file not found",
			check: func(issues []string) bool {
				return len(issues) >= 1 && strings.Contains(issues[0], "Missing file")
			},
		},
		{
			name:  "normal logs",
			logs:  "Starting application...\nListening on port 8080",
			check: func(issues []string) bool { return len(issues) == 0 },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AnalyzeLogs(tc.logs)
			if !tc.check(got) {
				t.Errorf("AnalyzeLogs(%q) = %v, failed check", tc.logs, got)
			}
		})
	}
}
