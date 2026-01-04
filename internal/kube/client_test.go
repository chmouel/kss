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
		origRunner := Runner
		Runner = mock
		defer func() { Runner = origRunner }()

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
		origRunner := Runner
		Runner = mock
		defer func() { Runner = origRunner }()

		_, err := FetchPod(nil, "pod-1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("json error", func(t *testing.T) {
		mock := &MockRunner{Response: []byte("invalid")}
		origRunner := Runner
		Runner = mock
		defer func() { Runner = origRunner }()

		_, err := FetchPod(nil, "pod-1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestShowLog(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock := &MockRunner{Response: []byte("logs...")}
		origRunner := Runner
		Runner = mock
		defer func() { Runner = origRunner }()

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
		origRunner := Runner
		Runner = mock
		defer func() { Runner = origRunner }()

		args := model.Args{MaxLines: "10"}
		_, _ = ShowLog("kubectl", args, "c1", "p1", true)
		if !strings.Contains(mock.CapturedArgs[1], "-p") {
			t.Errorf("expected -p flag in %s", mock.CapturedArgs[1])
		}
	})
}

func TestContainerInfoForPod(t *testing.T) {
	mock := &MockRunner{
		Response: []byte(`{
			"spec": {
				"containers": [{"name": "c1"}, {"name": "c2"}]
			},
			"status": {
				"containerStatuses": [
					{"name": "c1", "state": {"running": {}}}
				]
			}
		}`),
	}
	origRunner := Runner
	Runner = mock
	defer func() { Runner = origRunner }()

	infos, err := containerInfoForPod(nil, "pod-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(infos) != 2 {
		t.Fatalf("got %d infos, want 2", len(infos))
	}

	// c1 should be running
	if infos[0].Name != "c1" || !infos[0].Running {
		t.Errorf("c1: got name=%q running=%v, want c1 running=true", infos[0].Name, infos[0].Running)
	}

	// c2 should be unknown/not running
	if infos[1].Name != "c2" || infos[1].Running {
		t.Errorf("c2: got name=%q running=%v, want c2 running=false", infos[1].Name, infos[1].Running)
	}
}
