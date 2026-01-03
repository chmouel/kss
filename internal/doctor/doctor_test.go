package doctor

import (
	"strings"
	"testing"

	"github.com/chmouel/kss/internal/model"
)

func TestAnalyzeContainerStateTerminatedExitCodes(t *testing.T) {
	cases := []struct {
		name        string
		exitCode    int
		wantSubstr  string
		wantEntries int
	}{
		{
			name:        "oomkilled",
			exitCode:    137,
			wantSubstr:  "OOMKilled",
			wantEntries: 1,
		},
		{
			name:        "app error",
			exitCode:    1,
			wantSubstr:  "Exit Code 1",
			wantEntries: 1,
		},
		{
			name:        "command missing",
			exitCode:    127,
			wantSubstr:  "Command not found",
			wantEntries: 1,
		},
		{
			name:        "success",
			exitCode:    0,
			wantEntries: 0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			container := model.ContainerStatus{
				State: model.ContainerState{
					Terminated: &model.TerminatedState{ExitCode: tc.exitCode},
				},
			}
			issues := AnalyzeContainerState(container)
			if len(issues) != tc.wantEntries {
				t.Fatalf("expected %d issue(s), got %d: %v", tc.wantEntries, len(issues), issues)
			}
			if tc.wantSubstr != "" && !strings.Contains(issues[0], tc.wantSubstr) {
				t.Fatalf("expected issue to contain %q, got %q", tc.wantSubstr, issues[0])
			}
		})
	}
}

func TestAnalyzeContainerStateLastState(t *testing.T) {
	container := model.ContainerStatus{
		State: model.ContainerState{},
		LastState: &model.ContainerState{
			Terminated: &model.TerminatedState{ExitCode: 127},
		},
	}

	issues := AnalyzeContainerState(container)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d: %v", len(issues), issues)
	}
	if !strings.Contains(issues[0], "Command not found") {
		t.Fatalf("expected command not found issue, got %q", issues[0])
	}
}

func TestAnalyzeContainerStateWaitingReasons(t *testing.T) {
	cases := []struct {
		name       string
		reason     string
		wantSubstr string
	}{
		{
			name:       "image pull",
			reason:     "ImagePullBackOff",
			wantSubstr: "pull image",
		},
		{
			name:       "crashloop",
			reason:     "CrashLoopBackOff",
			wantSubstr: "crashing repeatedly",
		},
		{
			name:       "config error",
			reason:     "CreateContainerConfigError",
			wantSubstr: "Configuration error",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			container := model.ContainerStatus{
				State: model.ContainerState{
					Waiting: &model.WaitingState{Reason: tc.reason},
				},
			}
			issues := AnalyzeContainerState(container)
			if len(issues) != 1 {
				t.Fatalf("expected 1 issue, got %d: %v", len(issues), issues)
			}
			if !strings.Contains(issues[0], tc.wantSubstr) {
				t.Fatalf("expected issue to contain %q, got %q", tc.wantSubstr, issues[0])
			}
		})
	}
}

func TestAnalyzeLogs(t *testing.T) {
	logs := strings.Join([]string{
		"Connection refused while dialing",
		"timeout while contacting service",
		"permission denied",
		"config file not found",
	}, "\n")

	issues := AnalyzeLogs(logs)
	want := []string{
		"Network error",
		"Timeout",
		"Permission denied",
		"Missing file",
	}
	for _, fragment := range want {
		if !containsIssue(issues, fragment) {
			t.Fatalf("expected issue containing %q, got %v", fragment, issues)
		}
	}
}

func TestAnalyzeLogsEmpty(t *testing.T) {
	issues := AnalyzeLogs("")
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
}

func containsIssue(issues []string, fragment string) bool {
	for _, issue := range issues {
		if strings.Contains(issue, fragment) {
			return true
		}
	}
	return false
}
