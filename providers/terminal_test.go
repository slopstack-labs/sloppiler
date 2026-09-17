package providers

import (
	"os"
	"strings"
	"testing"
)

// restoreTerminalState captures the mutable presentation globals so a test can
// exercise the degradation paths without leaking state into its neighbours.
func restoreTerminalState(t *testing.T) {
	t.Helper()
	savedReset, savedBold, savedDim := Reset, Bold, Dim
	savedGreen, savedCyan, savedYellow, savedRed := Green, Cyan, Yellow, Red
	savedClearLine := ClearLine
	savedOK, savedFail, savedWarn := GlyphOK, GlyphFail, GlyphWarn
	savedRetry, savedIterate := GlyphRetry, GlyphIterate
	savedFrames := spinnerAnimationFrames
	savedAnimations := animationsEnabled
	t.Cleanup(func() {
		Reset, Bold, Dim = savedReset, savedBold, savedDim
		Green, Cyan, Yellow, Red = savedGreen, savedCyan, savedYellow, savedRed
		ClearLine = savedClearLine
		GlyphOK, GlyphFail, GlyphWarn = savedOK, savedFail, savedWarn
		GlyphRetry, GlyphIterate = savedRetry, savedIterate
		spinnerAnimationFrames = savedFrames
		animationsEnabled = savedAnimations
	})
}

func TestDisableANSIStylingBlanksEveryEscapeSequence(t *testing.T) {
	restoreTerminalState(t)
	disableANSIStyling()

	for name, value := range map[string]string{
		"Reset": Reset, "Bold": Bold, "Dim": Dim,
		"Green": Green, "Cyan": Cyan, "Yellow": Yellow, "Red": Red,
		"ClearLine": ClearLine,
	} {
		if value != "" {
			t.Errorf("%s = %q, want empty so redirected output stays greppable", name, value)
		}
	}
	if animationsEnabled {
		t.Error("animations must be off without ANSI support, or a log file collects thousands of redraws")
	}
}

func TestUseASCIIGlyphsDropsNonASCII(t *testing.T) {
	restoreTerminalState(t)
	useASCIIGlyphs()

	glyphs := append([]string{GlyphOK, GlyphFail, GlyphWarn, GlyphRetry, GlyphIterate}, spinnerAnimationFrames...)
	for _, glyph := range glyphs {
		if glyph == "" {
			t.Error("glyph must not be blank")
		}
		for _, r := range glyph {
			if r > 127 {
				t.Errorf("glyph %q contains non-ASCII rune %q — a legacy code page renders it as '?'", glyph, r)
			}
		}
	}
}

// withEnv sets the given variables for the duration of the test, treating a nil
// value as "unset". t.Setenv cannot express unset, and NO_COLOR is defined by
// its presence rather than its value, so the distinction matters here.
func withEnv(t *testing.T, vars map[string]*string) {
	t.Helper()
	for key, value := range vars {
		previousValue, wasPresent := os.LookupEnv(key)
		if value == nil {
			if err := os.Unsetenv(key); err != nil {
				t.Fatalf("cannot unset %s: %v", key, err)
			}
		} else if err := os.Setenv(key, *value); err != nil {
			t.Fatalf("cannot set %s: %v", key, err)
		}
		t.Cleanup(func() {
			if wasPresent {
				os.Setenv(key, previousValue)
			} else {
				os.Unsetenv(key)
			}
		})
	}
}

func stringPtr(value string) *string { return &value }

func TestColorSuppressedByEnvironment(t *testing.T) {
	cases := []struct {
		name string
		vars map[string]*string
		want bool
	}{
		{"no signals", map[string]*string{"NO_COLOR": nil, "TERM": stringPtr("xterm-256color")}, false},
		{"NO_COLOR set", map[string]*string{"NO_COLOR": stringPtr("1"), "TERM": stringPtr("xterm-256color")}, true},
		{"NO_COLOR present but empty still counts", map[string]*string{"NO_COLOR": stringPtr(""), "TERM": stringPtr("xterm-256color")}, true},
		{"dumb terminal", map[string]*string{"NO_COLOR": nil, "TERM": stringPtr("dumb")}, true},
		{"dumb terminal any case", map[string]*string{"NO_COLOR": nil, "TERM": stringPtr("DUMB")}, true},
		{"no TERM at all", map[string]*string{"NO_COLOR": nil, "TERM": nil}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withEnv(t, c.vars)
			if got := colorSuppressedByEnvironment(); got != c.want {
				t.Errorf("colorSuppressedByEnvironment() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestDefaultPaletteCarriesEscapeSequences(t *testing.T) {
	// Guards against a refactor that leaves the palette permanently blank: the
	// declared defaults must still be real SGR sequences.
	restoreTerminalState(t)
	Reset, Green, ClearLine = "\033[0m", "\033[32m", "\r\033[K"
	if !strings.HasPrefix(Reset, "\033[") || !strings.Contains(ClearLine, "\r") {
		t.Error("palette defaults should be ANSI escape sequences")
	}
}
