package model

import "fmt"

// Args holds command-line arguments and options
type Args struct {
	Namespace     string   // Kubernetes namespace to use
	Restrict      string   // Regex pattern to restrict container display
	ShowLog       bool     // Whether to show container logs
	MaxLines      string   // Maximum number of log lines to show
	Labels        bool     // Whether to show pod labels
	Annotations   bool     // Whether to show pod annotations
	Events        bool     // Whether to show pod events
	Watch         bool     // Whether to enable watch mode
	WatchInterval int      // Watch refresh interval in seconds
	Preview       bool     // Whether to use compact preview mode (for fzf)
	Pods          []string // List of pod names to display
	Shell         bool     // Whether to open an interactive shell
	Doctor        bool     // Enable heuristic analysis
	Explain       bool     // Enable AI explanation
	Model         string   // AI Model to use
	Persona       string   // AI Persona to use
	Completion    string   // Output shell completion code
}

// ContainerState mirrors the possible lifecycle phases of a pod container.
type ContainerState struct {
	Waiting    *WaitingState    `json:"waiting,omitempty"`
	Running    *RunningState    `json:"running,omitempty"`
	Terminated *TerminatedState `json:"terminated,omitempty"`
}

// WaitingState captures a container that is waiting to start or restart.
type WaitingState struct {
	Reason  string `json:"reason"`
	Message string `json:"message,omitempty"`
}

// RunningState indicates that the container is currently executing.
type RunningState struct {
	StartedAt string `json:"startedAt,omitempty"`
}

// TerminatedState describes a container that has exited, including exit code details.
type TerminatedState struct {
	ExitCode   int    `json:"exitCode"`
	Message    string `json:"message,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
}

// ContainerStatus reflects the real-time status and metadata for a single container.
type ContainerStatus struct {
	Name         string          `json:"name"`
	State        ContainerState  `json:"state"`
	LastState    *ContainerState `json:"lastState,omitempty"`
	Ready        bool            `json:"ready"`
	RestartCount int             `json:"restartCount"`
	Image        string          `json:"image,omitempty"`
	ImageID      string          `json:"imageID,omitempty"`
}

// StateLabel summarizes the container state as a user-friendly label plus readiness.
func (status ContainerStatus) StateLabel() (string, bool) {
	switch {
	case status.State.Running != nil:
		return "running", true
	case status.State.Waiting != nil:
		reason := status.State.Waiting.Reason
		if reason == "" {
			reason = "waiting"
		}
		return "waiting: " + reason, false
	case status.State.Terminated != nil:
		reason := status.State.Terminated.Message
		if reason == "" {
			reason = fmt.Sprintf("exit %d", status.State.Terminated.ExitCode)
		}
		return "terminated: " + reason, false
	default:
		return "unknown", false
	}
}

// PodStatus aggregates status information for all containers in a pod.
type PodStatus struct {
	InitContainerStatuses []ContainerStatus `json:"initContainerStatuses,omitempty"`
	ContainerStatuses     []ContainerStatus `json:"containerStatuses,omitempty"`
	Phase                 string            `json:"phase"`
	StartTime             string            `json:"startTime,omitempty"`
	PodIP                 string            `json:"podIP,omitempty"`
	HostIP                string            `json:"hostIP,omitempty"`
	NodeName              string            `json:"nodeName,omitempty"`
	QOSClass              string            `json:"qosClass,omitempty"`
	Conditions            []PodCondition    `json:"conditions,omitempty"`
}

// PodCondition records Kubernetes condition data for a pod.
type PodCondition struct {
	Type               string `json:"type"`
	Status             string `json:"status"`
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
}

// PodMetadata wraps identifying metadata for a pod.
type PodMetadata struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	CreationTimestamp string            `json:"creationTimestamp,omitempty"`
}

// Pod is the high-level model object for a Kubernetes pod.
type Pod struct {
	Metadata PodMetadata `json:"metadata"`
	Status   PodStatus   `json:"status"`
	Spec     PodSpec     `json:"spec,omitempty"`
}

// PodList represents a list of Pods.
type PodList struct {
	Items []Pod `json:"items"`
}

// PodSpec represents the spec field of a pod, mostly used for container info.
type PodSpec struct {
	Containers         []ContainerSpec `json:"containers,omitempty"`
	InitContainers     []ContainerSpec `json:"initContainers,omitempty"`
	NodeName           string          `json:"nodeName,omitempty"`
	ServiceAccountName string          `json:"serviceAccountName,omitempty"`
	PriorityClassName  string          `json:"priorityClassName,omitempty"`
}

// ResourceList maps Kubernetes resource names (CPU/memory) to string values.
type ResourceList map[string]string

// Resources bundles request/limit pairs for compute resources.
type Resources struct {
	Requests ResourceList `json:"requests,omitempty"`
	Limits   ResourceList `json:"limits,omitempty"`
}

// ContainerSpec mirrors the spec of each container defined on a pod.
type ContainerSpec struct {
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	Resources Resources `json:"resources,omitempty"`
}

// FailedContainers lists Kubernetes container states that indicate failure
var FailedContainers = []string{
	"ImagePullBackOff",
	"CrashLoopBackOff",
	"ErrImagePull",
	"CreateContainerConfigError",
	"InvalidImageName",
}
