package main

import (
	"reflect"
	"testing"
)

func TestFilterContainersByRestrict(t *testing.T) {
	containers := []string{"api", "worker", "sidecar"}

	cases := []struct {
		name      string
		restrict  string
		want      []string
		wantError bool
	}{
		{
			name:     "empty restrict",
			restrict: "",
			want:     containers,
		},
		{
			name:     "regex match",
			restrict: "^(api|side)",
			want:     []string{"api", "sidecar"},
		},
		{
			name:      "invalid regex",
			restrict:  "[",
			wantError: true,
		},
		{
			name:      "no matches",
			restrict:  "db",
			wantError: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := filterContainersByRestrict(containers, tc.restrict)
			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unexpected result: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestContainerStateLabel(t *testing.T) {
	tests := []struct {
		name    string
		status  ContainerStatus
		want    string
		running bool
	}{
		{
			name:    "running",
			status:  ContainerStatus{State: ContainerState{Running: &RunningState{StartedAt: "now"}}},
			want:    "running",
			running: true,
		},
		{
			name:    "waiting with reason",
			status:  ContainerStatus{State: ContainerState{Waiting: &WaitingState{Reason: "CrashLoopBackOff"}}},
			want:    "waiting: CrashLoopBackOff",
			running: false,
		},
		{
			name:    "terminated with exit code",
			status:  ContainerStatus{State: ContainerState{Terminated: &TerminatedState{ExitCode: 137}}},
			want:    "terminated: exit 137",
			running: false,
		},
		{
			name:    "unknown",
			status:  ContainerStatus{},
			want:    "unknown",
			running: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, running := containerStateLabel(tc.status)
			if got != tc.want {
				t.Fatalf("unexpected state: got %q want %q", got, tc.want)
			}
			if running != tc.running {
				t.Fatalf("unexpected running flag: got %v want %v", running, tc.running)
			}
		})
	}
}

func TestIsBashShell(t *testing.T) {
	cases := []struct {
		shell string
		want  bool
	}{
		{shell: "/bin/bash", want: true},
		{shell: "/usr/bin/bash", want: true},
		{shell: "/bin/sh", want: false},
		{shell: "/bin/ash", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.shell, func(t *testing.T) {
			got := isBashShell(tc.shell)
			if got != tc.want {
				t.Fatalf("unexpected result: got %v want %v", got, tc.want)
			}
		})
	}
}
