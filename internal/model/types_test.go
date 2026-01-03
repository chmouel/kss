package model

import "testing"

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
			got, running := tc.status.StateLabel()
			if got != tc.want {
				t.Fatalf("unexpected state: got %q want %q", got, tc.want)
			}
			if running != tc.running {
				t.Fatalf("unexpected running flag: got %v want %v", running, tc.running)
			}
		})
	}
}
