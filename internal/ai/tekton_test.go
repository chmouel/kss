package ai

import (
	"strings"
	"testing"

	"github.com/chmouel/kss/internal/tekton"
)

func TestTaskRunSummary(t *testing.T) {
	taskRuns := []tekton.TaskRun{
		{
			Metadata: tekton.Metadata{
				Name: "tr-1",
				Labels: map[string]string{
					"tekton.dev/pipelineTask": "build",
				},
			},
			Status: tekton.TaskRunStatus{
				Conditions: []tekton.Condition{
					{
						Type:    "Succeeded",
						Status:  "False",
						Reason:  "Failed",
						Message: "boom",
					},
				},
			},
		},
	}

	got := taskRunSummary(taskRuns)
	if !strings.Contains(got, "build (tr-1): Failed") {
		t.Fatalf("taskRunSummary() = %q, expected task status line", got)
	}
	if !strings.Contains(got, "boom") {
		t.Fatalf("taskRunSummary() = %q, expected message", got)
	}
}
