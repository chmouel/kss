package tekton

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/chmouel/kss/internal/util"
)

// Runner is the command runner used by the tekton package.
var Runner util.Runner = &util.RealRunner{}

// FetchPipelineRun retrieves a PipelineRun via kubectl.
func FetchPipelineRun(kubectlArgs []string, name string) (PipelineRun, error) {
	cmdArgs := append(append([]string{}, kubectlArgs...), "get", "pipelinerun", name, "-ojson")
	output, err := Runner.Run("kubectl", cmdArgs...)
	if err != nil {
		return PipelineRun{}, fmt.Errorf("could not fetch pipelinerun %s: %s", name, strings.TrimSpace(string(output)))
	}

	var pr PipelineRun
	if err := json.Unmarshal(output, &pr); err != nil {
		return PipelineRun{}, fmt.Errorf("could not parse pipelinerun %s: %w", name, err)
	}
	return pr, nil
}

// FetchTaskRunsForPipelineRun retrieves TaskRuns labeled for a PipelineRun.
func FetchTaskRunsForPipelineRun(kubectlArgs []string, pipelineRun string) ([]TaskRun, error) {
	selector := fmt.Sprintf("tekton.dev/pipelineRun=%s", pipelineRun)
	cmdArgs := append(append([]string{}, kubectlArgs...), "get", "taskruns", "-l", selector, "-ojson")
	output, err := Runner.Run("kubectl", cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("could not fetch taskruns for pipelinerun %s: %s", pipelineRun, strings.TrimSpace(string(output)))
	}

	var list TaskRunList
	if err := json.Unmarshal(output, &list); err != nil {
		return nil, fmt.Errorf("could not parse taskruns for pipelinerun %s: %w", pipelineRun, err)
	}
	return list.Items, nil
}

// PodNameForTaskRun resolves the pod name for a TaskRun.
func PodNameForTaskRun(kubectlArgs []string, tr TaskRun) (string, error) {
	if tr.Status.PodName != "" {
		return tr.Status.PodName, nil
	}

	selectors := []string{"tekton.dev/taskRun", "tekton.dev/taskrun"}
	for _, label := range selectors {
		selector := fmt.Sprintf("%s=%s", label, tr.Metadata.Name)
		cmdArgs := append(append([]string{}, kubectlArgs...), "get", "pods", "-l", selector, "-o", "jsonpath={.items[0].metadata.name}")
		output, err := Runner.Run("kubectl", cmdArgs...)
		if err != nil {
			continue
		}
		podName := strings.TrimSpace(string(output))
		if podName != "" {
			return podName, nil
		}
	}

	return "", fmt.Errorf("no pod found for taskrun %s", tr.Metadata.Name)
}
