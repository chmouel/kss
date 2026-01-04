package kube

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/chmouel/kss/internal/model"
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

func TestKubectlArgs(t *testing.T) {
	cases := []struct {
		name string
		args model.Args
		want []string
	}{
		{
			name: "no namespace",
			args: model.Args{},
			want: nil,
		},
		{
			name: "with namespace",
			args: model.Args{Namespace: "foo"},
			want: []string{"-", "n", "foo"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := KubectlArgs(tc.args)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("KubectlArgs(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestFetchPod(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &MockRunner{
			Response: []byte(`{"metadata":{"name":"pod-1"}}`),
		}
		origRunner := runner
		runner = mock
		defer func() { runner = origRunner }()

		pod, err := FetchPod([]string{"-n", "ns"}, "pod-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pod.Metadata.Name != "pod-1" {
			t.Errorf("got name %q, want pod-1", pod.Metadata.Name)
		}
		
		expectedArgs := []string{"-n", "ns", "get", "pod", "pod-1", "-ojson"}
		if !reflect.DeepEqual(mock.CapturedArgs, expectedArgs) {
			t.Errorf("got args %v, want %v", mock.CapturedArgs, expectedArgs)
		}
	})

	t.Run("command error", func(t *testing.T) {
		mock := &MockRunner{Err: errors.New("fail")}
		origRunner := runner
		runner = mock
		defer func() { runner = origRunner }()

		_, err := FetchPod(nil, "pod-1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("json error", func(t *testing.T) {
		mock := &MockRunner{Response: []byte("invalid")}
		origRunner := runner
		runner = mock
		defer func() { runner = origRunner }()

		_, err := FetchPod(nil, "pod-1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestShowLog(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &MockRunner{Response: []byte("logs...")}
		origRunner := runner
		runner = mock
		defer func() { runner = origRunner }()

		args := model.Args{MaxLines: "100"}
		got, err := ShowLog("kubectl", args, "c1", "p1", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "logs..." {
			t.Errorf("got logs %q, want logs...", got)
		}
		
		if mock.CapturedName != "sh" {
			t.Errorf("expected command 'sh', got %q", mock.CapturedName)
		}
		if len(mock.CapturedArgs) != 2 || mock.CapturedArgs[0] != "-c" {
			t.Errorf("expected sh -c ...")
		}
		if !strings.Contains(mock.CapturedArgs[1], "kubectl logs --tail=100 p1 -cc1") {
			t.Errorf("unexpected command string: %s", mock.CapturedArgs[1])
		}
	})

	t.Run("previous", func(t *testing.T) {
		mock := &MockRunner{Response: []byte("logs")}
		origRunner := runner
		runner = mock
		defer func() { runner = origRunner }()

		args := model.Args{MaxLines: "10"}
		_, _ = ShowLog("kubectl", args, "c1", "p1", true)
		if !strings.Contains(mock.CapturedArgs[1], "-p") {
			t.Errorf("expected -p flag in %s", mock.CapturedArgs[1])
		}
	})
}
