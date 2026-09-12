# Build libtetradec and install four timeslot copies beside the IC-SDR data tree.
# Usage: pwsh -File scripts/build-libtetradec.ps1

param(
    [string]$Source = "third_party/libtetradec",
    [string]$BuildDir = "build/libtetradec",
    [string]$DestDir = "DATA/tools/tetra/runtime/bin"
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

if (-not (Test-Path (Join-Path $Source "CMakeLists.txt"))) {
    throw "libtetradec sources missing at $Source. Run: git submodule update --init --recursive"
}

New-Item -ItemType Directory -Path $BuildDir -Force | Out-Null
New-Item -ItemType Directory -Path $DestDir -Force | Out-Null

cmake -S $Source -B $BuildDir -DCMAKE_BUILD_TYPE=Release
if ($LASTEXITCODE -ne 0) { throw "cmake configure failed" }
cmake --build $BuildDir --config Release
if ($LASTEXITCODE -ne 0) { throw "cmake build failed" }

$lib = Get-ChildItem -Path $BuildDir -Recurse -File |
    Where-Object { $_.Name -match '^libtetradec\.(dll|so|dylib)$' } |
    Select-Object -First 1
if (-not $lib) {
    throw "built libtetradec was not found under $BuildDir"
}

Copy-Item $lib.FullName (Join-Path $DestDir $lib.Name) -Force
$stem = [IO.Path]::GetFileNameWithoutExtension($lib.Name)
$ext = $lib.Extension.TrimStart('.')
foreach ($slot in 2, 3, 4) {
    Copy-Item $lib.FullName (Join-Path $DestDir "$stem-ts$slot.$ext") -Force
}

Get-ChildItem $DestDir | Format-Table Name, Length
