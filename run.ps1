#!/usr/bin/env pwsh
#Requires -Version 5.1
<#
.SYNOPSIS
    Builds sloppiler, ensures a local Ollama instance is serving the requested
    model, and compiles a source file with it. The Windows-native counterpart to
    run.sh.

.PARAMETER Source
    Source file to compile.

.PARAMETER Output
    Output binary path. Defaults to a.exe on Windows, a.out elsewhere.

.EXAMPLE
    ./run.ps1 testfiles/main.c

.EXAMPLE
    $env:MODEL = "codellama"; ./run.ps1 testfiles/main.c hello
#>
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string]$Source,

    [Parameter(Position = 1)]
    [string]$Output
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$IsWindowsHost = $IsWindows -or ($PSVersionTable.PSVersion.Major -le 5)
$SloppilerBinary = if ($IsWindowsHost) { './sloppiler.exe' } else { './sloppiler' }

if (-not $Output) {
    $Output = if ($IsWindowsHost) { 'a.exe' } else { 'a.out' }
}

$Model = if ($env:MODEL) { $env:MODEL } else { 'phi3' }
$OllamaRoot = 'http://localhost:11434'

function Test-OllamaReachable {
    try {
        Invoke-WebRequest -Uri $OllamaRoot -UseBasicParsing -TimeoutSec 2 | Out-Null
        return $true
    } catch {
        return $false
    }
}

function Assert-CommandAvailable([string]$Name, [string]$InstallHint) {
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        Write-Error "[error] '$Name' is not on PATH. $InstallHint"
    }
}

if (-not $Source) {
    Write-Host "Usage: ./run.ps1 <source-file> [output-binary]"
    Write-Host "       `$env:MODEL = 'codellama'; ./run.ps1 main.c hello"
    exit 1
}

if (-not (Test-Path -LiteralPath $Source)) {
    Write-Error "[error] source file not found: $Source"
}

Assert-CommandAvailable 'go' 'Install it with: winget install GoLang.Go'

# Build sloppiler if needed
if (-not (Test-Path -LiteralPath $SloppilerBinary)) {
    Write-Host "[setup] building sloppiler..."
    & go build -o $SloppilerBinary .
    if ($LASTEXITCODE -ne 0) { Write-Error "[error] go build failed" }
}

Assert-CommandAvailable 'ollama' 'Install it from https://ollama.com/download, or via: winget install Ollama.Ollama'

# Start ollama in the background if it is not already serving
if (-not (Test-OllamaReachable)) {
    Write-Host "[setup] starting ollama..."
    Start-Process -FilePath 'ollama' -ArgumentList 'serve' -WindowStyle Hidden | Out-Null
    $ready = $false
    foreach ($attempt in 1..20) {
        Start-Sleep -Milliseconds 500
        if (Test-OllamaReachable) { $ready = $true; break }
    }
    if (-not $ready) { Write-Error "[error] ollama did not start in time" }
}

# Pull the model if it is not present locally
$installedModels = & ollama list 2>$null
if (-not ($installedModels | Select-String -SimpleMatch -Pattern $Model -Quiet)) {
    Write-Host "[setup] pulling model $Model..."
    & ollama pull $Model
    if ($LASTEXITCODE -ne 0) { Write-Error "[error] could not pull model $Model" }
}

Write-Host "[run] sloppiling $Source -> $Output with model $Model"
& $SloppilerBinary -model $Model -o $Output $Source
exit $LASTEXITCODE
