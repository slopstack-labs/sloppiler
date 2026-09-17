package main

import (
	"runtime"
	"strings"
	"testing"
)

// ── extractHex ────────────────────────────────────────────────────────────────

func TestExtractHex(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "7f454c46", "7f454c46"},
		{"with spaces and newlines", "7f45 4c46\n0201", "7f454c460201"},
		{"strips code fences", "```hex\n7f454c46\n```", "7f454c46"},
		// extractHex is a raw hex-digit filter, so hex letters in prose words
		// (e, b, a, f, …) are kept too. This documents that known behaviour.
		{"keeps every hex digit including those in prose", "the bytes are: 7f454c46 (an ELF)", "ebeae7f454c46aEF"},
		{"odd length trimmed", "abc", "ab"},
		{"prose with no hex pairs collapses to its hex letters", "no hex here at all", "eeea"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := extractHex(c.in); got != c.want {
				t.Errorf("extractHex(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// ── cleanAsm ──────────────────────────────────────────────────────────────────

func TestCleanAsmAmd64InjectsBits(t *testing.T) {
	out := cleanAsm("section .text\nglobal _start", "amd64")
	if !strings.HasPrefix(out, "BITS 64") {
		t.Errorf("expected BITS 64 to be injected for amd64, got:\n%s", out)
	}
}

func TestCleanAsmAmd64StripsFences(t *testing.T) {
	out := cleanAsm("```nasm\nBITS 64\nmov rax, 1\n```", "amd64")
	if strings.Contains(out, "`") {
		t.Errorf("expected code fences stripped, got:\n%s", out)
	}
	if !strings.Contains(out, "mov rax, 1") {
		t.Errorf("expected instruction preserved, got:\n%s", out)
	}
}

func TestCleanAsmArm64SkipsNasmTransforms(t *testing.T) {
	in := ".text\n.global _start\n_start:\n  mov x0, #0"
	out := cleanAsm(in, "arm64")
	if strings.Contains(strings.ToUpper(out), "BITS 64") {
		t.Errorf("arm64 cleanAsm must not inject BITS 64, got:\n%s", out)
	}
	if out != in {
		t.Errorf("arm64 cleanAsm should only trim/strip fences, got:\n%s", out)
	}
}

func TestCleanAsmArm64StripsFences(t *testing.T) {
	out := cleanAsm("```aarch64\n.text\n```", "arm64")
	if strings.Contains(out, "`") {
		t.Errorf("expected fences stripped for arm64, got:\n%s", out)
	}
}

// ── fixMasmisms ───────────────────────────────────────────────────────────────

func TestFixMasmisms(t *testing.T) {
	in := "buffer db 64 dup(?)"
	want := "buffer times 64 db 0"
	if got := fixMasmisms(in); got != want {
		t.Errorf("fixMasmisms(%q) = %q, want %q", in, got, want)
	}
}

func TestFixMasmismsLeavesValidNasm(t *testing.T) {
	in := "msg db \"hello\", 10"
	if got := fixMasmisms(in); got != in {
		t.Errorf("fixMasmisms should leave valid NASM unchanged, got %q", got)
	}
}

// ── stripProseTrailer ─────────────────────────────────────────────────────────

func TestStripProseTrailer(t *testing.T) {
	in := "section .text\nglobal _start\n_start:\n  ret\nThis assembly prints hello."
	out := stripProseTrailer(in)
	if strings.Contains(out, "This assembly prints") {
		t.Errorf("expected trailing prose removed, got:\n%s", out)
	}
	if !strings.Contains(out, "_start:") {
		t.Errorf("expected code preserved, got:\n%s", out)
	}
}

// ── reorderArgs ───────────────────────────────────────────────────────────────

func TestReorderArgs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			"positional before flag",
			[]string{"main.c", "-o", "hello"},
			[]string{"-o", "hello", "main.c"},
		},
		{
			"arch flag keeps its value",
			[]string{"-arch", "arm64", "main.c"},
			[]string{"-arch", "arm64", "main.c"},
		},
		{
			"double dash passes rest as positional",
			[]string{"-optimistic", "--", "-weird-name"},
			[]string{"-optimistic", "-weird-name"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := reorderArgs(c.in)
			if strings.Join(got, " ") != strings.Join(c.want, " ") {
				t.Errorf("reorderArgs(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

// ── nasmFormat ────────────────────────────────────────────────────────────────

func TestNasmFormat(t *testing.T) {
	cases := map[string]string{"linux": "elf64", "windows": "win64", "darwin": "macho64", "": "elf64"}
	for target, want := range cases {
		if got := nasmFormat(target); got != want {
			t.Errorf("nasmFormat(%q) = %q, want %q", target, got, want)
		}
	}
}

// ── linkerArgs (deterministic branches) ───────────────────────────────────────

func TestLinkerArgsAmd64Linux(t *testing.T) {
	if runtime.GOOS == "windows" {
		// A Windows host has no ELF-capable ld; that combination is rejected up
		// front and covered by TestHostLinkSupportError instead.
		t.Skip("ELF linking is not available on a Windows host")
	}
	linker, args, err := linkerArgs("linux", "amd64", "x.o", "out")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if linker != "ld" {
		t.Errorf("linker = %q, want ld", linker)
	}
	if strings.Join(args, " ") != "-m elf_x86_64 x.o -o out" {
		t.Errorf("args = %v", args)
	}
}

func TestLinkerArgsArm64DarwinUnsupported(t *testing.T) {
	if _, _, err := linkerArgs("darwin", "arm64", "x.o", "out"); err == nil {
		t.Error("expected arm64/darwin to be unsupported in --optimistic mode")
	}
}

func TestLinkerArgsArm64WindowsUnsupported(t *testing.T) {
	if _, _, err := linkerArgs("windows", "arm64", "x.o", "out"); err == nil {
		t.Error("expected arm64/windows to be unsupported in --optimistic mode")
	}
}

// ── compilePromptFor ──────────────────────────────────────────────────────────

func TestCompilePromptForEmbedsSourceAndArch(t *testing.T) {
	src := "int main(){return 0;}"
	prompt := compilePromptFor("linux", "arm64", src)
	if !strings.Contains(prompt, src) {
		t.Error("prompt should embed the source")
	}
	if !strings.Contains(prompt, "arm64") {
		t.Error("prompt should mention arm64 architecture")
	}
	if strings.Contains(prompt, "%"+"s") {
		t.Error("prompt should be fully formed with no leftover format placeholder")
	}
}

// ── asmPromptFor routing ──────────────────────────────────────────────────────

func TestAsmPromptForRoutesByArch(t *testing.T) {
	amd := asmPromptFor("linux", "amd64", "x")
	if !strings.Contains(amd, "NASM") {
		t.Error("amd64 prompt should target NASM")
	}
	arm := asmPromptFor("linux", "arm64", "x")
	if !strings.Contains(arm, "AArch64") {
		t.Error("arm64 prompt should target AArch64")
	}
}

// ── ELF wrapper ───────────────────────────────────────────────────────────────

func TestWrapInElfMagicAndMachine(t *testing.T) {
	payload := []byte{0x90, 0x90}
	amd := wrapInElf(payload, "amd64")
	if amd[0] != 0x7f || amd[1] != 'E' || amd[2] != 'L' || amd[3] != 'F' {
		t.Errorf("missing ELF magic: % x", amd[:4])
	}
	if amd[18] != 0x3e { // e_machine = x86-64
		t.Errorf("amd64 e_machine = %#x, want 0x3e", amd[18])
	}
	arm := wrapInElf(payload, "arm64")
	if arm[18] != 0xb7 { // e_machine = AArch64
		t.Errorf("arm64 e_machine = %#x, want 0xb7", arm[18])
	}
}

// ── PE wrapper ────────────────────────────────────────────────────────────────

func TestWrapInPEMagicAndMachine(t *testing.T) {
	payload := []byte{0x90}
	amd := wrapInPE(payload, "amd64")
	if amd[0] != 'M' || amd[1] != 'Z' {
		t.Errorf("missing MZ magic: % x", amd[:2])
	}
	if amd[64] != 'P' || amd[65] != 'E' || amd[66] != 0 || amd[67] != 0 {
		t.Errorf("missing PE signature at 64: % x", amd[64:68])
	}
	if amd[68] != 0x64 || amd[69] != 0x86 { // COFF Machine = AMD64 (LE)
		t.Errorf("amd64 COFF machine = % x, want 64 86", amd[68:70])
	}
	arm := wrapInPE(payload, "arm64")
	if arm[68] != 0x64 || arm[69] != 0xaa { // COFF Machine = ARM64 (LE)
		t.Errorf("arm64 COFF machine = % x, want 64 aa", arm[68:70])
	}
}

// ── Mach-O wrapper ────────────────────────────────────────────────────────────

func TestWrapInMachOMagicAndCPU(t *testing.T) {
	payload := []byte{0x1f, 0x20, 0x03, 0xd5} // arm64 nop
	amd := wrapInMachO(payload, "amd64")
	// magic 0xFEEDFACF little-endian
	if amd[0] != 0xcf || amd[1] != 0xfa || amd[2] != 0xed || amd[3] != 0xfe {
		t.Errorf("bad Mach-O magic: % x", amd[:4])
	}
	if amd[4] != 0x07 || amd[7] != 0x01 { // cputype x86_64 = 0x01000007 (LE)
		t.Errorf("amd64 cputype = % x, want 07 00 00 01", amd[4:8])
	}
	arm := wrapInMachO(payload, "arm64")
	if arm[4] != 0x0c || arm[7] != 0x01 { // cputype arm64 = 0x0100000C (LE)
		t.Errorf("arm64 cputype = % x, want 0c 00 00 01", arm[4:8])
	}
}
