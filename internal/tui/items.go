package tui

import (
	"fmt"

	"github.com/chmouel/kss/internal/model"
	"github.com/chmouel/kss/internal/tekton"
	"github.com/chmouel/kss/internal/util"
)

// PodItem implements list.Item for Kubernetes Pods
type PodItem struct {
	pod   model.Pod
	title string
	desc  string
}

// Title returns the pod item title.
func (i PodItem) Title() string { return i.title }

// Description returns the pod item description.
func (i PodItem) Description() string { return i.desc }

// FilterValue returns the value to use for filtering.
func (i PodItem) FilterValue() string { return i.title }

// NewPodItem creates a new PodItem from a Pod.
func NewPodItem(pod model.Pod) PodItem {
	status := pod.Status.Phase
	if status == "" {
		status = "Unknown"
	}

	return PodItem{
		pod:   pod,
		title: pod.Metadata.Name,
		desc:  fmt.Sprintf("Status: %s | Age: %s", status, util.FormatDuration(pod.Status.StartTime)),
	}
}

// PipelineRunItem implements list.Item for Tekton PipelineRuns
type PipelineRunItem struct {
	pr    tekton.PipelineRun
	title string
	desc  string
}

// Title returns the pipelinerun item title.
func (i PipelineRunItem) Title() string { return i.title }

// Description returns the pipelinerun item description.
func (i PipelineRunItem) Description() string { return i.desc }

// FilterValue returns the value to use for filtering.
func (i PipelineRunItem) FilterValue() string { return i.title }

// NewPipelineRunItem creates a new PipelineRunItem from a PipelineRun.
func NewPipelineRunItem(pr tekton.PipelineRun) PipelineRunItem {
	status := "Unknown"
	if len(pr.Status.Conditions) > 0 {
		status = pr.Status.Conditions[0].Reason
	}

	return PipelineRunItem{
		pr:    pr,
		title: pr.Metadata.Name,
		desc:  fmt.Sprintf("Status: %s | Age: %s", status, util.FormatDuration(pr.Metadata.CreationTimestamp)),
	}
}
