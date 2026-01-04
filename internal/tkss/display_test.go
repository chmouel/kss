package tkss

import (
	"strings"
	"testing"
	"time"
)

func TestFormatDurationBetween(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name  string
		start string
		end   string
		want  string
	}{
		{
			name:  "seconds",
			start: now.Add(-30 * time.Second).Format(time.RFC3339),
			end:   now.Format(time.RFC3339),
			want:  "30s",
		},
		{
			name:  "minutes",
			start: now.Add(-5 * time.Minute).Format(time.RFC3339),
			end:   now.Format(time.RFC3339),
			want:  "5m",
		},
		{
			name:  "hours",
			start: now.Add(-2 * time.Hour).Format(time.RFC3339),
			end:   now.Format(time.RFC3339),
			want:  "2h",
		},
		{
			name:  "days",
			start: now.Add(-48 * time.Hour).Format(time.RFC3339),
			end:   now.Format(time.RFC3339),
			want:  "2d",
		},
		{
			name:  "empty start",
			start: "",
			end:   now.Format(time.RFC3339),
			want:  "N/A",
		},
		{
			name:  "invalid start",
			start: "invalid",
			end:   now.Format(time.RFC3339),
			want:  "N/A",
		},
		{
			name:  "empty end (uses now)",
			start: now.Add(-10 * time.Minute).Format(time.RFC3339),
			end:   "",
			want:  "10m",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := formatDurationBetween(tc.start, tc.end)
			if !strings.HasSuffix(got, tc.want[len(tc.want)-1:]) && got != tc.want {
				t.Errorf("formatDurationBetween(%q, %q) = %q, want %q", tc.start, tc.end, got, tc.want)
			}
		})
	}
}
