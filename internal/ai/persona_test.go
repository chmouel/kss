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

func TestPersonaASCIIArt(t *testing.T) {
	personas := []string{"butler", "pirate", "sergeant", "hacker", "genz", "neutral"}

	for _, persona := range personas {
		t.Run(persona+" persona", func(t *testing.T) {
			got := personaASCIIArt(persona)
			if got == "" {
				t.Fatalf("personaASCIIArt(%q) returned empty string", persona)
			}
			if !strings.Contains(got, "\n") {
				t.Fatalf("personaASCIIArt(%q) should return multi-line ASCII art", persona)
			}
		})
	}

	t.Run("unknown persona returns default", func(t *testing.T) {
		got := personaASCIIArt("unknown")
		defaultArt := personaASCIIArt("neutral")
		if got != defaultArt {
			t.Fatalf("personaASCIIArt(unknown) should return default art")
		}
	})
}
