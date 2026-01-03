package util

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/mattn/go-runewidth"
)

// ColorText applies color formatting to text based on the color name
func ColorText(text, colorName string) string {
	switch colorName {
	case "red":
		return color.New(color.FgRed, color.Bold).Sprint(text)
	case "yellow":
		return color.New(color.FgYellow, color.Bold).Sprint(text)
	case "blue":
		return color.New(color.FgBlue, color.Bold).Sprint(text)
	case "cyan":
		return color.New(color.FgCyan, color.Bold).Sprint(text)
	case "green":
		return color.New(color.FgGreen, color.Bold).Sprint(text)
	case "magenta":
		return color.New(color.FgMagenta, color.Bold).Sprint(text)
	case "white":
		return color.New(color.FgWhite).Sprint(text)
	case "white_bold":
		return color.New(color.FgWhite, color.Bold).Sprint(text)
	case "dim":
		return color.New(color.FgWhite, color.Faint).Sprint(text)
	default:
		return text
	}
}

// FormatDuration converts a Kubernetes timestamp to a human-readable duration
func FormatDuration(timestamp string) string {
	if timestamp == "" {
		return "N/A"
	}
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return timestamp
	}
	duration := time.Since(t)
	switch {
	case duration < time.Minute:
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	case duration < time.Hour:
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%dh", int(duration.Hours()))
	default:
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
}

// Contains checks if a string slice contains a specific item
func Contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}

// Which finds the full path of an executable in PATH
func Which(program string) string {
	if filepath.IsAbs(program) {
		if _, err := os.Stat(program); err == nil {
			return program
		}
		return ""
	}

	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		fullPath := filepath.Join(dir, program)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath
		}
	}
	return ""
}

// StripANSI removes ANSI color codes from a string to get actual display width
func StripANSI(s string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return re.ReplaceAllString(s, "")
}

// GetDisplayWidth calculates the actual display width of a string
func GetDisplayWidth(s string) int {
	clean := StripANSI(s)
	return runewidth.StringWidth(clean)
}

// PadToWidth pads a string to a specific display width
func PadToWidth(s string, width int) string {
	actualWidth := GetDisplayWidth(s)
	if actualWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-actualWidth)
}

// FilterContainersByRestrict filters a list of containers by a regex pattern
func FilterContainersByRestrict(containers []string, restrict string) ([]string, error) {
	if restrict == "" {
		return containers, nil
	}

	re, err := regexp.Compile(restrict)
	if err != nil {
		return nil, fmt.Errorf("invalid restrict regex: %w", err)
	}

	var filtered []string
	for _, container := range containers {
		if re.MatchString(container) {
			filtered = append(filtered, container)
		}
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("no containers matched restrict %q", restrict)
	}

	return filtered, nil
}

// IsBashShell checks if the shell path looks like bash
func IsBashShell(shell string) bool {
	return filepath.Base(shell) == "bash"
}
