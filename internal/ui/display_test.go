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
		{"Running", "✓"},
		{"RUNNING", "✓"},
		{"FAIL", "✗"},
		{"FAILED", "✗"},
		{"SUCCESS", "✓"},
		{"Waiting", "⏳"},
		{"Unknown", "•"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.state, func(t *testing.T) {
			got := getStateIcon(tc.state)
			if got != tc.want {
				t.Errorf("getStateIcon(%q) = %q, want %q", tc.state, got, tc.want)
			}
		})
	}
}

func TestHasFailure(t *testing.T) {
	cases := []struct {
		name       string
		containers []model.ContainerStatus
		want       bool
	}{
		{
			name: "no containers",
			want: false,
		},
		{
			name: "running container",
			containers: []model.ContainerStatus{
				{State: model.ContainerState{Running: &model.RunningState{}}},
			},
			want: false,
		},
		{
			name: "terminated success",
			containers: []model.ContainerStatus{
				{State: model.ContainerState{Terminated: &model.TerminatedState{ExitCode: 0}}},
			},
			want: false,
		},
		{
			name: "terminated failure",
			containers: []model.ContainerStatus{
				{State: model.ContainerState{Terminated: &model.TerminatedState{ExitCode: 1}}},
			},
			want: true,
		},
		{
			name: "waiting crashloop",
			containers: []model.ContainerStatus{
				{State: model.ContainerState{Waiting: &model.WaitingState{Reason: "CrashLoopBackOff"}}},
			},
			want: true,
		},
		{
			name: "waiting benign",
			containers: []model.ContainerStatus{
				{State: model.ContainerState{Waiting: &model.WaitingState{Reason: "ContainerCreating"}}},
			},
			want: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := hasFailure(tc.containers)
			if got != tc.want {
				t.Errorf("hasFailure() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGetStatus(t *testing.T) {
	cases := []struct {
		name        string
		hasFailures bool
		allc        int
		allf        int
		wantColor   string
		wantText    string
	}{
		{
			name:        "failure",
			hasFailures: true,
			wantColor:   "red",
			wantText:    "❌ FAIL",
		},
		{
			name:        "running partial",
			hasFailures: false,
			allc:        2,
			allf:        1,
			wantColor:   "blue",
			wantText:    "🔄 RUNNING",
		},
		{
			name:        "success",
			hasFailures: false,
			allc:        2,
			allf:        2,
			wantColor:   "green",
			wantText:    "✅ SUCCESS",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			gotColor, gotText := getStatus(tc.hasFailures, tc.allc, tc.allf)
			if gotColor != tc.wantColor {
				t.Errorf("getStatus() color = %q, want %q", gotColor, tc.wantColor)
			}
			if gotText != tc.wantText {
				t.Errorf("getStatus() text = %q, want %q", gotText, tc.wantText)
			}
		})
	}
}

func TestLensc(t *testing.T) {
	// lensc counts "successes" where success is defined as:
	// - Waiting with FailedContainers reason (Wait, logic seems inverted or I misunderstood lensc)
	// Let's re-read lensc in display.go

	/*
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
	*/

	// It counts failures (waiting failed) AND terminated success (exit code 0).
	// This seems to be "finished or failed" count?
	// Ah, looking at usage:
	// cntFailicontainers := lensc(podObj.Status.InitContainerStatuses)
	// cntAllicontainers := len(podObj.Status.InitContainerStatuses)
	// ...
	// getStatus(..., cntAllcontainers+cntAllicontainers, cntFailcontainers+cntFailicontainers)

	// If allc != allf -> RUNNING.
	// So lensc seems to count "containers that are NOT running"?
	// Waiting+Fail -> Counted.
	// Terminated+Success -> Counted.
	// Running -> Not counted.
	// Terminated+Fail -> Not counted (wait, really?)

	// If Terminated+Fail, hasFailure returns true, so getStatus returns FAIL immediately.

	// So lensc seems to count "completed successfully or failed to start"?
	// Wait, if Waiting+Fail is counted, and Terminated+Success is counted.
	// If Running, it is NOT counted.

	// If allc != allf, it returns RUNNING.
	// So if I have 1 container running. len=1. lensc=0. 1 != 0 -> RUNNING. Correct.
	// If I have 1 container terminated success. len=1. lensc=1. 1 == 1 -> SUCCESS. Correct.

	// Test cases based on this understanding.

	cases := []struct {
		name       string
		containers []model.ContainerStatus
		want       int
	}{
		{
			name: "running",
			containers: []model.ContainerStatus{
				{State: model.ContainerState{Running: &model.RunningState{}}},
			},
			want: 0,
		},
		{
			name: "terminated success",
			containers: []model.ContainerStatus{
				{State: model.ContainerState{Terminated: &model.TerminatedState{ExitCode: 0}}},
			},
			want: 1,
		},
		{
			name: "waiting failed",
			containers: []model.ContainerStatus{
				{State: model.ContainerState{Waiting: &model.WaitingState{Reason: "CrashLoopBackOff"}}},
			},
			want: 1,
		},
		{
			name: "terminated fail",
			containers: []model.ContainerStatus{
				{State: model.ContainerState{Terminated: &model.TerminatedState{ExitCode: 1}}},
			},
			want: 0,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := lensc(tc.containers)
			if got != tc.want {
				t.Errorf("lensc() = %d, want %d", got, tc.want)
			}
		})
	}
}
