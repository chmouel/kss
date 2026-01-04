package tekton

import "testing"

func TestStatusLabel(t *testing.T) {
	cases := []struct {
		name      string
		conds     []Condition
		wantLabel string
		wantColor string
	}{
		{
			name:      "missing condition",
			conds:     nil,
			wantLabel: "Unknown",
			wantColor: "yellow",
		},
		{
			name: "succeeded",
			conds: []Condition{{
				Type:   "Succeeded",
				Status: "True",
			}},
			wantLabel: "Succeeded",
			wantColor: "green",
		},
		{
			name: "failed",
			conds: []Condition{{
				Type:   "Succeeded",
				Status: "False",
			}},
			wantLabel: "Failed",
			wantColor: "red",
		},
		{
			name: "running",
			conds: []Condition{{
				Type:   "Succeeded",
				Status: "Unknown",
			}},
			wantLabel: "Running",
			wantColor: "yellow",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			label, color, _, _ := StatusLabel(tc.conds)
			if label != tc.wantLabel {
				t.Fatalf("label = %q, want %q", label, tc.wantLabel)
			}
			if color != tc.wantColor {
				t.Fatalf("color = %q, want %q", color, tc.wantColor)
			}
		})
	}
}

func TestTaskRunDisplayName(t *testing.T) {
	cases := []struct {
		name string
		tr   TaskRun
		want string
	}{
		{
			name: "pipeline task label",
			tr: TaskRun{Metadata: Metadata{
				Name: "tr-name",
				Labels: map[string]string{
					"tekton.dev/pipelineTask": "build",
				},
			}},
			want: "build",
		},
		{
			name: "task label",
			tr: TaskRun{Metadata: Metadata{
				Name: "tr-name",
				Labels: map[string]string{
					"tekton.dev/task": "lint",
				},
			}},
			want: "lint",
		},
		{
			name: "fallback name",
			tr:   TaskRun{Metadata: Metadata{Name: "tr-name"}},
			want: "tr-name",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := TaskRunDisplayName(tc.tr)
			if got != tc.want {
				t.Fatalf("TaskRunDisplayName() = %q, want %q", got, tc.want)
			}
		})
	}
}
