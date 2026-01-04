package tkss

// Args holds command-line arguments and options for tkss.
type Args struct {
	Namespace     string
	ShowLog       bool
	MaxLines      string
	Watch         bool
	WatchInterval int
	Preview       bool
	PipelineRuns  []string
	Shell         bool
	Follow        bool
	Explain       bool
	Model         string
	Persona       string
	Completion    string
}
