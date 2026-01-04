package tkss

import "testing"

func TestFormatDurationBetween(t *testing.T) {
	cases := []struct {
		name  string
		start string
		end   string
		want  string
	}{
		{
			name:  "seconds",
			start: "2024-01-01T00:00:00Z",
			end:   "2024-01-01T00:00:42Z",
			want:  "42s",
		},
		{
			name:  "minutes",
			start: "2024-01-01T00:00:00Z",
			end:   "2024-01-01T00:05:00Z",
			want:  "5m",
		},
		{
			name:  "hours",
			start: "2024-01-01T00:00:00Z",
			end:   "2024-01-01T03:00:00Z",
			want:  "3h",
		},
		{
			name:  "days",
			start: "2024-01-01T00:00:00Z",
			end:   "2024-01-03T00:00:00Z",
			want:  "2d",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := formatDurationBetween(tc.start, tc.end)
			if got != tc.want {
				t.Fatalf("formatDurationBetween() = %q, want %q", got, tc.want)
			}
		})
	}
}
