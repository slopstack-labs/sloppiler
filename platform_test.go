package main

import (
	"runtime"
	"strings"
	"testing"
)

// ── Host defaults ─────────────────────────────────────────────────────────────

func TestDefaultTargetIsASupportedTarget(t *testing.T) {
	switch got := defaultTarget(); got {
	case "linux", "windows", "darwin":
	default:
		t.Errorf("defaultTarget() = %q, want one of linux/windows/darwin", got)
	}
}

func TestDefaultTargetTracksHostOS(t *testing.T) {
	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		if got := defaultTarget(); got != runtime.GOOS {
			t.Errorf("defaultTarget() = %q, want %q on this host", got, runtime.GOOS)
		}
	}
}

func TestDefaultArchIsASupportedArch(t *testing.T) {
	switch got := defaultArch(); got {
	case "amd64", "arm64":
	default:
		t.Errorf("defaultArch() = %q, want amd64 or arm64", got)
	}
}

// ── Output naming ─────────────────────────────────────────────────────────────

func TestResolveOutputPath(t *testing.T) {
	cases := []struct {
		name      string
		requested string
		target    string
		want      string
	}{
		{"default name on windows becomes a.exe", "a.out", "windows", "a.exe"},
		{"extensionless name on windows gains .exe", "hello", "windows", "hello.exe"},
		{"explicit extension on windows is preserved", "hello.exe", "windows", "hello.exe"},
		{"unrelated extension on windows is preserved", "hello.bin", "windows", "hello.bin"},
		{"nested path on windows gains .exe", "out/hello", "windows", "out/hello.exe"},
		{"linux is untouched", "a.out", "linux", "a.out"},
		{"linux extensionless is untouched", "hello", "linux", "hello"},
		{"darwin is untouched", "hello", "darwin", "hello"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveOutputPath(c.requested, defaultOutputPath, c.target); got != c.want {
				t.Errorf("resolveOutputPath(%q, %q) = %q, want %q", c.requested, c.target, got, c.want)
			}
		})
	}
}

// ── Host capability guardrails ────────────────────────────────────────────────

func TestHostLinkSupportError(t *testing.T) {
	cases := []struct {
		hostOS    string
		target    string
		wantError bool
	}{
		{"windows", "linux", true},
		{"windows", "", true},
		{"windows", "windows", false},
		{"windows", "darwin", false},
		{"linux", "linux", false},
		{"linux", "windows", false},
		{"darwin", "linux", false},
	}
	for _, c := range cases {
		err := hostLinkSupportError(c.hostOS, c.target)
		if (err != nil) != c.wantError {
			t.Errorf("hostLinkSupportError(%q, %q) error = %v, wantError = %v",
				c.hostOS, c.target, err, c.wantError)
		}
	}
}

func TestHostLinkSupportErrorExplainsTheWayOut(t *testing.T) {
	err := hostLinkSupportError("windows", "linux")
	if err == nil {
		t.Fatal("expected an error for linux target on a windows host")
	}
	if !strings.Contains(err.Error(), "hex") || !strings.Contains(err.Error(), "WSL") {
		t.Errorf("error should point at the available workarounds, got: %v", err)
	}
}

// ── Toolchain discovery ───────────────────────────────────────────────────────

func TestLookToolPathFindsNothingForNonsense(t *testing.T) {
	if _, found := lookToolPath("sloppiler-tool-that-does-not-exist"); found {
		t.Error("expected no match for a nonexistent tool")
	}
}

func TestWindowsNativeGNULinkerCandidatesAreHostGated(t *testing.T) {
	candidates := windowsNativeGNULinkerCandidates()
	if runtime.GOOS == "windows" {
		if len(candidates) == 0 {
			t.Error("a Windows host should probe the unprefixed PE-native linkers")
		}
		return
	}
	// On Linux and macOS a bare "ld" is the host ELF/Mach-O linker; probing it
	// for a PE link would silently produce an unusable artifact.
	if len(candidates) != 0 {
		t.Errorf("non-Windows hosts must not probe a bare ld, got %v", candidates)
	}
}

// ── MSVC linker dialect ───────────────────────────────────────────────────────

func TestMSVCLinkerArgs(t *testing.T) {
	args := msvcLinkerArgs("x.o", "hello.exe")
	joined := strings.Join(args, " ")
	for _, required := range []string{"/entry:_start", "/subsystem:console", "/out:hello.exe", "x.o", "kernel32.lib"} {
		if !strings.Contains(joined, required) {
			t.Errorf("msvcLinkerArgs missing %q, got: %s", required, joined)
		}
	}
	// GNU flag syntax would be silently misread by link.exe as an input file.
	if strings.Contains(joined, "--entry") || strings.Contains(joined, "-o ") {
		t.Errorf("msvcLinkerArgs must not use GNU ld flag syntax, got: %s", joined)
	}
}

// ── Install guidance ──────────────────────────────────────────────────────────

func TestInstallHintForIsHostSpecific(t *testing.T) {
	hint := installHintFor("nasm")
	if hint == "" {
		t.Fatal("expected an install hint for nasm")
	}
	if runtime.GOOS == "windows" && strings.Contains(hint, "pacman") {
		t.Errorf("Windows users cannot run pacman, got: %s", hint)
	}
	if runtime.GOOS == "linux" && strings.Contains(hint, "winget") {
		t.Errorf("Linux users cannot run winget, got: %s", hint)
	}
}

func TestInstallHintForUnknownComponent(t *testing.T) {
	if hint := installHintFor("not-a-real-component"); hint != "" {
		t.Errorf("expected no hint for an unknown component, got: %s", hint)
	}
}

func TestMissingToolErrorNamesTheTool(t *testing.T) {
	err := missingToolError("nasm", "nasm")
	if err == nil || !strings.Contains(err.Error(), "nasm") {
		t.Errorf("expected the tool name in the error, got: %v", err)
	}
}
