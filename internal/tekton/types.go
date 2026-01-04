package tekton

// Metadata captures the Kubernetes metadata used by Tekton resources.
type Metadata struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	CreationTimestamp string            `json:"creationTimestamp,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
}

// Condition represents a Tekton condition entry.
type Condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// PipelineRunSpec contains pipeline reference details.
type PipelineRunSpec struct {
	PipelineRef  *Ref          `json:"pipelineRef,omitempty"`
	PipelineSpec *PipelineSpec `json:"pipelineSpec,omitempty"`
}

// Ref points to a named resource.
type Ref struct {
	Name string `json:"name,omitempty"`
}

// PipelineSpec is a minimal inline pipeline spec placeholder.
type PipelineSpec struct {
	Description string `json:"description,omitempty"`
}

// PipelineRunStatus captures PipelineRun status timestamps and conditions.
type PipelineRunStatus struct {
	Conditions     []Condition `json:"conditions,omitempty"`
	StartTime      string      `json:"startTime,omitempty"`
	CompletionTime string      `json:"completionTime,omitempty"`
}

// PipelineRun is a minimal Tekton PipelineRun model.
type PipelineRun struct {
	Metadata Metadata          `json:"metadata"`
	Spec     PipelineRunSpec   `json:"spec,omitempty"`
	Status   PipelineRunStatus `json:"status,omitempty"`
}

// PipelineRunList wraps a list of PipelineRuns.
type PipelineRunList struct {
	Items []PipelineRun `json:"items"`
}

// TaskRunStatus captures TaskRun status fields.
type TaskRunStatus struct {
	Conditions     []Condition `json:"conditions,omitempty"`
	StartTime      string      `json:"startTime,omitempty"`
	CompletionTime string      `json:"completionTime,omitempty"`
	PodName        string      `json:"podName,omitempty"`
}

// TaskRun is a minimal Tekton TaskRun model.
type TaskRun struct {
	Metadata Metadata      `json:"metadata"`
	Status   TaskRunStatus `json:"status,omitempty"`
}

// TaskRunList wraps a list of TaskRuns.
type TaskRunList struct {
	Items []TaskRun `json:"items"`
}

// ConditionForType returns the first condition matching the given type.
func ConditionForType(conditions []Condition, condType string) *Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

// StatusLabel returns a friendly status label and color for Succeeded conditions.
func StatusLabel(conditions []Condition) (label, color, reason, message string) {
	cond := ConditionForType(conditions, "Succeeded")
	if cond == nil {
		return "Unknown", "yellow", "", ""
	}

	reason = cond.Reason
	message = cond.Message

	switch cond.Status {
	case "True":
		return "Succeeded", "green", reason, message
	case "False":
		return "Failed", "red", reason, message
	default:
		return "Running", "yellow", reason, message
	}
}

// TaskRunDisplayName returns a friendly name for the TaskRun.
func TaskRunDisplayName(tr TaskRun) string {
	if tr.Metadata.Labels != nil {
		if name := tr.Metadata.Labels["tekton.dev/pipelineTask"]; name != "" {
			return name
		}
		if name := tr.Metadata.Labels["tekton.dev/task"]; name != "" {
			return name
		}
	}
	return tr.Metadata.Name
}
