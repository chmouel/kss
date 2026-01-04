package ai

import (
	"strings"
	"testing"
)

func TestPersonaDisplayName(t *testing.T) {
	t.Run("known persona", func(t *testing.T) {
		if got := personaDisplayName("pirate"); got == "pirate" {
			t.Fatalf("personaDisplayName() = %q, expected display name", got)
		}
	})

	t.Run("unknown persona", func(t *testing.T) {
		if got := personaDisplayName("custom"); got != "custom" {
			t.Fatalf("personaDisplayName() = %q, want %q", got, "custom")
		}
	})
}

func TestPersonaInstructions(t *testing.T) {
	t.Run("neutral persona", func(t *testing.T) {
		got := personaInstructions("neutral")
		if !strings.Contains(got, "neutral") {
			t.Fatalf("personaInstructions() = %q, expected neutral guidance", got)
		}
	})

	t.Run("default persona", func(t *testing.T) {
		got := personaInstructions("something-else")
		if !strings.Contains(got, "Alfred") {
			t.Fatalf("personaInstructions() = %q, expected default Alfred guidance", got)
		}
	})
}
