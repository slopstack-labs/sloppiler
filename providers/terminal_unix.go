//go:build !windows

package providers

// enablePlatformTerminalFeatures is a no-op outside Windows: every terminal
// sloppiler is likely to meet on Linux or macOS already interprets ANSI escape
// sequences and already speaks UTF-8.
func enablePlatformTerminalFeatures() (ansiSupported, utf8Supported bool) {
	return true, true
}
