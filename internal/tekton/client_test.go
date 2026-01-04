package tekton

import (
	"errors"
	"strings"
	"testing"
)

// MockRunner captures calls and returns canned responses.
type MockRunner struct {
	CapturedName string
	CapturedArgs []string
	Response     []byte
	Err          error
}

func (m *MockRunner) Run(name string, args ...string) ([]byte, error) {
	m.CapturedName = name
	m.CapturedArgs = args
	return m.Response, m.Err
}

func TestFetchPipelineRun(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &MockRunner{
			Response: []byte(`{"metadata":{"name":"pr-1","namespace":"ns"},"spec":{"pipelineRef":{"name":"p-1"}}}`),
		}
		origRunner := Runner
		Runner = mock
		defer func() { Runner = origRunner }()

		pr, err := FetchPipelineRun([]string{"-n", "ns"}, "pr-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pr.Metadata.Name != "pr-1" {
			t.Errorf("got name %q, want pr-1", pr.Metadata.Name)
		}

		// Verify args
		expectedArgs := []string{"-n", "ns", "get", "pipelinerun", "pr-1", "-ojson"}
		if len(mock.CapturedArgs) != len(expectedArgs) {
			t.Errorf("got args %v, want %v", mock.CapturedArgs, expectedArgs)
		}
	})

	t.Run("command error", func(t *testing.T) {
		mock := &MockRunner{
			Err: errors.New("command failed"),
		}
		origRunner := Runner
		Runner = mock
		defer func() { Runner = origRunner }()

		_, err := FetchPipelineRun(nil, "pr-1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "could not fetch pipelinerun") {
			t.Errorf("error %q should contain 'could not fetch pipelinerun'", err.Error())
		}
	})

	t.Run("json error", func(t *testing.T) {
		mock := &MockRunner{
			Response: []byte(`invalid json`),
		}
		origRunner := Runner
		Runner = mock
		defer func() { Runner = origRunner }()

		_, err := FetchPipelineRun(nil, "pr-1")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "could not parse pipelinerun") {
			t.Errorf("error %q should contain 'could not parse pipelinerun'", err.Error())
		}
	})
}

func TestFetchTaskRunsForPipelineRun(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &MockRunner{
			Response: []byte(`{"items":[{"metadata":{"name":"tr-1"}}]}`),
		}
		origRunner := Runner
		Runner = mock
		defer func() { Runner = origRunner }()

		trs, err := FetchTaskRunsForPipelineRun([]string{"-n", "ns"}, "pr-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(trs) != 1 || trs[0].Metadata.Name != "tr-1" {
			t.Errorf("got %v, want one taskrun 'tr-1'", trs)
		}
	})

	t.Run("command error", func(t *testing.T) {
		mock := &MockRunner{Err: errors.New("fail")}
		origRunner := Runner
		Runner = mock
		defer func() { Runner = origRunner }()

		_, err := FetchTaskRunsForPipelineRun(nil, "pr-1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPodNameForTaskRun(t *testing.T) {
	t.Run("from status", func(t *testing.T) {
		tr := TaskRun{
			Status: TaskRunStatus{PodName: "pod-1"},
		}
		got, err := PodNameForTaskRun(nil, tr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "pod-1" {
			t.Errorf("got %q, want pod-1", got)
		}
	})

	t.Run("from lookup success", func(t *testing.T) {
		tr := TaskRun{Metadata: Metadata{Name: "tr-1"}}
		mock := &MockRunner{
			Response: []byte("pod-looked-up"),
		}
		origRunner := Runner
		Runner = mock
		defer func() { Runner = origRunner }()

		got, err := PodNameForTaskRun(nil, tr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "pod-looked-up" {
			t.Errorf("got %q, want pod-looked-up", got)
		}
	})

	t.Run("from lookup fail", func(t *testing.T) {
		tr := TaskRun{Metadata: Metadata{Name: "tr-1"}}
		mock := &MockRunner{
			Err: errors.New("not found"),
		}
		origRunner := Runner
		Runner = mock
		defer func() { Runner = origRunner }()

		_, err := PodNameForTaskRun(nil, tr)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "no pod found") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
