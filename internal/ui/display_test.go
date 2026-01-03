package ui

import (
	"testing"

	"github.com/chmouel/kss/internal/model"
)

func TestGetStateIcon(t *testing.T) {
	cases := []struct {
		state string
		want  string
	}{
		{state: "Running", want: "✓"},
		{state: "RUNNING", want: "✓"},
		{state: "FAIL", want: "✗"},
		{state: "FAILED", want: "✗"},
		{state: "SUCCESS", want: "✓"},
		{state: "Waiting", want: "⏳"},
		{state: "Other", want: "•"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.state, func(t *testing.T) {
			got := getStateIcon(tc.state)
			if got != tc.want {
				t.Fatalf("getStateIcon(%q) = %q, want %q", tc.state, got, tc.want)
			}
		})
	}
}

func TestLensc(t *testing.T) {
	containers := []model.ContainerStatus{
		{State: model.ContainerState{Waiting: &model.WaitingState{Reason: "CrashLoopBackOff"}}},
		{State: model.ContainerState{Terminated: &model.TerminatedState{ExitCode: 0}}},
		{State: model.ContainerState{Waiting: &model.WaitingState{Reason: "ContainerCreating"}}},
		{State: model.ContainerState{Terminated: &model.TerminatedState{ExitCode: 2}}},
	}

	got := lensc(containers)
	if got != 2 {
		t.Fatalf("lensc() = %d, want 2", got)
	}
}

func TestHasFailure(t *testing.T) {
	cases := []struct {
		name       string
		containers []model.ContainerStatus
		want       bool
	}{
		{
			name: "waiting failure",
			containers: []model.ContainerStatus{
				{State: model.ContainerState{Waiting: &model.WaitingState{Reason: "ImagePullBackOff"}}},
			},
			want: true,
		},
		{
			name: "terminated failure",
			containers: []model.ContainerStatus{
				{State: model.ContainerState{Terminated: &model.TerminatedState{ExitCode: 1}}},
			},
			want: true,
		},
		{
			name: "no failure",
			containers: []model.ContainerStatus{
				{State: model.ContainerState{Waiting: &model.WaitingState{Reason: "ContainerCreating"}}},
				{State: model.ContainerState{Terminated: &model.TerminatedState{ExitCode: 0}}},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := hasFailure(tc.containers)
			if got != tc.want {
				t.Fatalf("hasFailure() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGetStatus(t *testing.T) {
	cases := []struct {
		name       string
		hasFailure bool
		allc       int
		allf       int
		wantColor  string
		wantText   string
	}{
		{
			name:       "has failures",
			hasFailure: true,
			allc:       2,
			allf:       2,
			wantColor:  "red",
			wantText:   "❌ FAIL",
		},
		{
			name:       "running",
			hasFailure: false,
			allc:       2,
			allf:       1,
			wantColor:  "blue",
			wantText:   "🔄 RUNNING",
		},
		{
			name:       "success",
			hasFailure: false,
			allc:       2,
			allf:       2,
			wantColor:  "green",
			wantText:   "✅ SUCCESS",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			color, text := getStatus(tc.hasFailure, tc.allc, tc.allf)
			if color != tc.wantColor || text != tc.wantText {
				t.Fatalf("getStatus() = (%q, %q), want (%q, %q)", color, text, tc.wantColor, tc.wantText)
			}
		})
	}
}
