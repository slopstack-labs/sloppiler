//go:build windows

package providers

import (
	"os"
	"syscall"
	"unsafe"
)

const (
	// ENABLE_VIRTUAL_TERMINAL_PROCESSING — makes the console interpret ANSI
	// escape sequences instead of printing them literally. Off by default on
	// every Windows console host.
	enableVirtualTerminalProcessing = 0x0004
	// The UTF-8 code page, needed before the console can render the braille
	// spinner frames and status glyphs.
	utf8CodePage = 65001
)

var (
	kernel32DynamicLinkLibrary = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode         = kernel32DynamicLinkLibrary.NewProc("GetConsoleMode")
	procSetConsoleMode         = kernel32DynamicLinkLibrary.NewProc("SetConsoleMode")
	procSetConsoleOutputCP     = kernel32DynamicLinkLibrary.NewProc("SetConsoleOutputCP")
)

// enablePlatformTerminalFeatures opts the attached Windows console into ANSI
// escape interpretation and UTF-8 output, and reports what it managed to
// negotiate. Both are opt-in on Windows: without them, every colour code
// renders as literal "←[36m" and every spinner frame as "?".
//
// These calls are reached through kernel32 directly rather than through
// golang.org/x/sys so the module keeps its zero-dependency, CGO_ENABLED=0
// release builds.
func enablePlatformTerminalFeatures() (ansiSupported, utf8Supported bool) {
	consoleHandle := syscall.Handle(os.Stderr.Fd())

	var existingConsoleMode uint32
	queryResult, _, _ := procGetConsoleMode.Call(
		uintptr(consoleHandle),
		uintptr(unsafe.Pointer(&existingConsoleMode)),
	)
	if queryResult == 0 {
		// stderr is not a console — it is a pipe or a file. Escape sequences
		// would be literal noise there, but UTF-8 bytes land intact.
		return false, true
	}

	ansiSupported = existingConsoleMode&enableVirtualTerminalProcessing != 0
	if !ansiSupported {
		applyResult, _, _ := procSetConsoleMode.Call(
			uintptr(consoleHandle),
			uintptr(existingConsoleMode|enableVirtualTerminalProcessing),
		)
		ansiSupported = applyResult != 0
	}

	codePageResult, _, _ := procSetConsoleOutputCP.Call(uintptr(utf8CodePage))
	return ansiSupported, codePageResult != 0
}
