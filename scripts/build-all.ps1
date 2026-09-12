# Cross-compile IC-SDR for Windows, Linux, and macOS (amd64 + arm64).
# Requires Go with CGO_ENABLED=0 (raylib-go purego / embedded raylib).

param(
    [string]$OutDir = "dist\multi"
)

$ErrorActionPreference = "Stop"
$env:CGO_ENABLED = "0"

New-Item -ItemType Directory -Path $OutDir -Force | Out-Null

$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; Ext = ".exe"; Extra = "-H=windowsgui" }
    @{ GOOS = "windows"; GOARCH = "arm64"; Ext = ".exe"; Extra = "-H=windowsgui" }
    @{ GOOS = "linux";   GOARCH = "amd64"; Ext = "";     Extra = "" }
    @{ GOOS = "linux";   GOARCH = "arm64"; Ext = "";     Extra = "" }
    @{ GOOS = "darwin";  GOARCH = "amd64"; Ext = "";     Extra = "" }
    @{ GOOS = "darwin";  GOARCH = "arm64"; Ext = "";     Extra = "" }
)

foreach ($target in $targets) {
    $name = "IC-SDR-$($target.GOOS)-$($target.GOARCH)$($target.Ext)"
    $out = Join-Path $OutDir $name
    Write-Host "Building $name"
    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    $ldflags = "-s -w"
    if ($target.Extra) { $ldflags = "$ldflags $($target.Extra)" }
    & go build -trimpath -ldflags $ldflags -o $out .
    if ($LASTEXITCODE -ne 0) { throw "build failed: $name" }
}

Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue

Get-ChildItem $OutDir | Format-Table Name, Length, LastWriteTime
