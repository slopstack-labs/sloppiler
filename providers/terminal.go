package providers

import (
	"os"
	"strings"
)

// ── ANSI palette ──────────────────────────────────────────────────────────────

// The escape sequences sloppiler styles its output with. These are variables
// rather than constants because not every destination can render them: a
// redirected stream, a NO_COLOR environment, or a Windows console that refuses
// virtual-terminal processing all want them neutralized to empty strings rather
// than emitted as literal "\033[36m" noise.
var (
	Reset  = "\033[0m"
	Bold   = "\033[1m"
	Dim    = "\033[2m"
	Green  = "\033[32m"
	Cyan   = "\033[36m"
	Yellow = "\033[33m"
	Red    = "\033[31m"
	// ClearLine returns the cursor to column zero and erases the line, so a
	// spinner frame can overwrite its predecessor in place.
	ClearLine = "\r\033[K"
)

// ── Glyphs ────────────────────────────────────────────────────────────────────

// Status glyphs, downgraded to ASCII when the console cannot be switched to
// UTF-8 output. A legacy Windows code page renders "✓" as a question mark,
// which reads as a failure rather than a success.
var (
	GlyphOK      = "✓"
	GlyphFail    = "✗"
	GlyphWarn    = "⚠"
	GlyphRetry   = "↻"
	GlyphIterate = "⟳"
)

var spinnerAnimationFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// animationsEnabled reports whether the spinner may redraw in place. Without
// ANSI support an "animation" is just thousands of duplicated log lines, so the
// spinner degrades to printing one settled line per step instead.
var animationsEnabled = true

func init() {
	configureTerminalOutput()
}

// configureTerminalOutput negotiates what the attached output stream can render
// and degrades the palette, glyphs, and animation accordingly.
func configureTerminalOutput() {
	ansiSupported, utf8Supported := enablePlatformTerminalFeatures()

	if !ansiSupported || !stderrIsInteractive() || colorSuppressedByEnvironment() {
		disableANSIStyling()
	}
	if !utf8Supported {
		useASCIIGlyphs()
	}
}

// colorSuppressedByEnvironment honours the two conventions users reach for when
// they want plain output: the NO_COLOR standard and TERM=dumb.
func colorSuppressedByEnvironment() bool {
	if _, present := os.LookupEnv("NO_COLOR"); present {
		return true
	}
	return strings.EqualFold(os.Getenv("TERM"), "dumb")
}

// stderrIsInteractive reports whether stderr is a character device — a console
// or terminal — rather than a file or pipe.
func stderrIsInteractive() bool {
	info, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// disableANSIStyling blanks every escape sequence. The existing format strings
// keep working verbatim — they simply interpolate empty strings — so a
// redirected sloppiler run produces a clean, greppable log.
func disableANSIStyling() {
	Reset, Bold, Dim, Green, Cyan, Yellow, Red = "", "", "", "", "", "", ""
	ClearLine = ""
	animationsEnabled = false
}

// useASCIIGlyphs swaps the Unicode status markers and spinner frames for
// characters every code page can represent.
func useASCIIGlyphs() {
	GlyphOK, GlyphFail, GlyphWarn, GlyphRetry, GlyphIterate = "OK", "XX", "!!", "->", "=>"
	spinnerAnimationFrames = []string{"|", "/", "-", "\\"}
}
