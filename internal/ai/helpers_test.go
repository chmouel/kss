package ai

import (
	"strings"
	"testing"
)

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

func TestFetchEventsJSON(t *testing.T) {
	mock := &MockRunner{Response: []byte("{}")}
	origRunner := Runner
	Runner = mock
	defer func() { Runner = origRunner }()

	got := fetchEventsJSON("kubectl", "Pod", "my-pod")
	if got != "{}" {
		t.Errorf("fetchEventsJSON() = %q, want %q", got, "{}")
	}

	if mock.CapturedName != "sh" {
		t.Errorf("expected sh command, got %s", mock.CapturedName)
	}
	if !strings.Contains(mock.CapturedArgs[1], "involvedObject.name=my-pod") {
		t.Errorf("expected command to filter by name")
	}
}
