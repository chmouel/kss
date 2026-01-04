package tkss

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/chmouel/kss/internal/kube"
	"github.com/chmouel/kss/internal/tekton"
	"github.com/chmouel/kss/internal/util"
)

// StepTarget identifies the TaskRun, pod, and container to target.
type StepTarget struct {
	TaskRunName   string
	TaskName      string
	PodName       string
	ContainerName string
}

// RequireFzf ensures fzf is available in PATH.
func RequireFzf() error {
	if util.Which("fzf") == "" {
		return fmt.Errorf("fzf is required but was not found in PATH")
	}
	return nil
}

// BuildStepTargets resolves TaskRun pods and returns selectable step targets.
func BuildStepTargets(kubectlArgs []string, taskRuns []tekton.TaskRun) ([]StepTarget, error) {
	var targets []StepTarget
	for i := range taskRuns {
		tr := &taskRuns[i]
		podName, err := tekton.PodNameForTaskRun(kubectlArgs, *tr)
		if err != nil {
			return nil, err
		}

		podObj, err := kube.FetchPod(kubectlArgs, podName)
		if err != nil {
			return nil, err
		}

		taskName := tekton.TaskRunDisplayName(*tr)
		for _, container := range podObj.Spec.Containers {
			targets = append(targets, StepTarget{
				TaskRunName:   tr.Metadata.Name,
				TaskName:      taskName,
				PodName:       podName,
				ContainerName: container.Name,
			})
		}
	}
	return targets, nil
}

func (t StepTarget) fzfLine() string {
	displayTask := t.TaskRunName
	if t.TaskName != "" && t.TaskName != t.TaskRunName {
		displayTask = fmt.Sprintf("%s (%s)", t.TaskName, t.TaskRunName)
	}
	return fmt.Sprintf("%s\t%s\t%s", displayTask, t.PodName, t.ContainerName)
}

// SelectStepTarget prompts for a step selection using fzf.
func SelectStepTarget(targets []StepTarget) (StepTarget, error) {
	if len(targets) == 0 {
		return StepTarget{}, fmt.Errorf("no steps available for selection")
	}

	lines := make([]string, 0, len(targets))
	byLine := make(map[string]StepTarget, len(targets))
	for _, target := range targets {
		line := target.fzfLine()
		lines = append(lines, line)
		byLine[line] = target
	}

	cmd := exec.Command("fzf", "-0", "-1", "--prompt", "step> ", "--with-nth", "1,2,3")
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return StepTarget{}, fmt.Errorf("fzf selection failed: %w", err)
	}

	selection := strings.TrimSpace(output.String())
	if selection == "" {
		return StepTarget{}, fmt.Errorf("no step selected")
	}

	selected, ok := byLine[selection]
	if !ok {
		return StepTarget{}, fmt.Errorf("unexpected fzf selection")
	}

	return selected, nil
}
