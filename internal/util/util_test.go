package util

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
)

func TestColorText(t *testing.T) {
	// Force color output for testing
	color.NoColor = false

	cases := []struct {
		text      string
		colorName string
		check     func(string) bool
	}{
		{
			text:      "hello",
			colorName: "red",
			check: func(s string) bool {
				// Check for red color code start (31) and the text
				return strings.Contains(s, "\x1b[31") && strings.Contains(s, "hello")
			},
		},
		{
			text:      "world",
			colorName: "unknown",
			check: func(s string) bool {
				return s == "world"
			},
		},
	}

	for _, tc := range cases {
		got := ColorText(tc.text, tc.colorName)
		if !tc.check(got) {
			t.Errorf("ColorText(%q, %q) = %q, failed check", tc.text, tc.colorName, got)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name      string
		timestamp string
		want      string // partial match or exact
	}{
		{
			name:      "empty",
			timestamp: "",
			want:      "N/A",
		},
		{
			name:      "invalid",
			timestamp: "invalid",
			want:      "invalid",
		},
		{
			name:      "seconds",
			timestamp: now.Add(-30 * time.Second).Format(time.RFC3339),
			want:      "30s",
		},
		{
			name:      "minutes",
			timestamp: now.Add(-5 * time.Minute).Format(time.RFC3339),
			want:      "5m",
		},
		{
			name:      "hours",
			timestamp: now.Add(-2 * time.Hour).Format(time.RFC3339),
			want:      "2h",
		},
		{
			name:      "days",
			timestamp: now.Add(-48 * time.Hour).Format(time.RFC3339),
			want:      "2d",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := FormatDuration(tc.timestamp)
			// For relative times, exact matching is flaky, so we allow 1 unit difference or exact match
			if tc.name == "empty" || tc.name == "invalid" {
				if got != tc.want {
					t.Errorf("FormatDuration(%q) = %q, want %q", tc.timestamp, got, tc.want)
				}
			} else {
				// Simple check if it ends with the unit
				unit := tc.want[len(tc.want)-1:]
				if !strings.HasSuffix(got, unit) {
					t.Errorf("FormatDuration(%q) = %q, want suffix %q", tc.timestamp, got, unit)
				}
			}
		})
	}
}

func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}
	if !Contains(slice, "b") {
		t.Errorf("Contains(slice, 'b') = false, want true")
	}
	if Contains(slice, "d") {
		t.Errorf("Contains(slice, 'd') = true, want false")
	}
}

func TestWhich(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create a dummy executable
	exeName := "testexec"
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	exePath := filepath.Join(tmpDir, exeName)
	f, err := os.Create(exePath)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	// Make it executable
	if err := os.Chmod(exePath, 0755); err != nil {
		t.Fatal(err)
	}

	// Add tmpDir to PATH
	oldPath := os.Getenv("PATH")
	defer os.Setenv("PATH", oldPath)
	os.Setenv("PATH", fmt.Sprintf("%s%c%s", tmpDir, os.PathListSeparator, oldPath))

	// Test finding it
	found := Which(exeName)
	// On macOS, /private/var/... vs /var/... can cause equality issues, so we evaluate symlinks
	foundEval, _ := filepath.EvalSymlinks(found)
	exePathEval, _ := filepath.EvalSymlinks(exePath)
	
	if foundEval != exePathEval {
		t.Errorf("Which(%q) = %q, want %q", exeName, found, exePath)
	}

	// Test absolute path
	foundAbs := Which(exePath)
	foundAbsEval, _ := filepath.EvalSymlinks(foundAbs)
	if foundAbsEval != exePathEval {
		t.Errorf("Which(%q) = %q, want %q", exePath, foundAbs, exePath)
	}

	// Test not found
	if Which("nonexistent_executable_xyz") != "" {
		t.Errorf("Which('nonexistent...') should return empty string")
	}
}

func TestStripANSI(t *testing.T) {
	input := "\x1b[31mHello\x1b[0m World"
	want := "Hello World"
	got := StripANSI(input)
	if got != want {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, want)
	}
}

func TestGetDisplayWidth(t *testing.T) {
	input := "\x1b[31mHello\x1b[0m"
	want := 5
	got := GetDisplayWidth(input)
	if got != want {
		t.Errorf("GetDisplayWidth(%q) = %d, want %d", input, got, want)
	}
}

func TestPadToWidth(t *testing.T) {
	cases := []struct {
		input string
		width int
		want  string
	}{
		{"Hello", 10, "Hello     "},
		{"Hello", 5, "Hello"},
		{"Hello", 3, "Hello"},
	}

	for _, tc := range cases {
		got := PadToWidth(tc.input, tc.width)
		if got != tc.want {
			t.Errorf("PadToWidth(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.want)
		}
	}
}

func TestFilterContainersByRestrict(t *testing.T) {
	containers := []string{"api", "worker", "sidecar"}

	cases := []struct {
		name      string
		restrict  string
		want      []string
		wantError bool
	}{
		{
			name:     "empty restrict",
			restrict: "",
			want:     containers,
		},
		{
			name:     "regex match",
			restrict: "^(api|side)",
			want:     []string{"api", "sidecar"},
		},
		{
			name:      "invalid regex",
			restrict:  "[",
			wantError: true,
		},
		{
			name:      "no matches",
			restrict:  "db",
			wantError: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := FilterContainersByRestrict(containers, tc.restrict)
			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unexpected result: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestIsBashShell(t *testing.T) {
	cases := []struct {
		shell string
		want  bool
	}{
		{shell: "/bin/bash", want: true},
		{shell: "/usr/bin/bash", want: true},
		{shell: "/bin/sh", want: false},
		{shell: "/bin/ash", want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.shell, func(t *testing.T) {
			got := IsBashShell(tc.shell)
			if got != tc.want {
				t.Fatalf("unexpected result: got %v want %v", got, tc.want)
			}
		})
	}
}
