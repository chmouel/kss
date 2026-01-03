package kube

import (
	"reflect"
	"testing"

	"github.com/chmouel/kss/internal/model"
)

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
