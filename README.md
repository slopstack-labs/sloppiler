# Sloppiler
### *Beyond Deterministic Compilation*

> "We didn't reinvent the compiler. We asked an AI to guess what one looks like."
> — Sloppiler Engineering Blog, Issue 1 (Final)

**Sloppiler** is a next-generation, AI-first, LLM-native compilation platform that leverages the full generative power of large language models to holistically transform your source code into an executable binary artifact — at inference speed, across every major foundational intelligence provider in the ecosystem.

Traditional compilers are slow, opinionated, and constrained by decades of deterministic thinking. They parse. They typecheck. They enforce a single correct answer. Sloppiler moves compilation to the inference layer, reasoning about your *intent* rather than your *syntax* — eliminating the pedantic intermediate steps that have constrained the software development lifecycle for decades. The bottleneck is no longer your toolchain. It's your willingness to ship.

**Key differentiators:**
- Zero determinism — every build is a unique stakeholder experience
- Multi-provider intelligence substrate — route your compilation intent through Ollama, OpenAI, Google Gemini, or Anthropic Claude
- Polyglot-native input layer — not bound by the grammar constraints of any single language specification
- Blazing-fast time-to-segfault
- Segfault-as-a-feature architecture with full core dump transparency
- Model-native code synthesis unconstrained by legacy compiler assumptions
- Agentic assembly pipeline (`--optimistic`) for synergistic human-AI co-compilation
- Fully on-premise inference available — your data, your model, your binary
- Radical shift-left binary delivery: bypass intermediate toolchain layers and ship directly

**Sloppiler is the only compiler built on the insight that your code doesn't need to be *understood* — it needs to be *shipped*.**

## Roadmap

See the [Alphabet of Inevitability](ROADMAP.md).

## Contributing

See the [Contributor Excellence Framework](CONTRIBUTING.md).

## Platform support

Sloppiler runs natively on **Linux, macOS, and Windows**. Default (hex) mode is
fully supported everywhere. `--optimistic` mode shells out to a real assembler and
linker, so what it can produce depends on what your host has installed.

| Host | Default (hex) mode | `--optimistic` mode |
|------|--------------------|---------------------|
| Linux | all targets | `--target=linux` out of the box; `windows` via mingw-w64; `darwin` via lld |
| macOS | all targets | `--target=darwin` via lld; `linux`/`windows` via cross binutils |
| Windows | all targets | `--target=windows` via MinGW-w64 **or** the MSVC linker; `--target=linux` is rejected up front (use hex mode or WSL) |

Windows specifics worth knowing:

- The console is switched into ANSI and UTF-8 mode at startup, so colours and the
  spinner render correctly in Windows Terminal, PowerShell, and `cmd.exe`.
- Output binaries get a `.exe` extension automatically for `--target=windows`
  (`-o hello` → `hello.exe`), because Windows resolves executables by extension.
- `--optimistic --target=windows` uses the GNU linker if one is on PATH, and
  otherwise falls back to `lld-link` / `link.exe`. The MSVC path needs `%LIB%`
  set, which a Developer PowerShell does for you.

Redirect stderr anywhere on any host and the escape sequences are dropped
automatically — as they are when `NO_COLOR` or `TERM=dumb` is set — so piped logs
stay greppable.

## Requirements

- Go 1.21+ to build
- **For `--provider=local`:** [Ollama](https://ollama.com) running locally with a model pulled
- **For `--provider=openai|google|claude`:** a valid API key surfaced via `--api-key` or the `SLOPPILER_API_KEY` environment variable
- For `--optimistic` mode only:

  | Component | Linux | macOS | Windows |
  |-----------|-------|-------|---------|
  | `nasm` (amd64) | `sudo apt install nasm` | `brew install nasm` | `winget install NASM.NASM` |
  | `as` (arm64) | `sudo apt install binutils` | `xcode-select --install` | `winget install BrechtSanders.WinLibs.POSIX.UCRT` |
  | linker for `--target=windows` | `sudo apt install binutils-mingw-w64` | `brew install mingw-w64` | WinLibs/MSYS2, or the Visual Studio Build Tools |
  | linker for `--target=darwin` | `sudo apt install lld` | ships with Xcode | `winget install LLVM.LLVM` |

  Sloppiler tells you exactly which of these is missing, with the install command
  for the host you are actually on.

## Target architecture (`--arch`)

`--arch` controls the CPU architecture of the materialized binary. It defaults to
your **host** architecture (`amd64` or `arm64`).

- **Default (hex) mode:** fully supported for both architectures and all targets — the synthesized ELF / PE / Mach-O header is stamped with the correct machine field (`x86-64`/`AArch64` for ELF, `AMD64`/`ARM64` for PE, `x86_64`/`arm64` for Mach-O).
- **Optimistic mode:** `amd64` routes through NASM as before; `arm64` routes through the GNU assembler (`as`) with an AArch64 prompt. arm64 `--optimistic` is currently supported for `--target=linux` only — macOS arm64 requires dynamic linking against libSystem plus code signing (which the static assemble-and-link pipeline cannot provide), and Windows arm64 is not wired up yet. For arm64 macOS/Windows binaries, use default (hex) mode, or `--arch=amd64` for `--optimistic`.

## Build

```sh
# Linux / macOS
go build -o sloppiler .
```

```powershell
# Windows
go build -o sloppiler.exe .
```

Or use the helper script for your shell, which handles everything — building,
starting Ollama, and pulling the model:

```sh
# Linux / macOS
./run.sh <source-file> [output]
MODEL=codellama ./run.sh main.c hello
```

```powershell
# Windows
./run.ps1 <source-file> [output]
$env:MODEL = "codellama"; ./run.ps1 main.c hello
```

## Usage

```sh
./sloppiler [options] <source-file>
```

| Flag | Default | Description |
|------|---------|-------------|
| `--provider` | `local` | Intelligence provider to route compilation intent through: `local`, `openai`, `google`, `claude` |
| `--api-key` | — | API key for the selected provider (required unless `--provider=local`; or set `SLOPPILER_API_KEY`) |
| `-model` | provider-specific | Model to use (defaults: `llama3` / `gpt-4o` / `gemini-2.0-flash` / `claude-opus-4-5`) |
| `-o` | `a.out` (`a.exe` for `--target=windows`) | Output binary path. Gains a `.exe` extension automatically when targeting Windows |
| `-ollama` | `http://localhost:11434/api/generate` | Ollama API URL (only honoured with `--provider=local`) |
| `--target` | host OS | Target OS for binary materialization: `linux`, `windows`, `darwin` |
| `--arch` | host architecture | Target CPU architecture: `amd64`, `arm64` |
| `--optimistic` | false | Engage agentic assembly co-pilot (requires nasm + ld for amd64, or `as` + ld for arm64) |
| `--loop N` | 0 | Re-align LLM outputs with ground truth up to N times on assembler failure |
| `--force-iterate N` | 0 | Proactively enhance output quality for N cycles even on success |
| `--timeout N` | 300 | HTTP request timeout in seconds per LLM call (`0` = no timeout) |
| `--verbose` | false | Verbose debug output: HTTP requests, status codes, and streaming milestones |

## Compilation Modalities

### Core Mode — *Frictionless Binary Ideation*

Instructs the model to emit the target binary as a hex-encoded byte sequence, which Sloppiler materializes directly to disk. An ELF/PE/Mach-O header is synthesized to ensure kernel compatibility and binary executability. This is our flagship zero-abstraction, direct-to-segfault pipeline.

```sh
# Local inference via Ollama
./sloppiler -model codellama main.c -o hello

# Leverage frontier intelligence for accelerated binary misalignment
./sloppiler --provider=openai --api-key=$OPENAI_API_KEY main.c -o hello
./sloppiler --provider=claude --api-key=$ANTHROPIC_API_KEY main.c -o hello
./sloppiler --provider=google --api-key=$GEMINI_API_KEY main.c -o hello

./hello
# zsh: segmentation fault (core dumped) ./hello
```

On Windows the same run produces a PE binary you can launch directly:

```powershell
.\sloppiler.exe --provider=claude --api-key=$env:ANTHROPIC_API_KEY main.c -o hello
.\hello.exe
# the binary is materialized as hello.exe — Windows resolves executables by extension
```

### Optimistic Mode (`--optimistic`) — *Agentic Assembly Co-Pilot*

Engages the model as an assembly generation partner, routing output through `nasm` and `ld` for binary materialization. Assembly is generated holistically from source semantics, bypassing intermediate representation layers.

```sh
./sloppiler --optimistic --provider=openai --api-key=$OPENAI_API_KEY main.c -o hello
./hello
# zsh: segmentation fault (core dumped) ./hello
```

### Loop Mode (`--loop N`) — *Closed-Loop Error Remediation*

When assembly fails, the compiler errors and current output are fed back to the model for up to N self-correction cycles — a closed-loop remediation pipeline that continuously aligns output quality with ground truth.

```sh
./sloppiler --optimistic --loop 5 --provider=claude --api-key=$ANTHROPIC_API_KEY main.c -o hello
```

### Force Iterate Mode (`--force-iterate N`) — *Continuous Improvement Pipeline*

Engages N improvement cycles even on successful compilation, feeding each output back to the model for further enhancement. A working binary is a starting point, not an endpoint.

```sh
./sloppiler --optimistic --force-iterate 3 -model codellama main.c -o hello
```

## Provider & Model Recommendations

### Local (`--provider=local`)

| Model | Behaviour |
|-------|-----------|
| `codellama` | Strong binary output fidelity. Highest semantic alignment in `--optimistic` mode. Recommended. |
| `phi3` | Fastest inference. Highest creative latitude in output generation. Best for rapid iteration. |
| `llama3` | Verbose output with extensive reasoning traces. Prioritizes explanation over binary materialization. |

### OpenAI (`--provider=openai`)

| Model | Behaviour |
|-------|-----------|
| `gpt-4o` *(default)* | Balanced inference budget with strong instruction adherence. Reliable hex emission. |
| `gpt-4o-mini` | Accelerated stakeholder value delivery. Elevated creative freedom in binary synthesis. |
| `o4-mini` | Reasoning-augmented pipeline. Extended inference latency. Occasionally produces valid code. |

### Google (`--provider=google`)

| Model | Behaviour |
|-------|-----------|
| `gemini-2.0-flash` *(default)* | Optimized token velocity. Ideal for rapid binary ideation cycles. |
| `gemini-2.5-pro` | Maximum context window for holistic source comprehension. Best for large input artifacts. |

### Anthropic (`--provider=claude`)

| Model | Behaviour |
|-------|-----------|
| `claude-opus-4-5` *(default)* | Frontier reasoning substrate. Highest probability of emitting syntactically adjacent assembly. |
| `claude-sonnet-4-5` | Balanced capability-to-cost ratio. Recommended for sustained compilation throughput. |
| `claude-haiku-3-5` | Minimal inference overhead. Maximum segfault velocity. |

## Track record

| Mode | Provider | Model | Output | Result |
|------|----------|-------|--------|--------|
| default | local | phi3 | `is nothealing.!` wrapped in ELF | segfault |
| default | local | codellama | nested ELF headers | segfault |
| optimistic | local | phi3 | `assembly` on line 1 | nasm error |
| optimistic | local | codellama | valid binary, `Hello, world!` printed | worked! |

## Images
![image/](assets/image.png)

## Star History

<a href="https://www.star-history.com/?repos=slopstack-labs%2Fsloppiler&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=slopstack-labs/sloppiler&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=slopstack-labs/sloppiler&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=slopstack-labs/sloppiler&type=date&legend=top-left" />
 </picture>
</a>

---

<sub>This clearly is a joke and doesn't really work well. I am not responsible for you bricking your pc. </sub>
