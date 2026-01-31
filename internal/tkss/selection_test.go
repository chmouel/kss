package tkss

import (
	"strings"
	"testing"
)

func TestFzfLine(t *testing.T) {
	cases := []struct {
		name   string
		target StepTarget
		want   []string
	}{
		{
			name: "with display name",
			target: StepTarget{
				TaskRunName:   "tr-1",
				TaskName:      "task-1",
				PodName:       "pod-1",
				ContainerName: "c1",
			},
			want: []string{"task-1 (tr-1)", "pod-1", "c1"},
		},
		{
			name: "without display name",
			target: StepTarget{
				TaskRunName:   "tr-1",
				TaskName:      "",
				PodName:       "pod-1",
				ContainerName: "c1",
			},
			want: []string{"tr-1", "pod-1", "c1"},
		},
		{
			name: "same display name",
			target: StepTarget{
				TaskRunName:   "tr-1",
				TaskName:      "tr-1",
				PodName:       "pod-1",
				ContainerName: "c1",
			},
			want: []string{"tr-1", "pod-1", "c1"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.target.fzfLine()
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("fzfLine() = %q, want it to contain %q", got, w)
				}
			}
		})
	}
}

func TestDisplayLine(t *testing.T) {
	cases := []struct {
		name   string
		target StepTarget
		want   []string
	}{
		{
			name: "with display name",
			target: StepTarget{
				TaskRunName:   "tr-1",
				TaskName:      "task-1",
				PodName:       "pod-1",
				ContainerName: "c1",
			},
			want: []string{"task-1 (tr-1)", "c1"},
		},
		{
			name: "without display name",
			target: StepTarget{
				TaskRunName:   "tr-1",
				TaskName:      "",
				PodName:       "pod-1",
				ContainerName: "c1",
			},
			want: []string{"tr-1", "c1"},
		},
		{
			name: "same display name",
			target: StepTarget{
				TaskRunName:   "tr-1",
				TaskName:      "tr-1",
				PodName:       "pod-1",
				ContainerName: "c1",
			},
			want: []string{"tr-1", "c1"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.target.displayLine()
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("displayLine() = %q, want it to contain %q", got, w)
				}
			}
		})
	}
}

func TestSelectStepTarget(t *testing.T) {
	t.Run("empty targets", func(t *testing.T) {
		_, err := SelectStepTarget(nil)
		if err == nil {
			t.Error("expected error for empty targets")
		}
		if !strings.Contains(err.Error(), "no steps available") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("single target returns immediately", func(t *testing.T) {
		target := StepTarget{
			TaskRunName:   "tr-1",
			TaskName:      "task-1",
			PodName:       "pod-1",
			ContainerName: "c1",
		}
		got, err := SelectStepTarget([]StepTarget{target})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != target {
			t.Errorf("got %+v, want %+v", got, target)
		}
	})
}
