package doctor

import (
	"strings"
	"testing"

	"github.com/chmouel/kss/internal/kube"
	"github.com/chmouel/kss/internal/model"
)

type mockRunner struct {
	response []byte
	err      error
}

func (m *mockRunner) Run(name string, args ...string) ([]byte, error) {
	return m.response, m.err
}

func TestDiagnosePod(t *testing.T) {
	// Mock kube.Runner
	origRunner := kube.Runner
	mock := &mockRunner{
		response: []byte("error: connection refused"),
	}
	kube.Runner = mock
	defer func() { kube.Runner = origRunner }()

	pod := model.Pod{
		Metadata: model.PodMetadata{Name: "pod-1"},
		Status: model.PodStatus{
			ContainerStatuses: []model.ContainerStatus{
				{
					Name: "c1",
					State: model.ContainerState{
						Terminated: &model.TerminatedState{ExitCode: 1},
					},
				},
			},
		},
	}

	// We can't easily capture stdout without refactoring DiagnosePod to take an io.Writer,
	// but we can at least run it to ensure no panics and coverage.
	// For now, let's just ensure it runs.
	DiagnosePod(pod, "kubectl", "pod-1", model.Args{})
}

func TestAnalyzeContainerState(t *testing.T) {
	cases := []struct {
		name      string
		container model.ContainerStatus
		want      []string
	}{
		{
			name: "terminated oomkilled",
			container: model.ContainerStatus{
				State: model.ContainerState{
					Terminated: &model.TerminatedState{ExitCode: 137},
				},
			},
			want: []string{"Likely OOMKilled (Out of Memory)"},
		},
		{
			name: "terminated crash",
			container: model.ContainerStatus{
				State: model.ContainerState{
					Terminated: &model.TerminatedState{ExitCode: 1},
				},
			},
			want: []string{"Application crashed (Exit Code 1)"},
		},
		{
			name: "waiting crashloop",
			container: model.ContainerStatus{
				State: model.ContainerState{
					Waiting: &model.WaitingState{Reason: "CrashLoopBackOff"},
				},
			},
			want: []string{"Container is crashing repeatedly"},
		},
		{
			name: "waiting image pull",
			container: model.ContainerStatus{
				State: model.ContainerState{
					Waiting: &model.WaitingState{Reason: "ImagePullBackOff"},
				},
			},
			want: []string{"Failed to pull image"},
		},
		{
			name:      "healthy",
			container: model.ContainerStatus{},
			want:      nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := AnalyzeContainerState(tc.container)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d issues, want %d", len(got), len(tc.want))
			}
			for i, issue := range got {
				if tc.want[i] != "" && !strings.Contains(issue, tc.want[i]) {
					t.Errorf("issue %q does not contain %q", issue, tc.want[i])
				}
			}
		})
	}
}

func TestAnalyzeLogs(t *testing.T) {
	cases := []struct {
		name string
		logs string
		want []string
	}{
		{
			name: "empty",
			logs: "",
			want: nil,
		},
		{
			name: "timeout",
			logs: "error: context deadline exceeded while connecting",
			want: []string{"Timeout detected"},
		},
		{
			name: "connection refused",
			logs: "dial tcp 127.0.0.1:80: connection refused",
			want: []string{"Network error detected"},
		},
		{
			name: "permission denied",
			logs: "open /etc/secret: permission denied",
			want: []string{"Permission denied"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := AnalyzeLogs(tc.logs)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d issues, want %d", len(got), len(tc.want))
			}
			for i, issue := range got {
				if tc.want[i] != "" && !strings.Contains(issue, tc.want[i]) {
					t.Errorf("issue %q does not contain %q", issue, tc.want[i])
				}
			}
		})
	}
}
