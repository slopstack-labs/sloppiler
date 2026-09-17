package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
)

// ── Host defaults ─────────────────────────────────────────────────────────────

// defaultTarget returns the target OS to materialize a binary for when the user
// did not pass --target. Defaulting to the host means the binary sloppiler hands
// back is one the machine that produced it can actually attempt to execute.
func defaultTarget() string {
	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		return runtime.GOOS
	default:
		return "linux"
	}
}

// defaultArch returns the CPU architecture to materialize a binary for when the
// user did not pass --arch. Anything outside the two supported architectures
// falls back to amd64.
func defaultArch() string {
	if runtime.GOARCH == "arm64" {
		return "arm64"
	}
	return "amd64"
}

// ── Output naming ─────────────────────────────────────────────────────────────

// resolveOutputPath adapts the requested output path to the target OS. Windows
// resolves executables by extension, so a PE binary written to an extensionless
// path cannot be launched from a shell at all. The default name is swapped
// wholesale (a.out → a.exe) and any user-supplied name without an extension
// gains one; a name that already carries an extension is left untouched.
func resolveOutputPath(requestedPath, defaultPath, target string) string {
	if target != "windows" {
		return requestedPath
	}
	if requestedPath == defaultPath {
		return "a.exe"
	}
	if filepath.Ext(requestedPath) == "" {
		return requestedPath + ".exe"
	}
	return requestedPath
}

// ── Toolchain discovery ───────────────────────────────────────────────────────

// lookToolPath returns the first candidate command that resolves on PATH.
// Toolchain binaries carry different names depending on how they were installed:
// a cross toolchain prefixes them (x86_64-w64-mingw32-ld), while a native
// Windows MinGW-w64 or MSYS2 install exposes them bare (ld).
func lookToolPath(candidates ...string) (string, bool) {
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}

// windowsNativeGNULinkerCandidates returns the unprefixed GNU linker names that
// may be probed for a Windows PE link. On a Windows host a bare "ld" is
// necessarily the PE-native MinGW linker, so probing it is safe and lets a
// standard MSYS2 or WinLibs install work with no cross-prefixed aliases. On
// Linux and macOS a bare "ld" is the host ELF or Mach-O linker and would
// silently produce an unusable artifact, so it is deliberately excluded.
func windowsNativeGNULinkerCandidates() []string {
	if runtime.GOOS == "windows" {
		return []string{"ld", "ld.bfd", "ld.lld"}
	}
	return nil
}

// msvcLinkerArgs builds the argument vector for an MSVC-style linker
// (link.exe or lld-link), which uses a completely different flag dialect from
// GNU ld: slash-prefixed options and .lib inputs resolved through %LIB%.
func msvcLinkerArgs(objFile, outputPath string) []string {
	return []string{
		"/entry:_start",
		"/subsystem:console",
		"/nologo",
		"/out:" + outputPath,
		objFile,
		"kernel32.lib",
	}
}

// windowsLinkerGuidanceComponent picks which install hint to surface when no
// Windows linker was found. On a Windows host, pointing at the Visual Studio
// Build Tools reaches more developers than a MinGW cross toolchain does.
func windowsLinkerGuidanceComponent() string {
	if runtime.GOOS == "windows" {
		return "msvc"
	}
	return "mingw"
}

// ── Host capability guardrails ────────────────────────────────────────────────

// hostLinkSupportError reports why --optimistic cannot link for the requested
// target from the given host, or nil when the combination is plausible.
//
// This exists because the failure it prevents is silent rather than loud: a
// Windows host has a PE-native "ld" on PATH, and asking it for a Linux ELF
// yields a confusing link error or, worse, an artifact that is not what was
// asked for. Failing fast with an explanation beats shipping a mystery binary.
func hostLinkSupportError(hostOS, target string) error {
	if hostOS != "windows" {
		return nil
	}
	if target == "windows" || target == "darwin" {
		return nil
	}
	return fmt.Errorf("--optimistic with --target=%s is not supported on a Windows host: the PE-native `ld` on PATH cannot emit ELF executables — use default (hex) mode for a Linux binary, or run sloppiler under WSL", target)
}

// ── Install guidance ──────────────────────────────────────────────────────────

// toolInstallGuidance holds the per-host install instruction for one toolchain
// component. A "sudo pacman -S nasm" hint is actively unhelpful to the 70% of
// developers who are not on Arch, and unusable to everyone on Windows.
type toolInstallGuidance struct {
	windows string
	darwin  string
	linux   string
}

var toolInstallGuidanceByName = map[string]toolInstallGuidance{
	"nasm": {
		windows: "winget install NASM.NASM (then add %ProgramFiles%\\NASM to PATH)",
		darwin:  "brew install nasm",
		linux:   "sudo apt install nasm  /  sudo pacman -S nasm",
	},
	"as": {
		windows: "winget install BrechtSanders.WinLibs.POSIX.UCRT (ships the GNU assembler)",
		darwin:  "xcode-select --install",
		linux:   "sudo apt install binutils  /  sudo pacman -S binutils",
	},
	"mingw": {
		windows: "winget install BrechtSanders.WinLibs.POSIX.UCRT (ships ld and dlltool)",
		darwin:  "brew install mingw-w64",
		linux:   "sudo apt install binutils-mingw-w64  /  sudo pacman -S mingw-w64-binutils",
	},
	"msvc": {
		windows: "install the Visual Studio Build Tools and run sloppiler from a Developer PowerShell so link.exe and %LIB% are on PATH",
		darwin:  "not available on macOS",
		linux:   "not available on Linux",
	},
	"lld": {
		windows: "winget install LLVM.LLVM",
		darwin:  "brew install lld",
		linux:   "sudo apt install lld  /  sudo pacman -S lld",
	},
	"binutils-arm64": {
		windows: "winget install LLVM.LLVM (ld.lld can link AArch64)",
		darwin:  "brew install aarch64-elf-binutils  /  brew install lld",
		linux:   "sudo apt install binutils-aarch64-linux-gnu  /  sudo pacman -S aarch64-linux-gnu-binutils",
	},
}

// installHintFor renders the install instruction for the named component on the
// current host, formatted for appending to an error message.
func installHintFor(componentName string) string {
	guidance, known := toolInstallGuidanceByName[componentName]
	if !known {
		return ""
	}
	var instruction string
	switch runtime.GOOS {
	case "windows":
		instruction = guidance.windows
	case "darwin":
		instruction = guidance.darwin
	default:
		instruction = guidance.linux
	}
	if instruction == "" {
		return ""
	}
	return "install it first: " + instruction
}

// missingToolError builds a consistent "not found" error carrying host-specific
// remediation guidance.
func missingToolError(displayName, componentName string) error {
	if hint := installHintFor(componentName); hint != "" {
		return fmt.Errorf("%s not found in PATH — %s", displayName, hint)
	}
	return fmt.Errorf("%s not found in PATH", displayName)
}
