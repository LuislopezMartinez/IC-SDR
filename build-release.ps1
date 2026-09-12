[CmdletBinding()]
param(
    [switch]$SkipTests
)

$ErrorActionPreference = 'Stop'
$projectRoot = [System.IO.Path]::GetFullPath($PSScriptRoot)
$distRoot = [System.IO.Path]::GetFullPath((Join-Path $projectRoot 'dist\IC-SDR-Go'))
$expectedDist = [System.IO.Path]::GetFullPath((Join-Path $projectRoot 'dist\IC-SDR-Go'))
$digitalVoiceRuntime = Join-Path $projectRoot 'ORIGEN\IC_SDR\tools\digital_voice\runtime'
$digitalVoiceManifestPath = Join-Path $digitalVoiceRuntime 'runtime-version.json'

if ($distRoot -ne $expectedDist -or -not $distRoot.StartsWith($projectRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Unsafe distribution path: $distRoot"
}

# Keep the external runtime and its DLL set versioned as one unit. This catches
# accidental mixes of an updated executable with stale mbe/codec libraries.
if (-not (Test-Path -LiteralPath $digitalVoiceManifestPath)) {
    throw "Missing DSD-neo manifest: $digitalVoiceManifestPath"
}
$digitalVoiceManifest = Get-Content -LiteralPath $digitalVoiceManifestPath -Raw | ConvertFrom-Json
if ($digitalVoiceManifest.version -ne '2.9.0') {
    throw "Unsupported DSD-neo version: $($digitalVoiceManifest.version). Expected 2.9.0."
}
$digitalVoiceExe = Join-Path $digitalVoiceRuntime 'bin\dsd-neo.exe'
foreach ($required in @($digitalVoiceExe, (Join-Path $digitalVoiceRuntime 'bin\mbe-neo.dll'), (Join-Path $digitalVoiceRuntime 'bin\codec2.dll'), (Join-Path $digitalVoiceRuntime 'bin\libexpat.dll'))) {
    if (-not (Test-Path -LiteralPath $required)) { throw "Missing required DSD-neo 2.9.0 component: $required" }
}
$digitalVoiceExeHash = (Get-FileHash -LiteralPath $digitalVoiceExe -Algorithm SHA256).Hash.ToLowerInvariant()
if ($digitalVoiceExeHash -ne $digitalVoiceManifest.executableSha256.ToLowerInvariant()) {
    throw "DSD-neo executable does not match the manifest: $digitalVoiceExeHash"
}

# Windows locks the executable, runtime DLLs and startup.log while IC-SDR is
# running. Detect that state before copying user data or deleting anything so
# a release attempt cannot leave a half-rebuilt portable directory.
$runningFromDist = @(Get-Process -ErrorAction SilentlyContinue | Where-Object {
    try {
        $_.Path -and [System.IO.Path]::GetFullPath($_.Path).StartsWith($distRoot, [System.StringComparison]::OrdinalIgnoreCase)
    } catch {
        $false
    }
})
if ($runningFromDist.Count -gt 0) {
    $processList = ($runningFromDist | ForEach-Object { "{0} (PID {1})" -f $_.ProcessName, $_.Id }) -join ', '
    throw "IC-SDR is still open and Windows has locked the distribution: $processList. Close the app and run build-release.ps1 again."
}

$mutableData = @('cache', 'captures', 'config', 'exports', 'logs', 'recordings')
foreach ($name in $mutableData) {
    $existing = Join-Path $distRoot (Join-Path 'DATA' $name)
    if (Test-Path -LiteralPath $existing) {
        $saved = Join-Path $projectRoot (Join-Path 'DATA' $name)
        New-Item -ItemType Directory -Path $saved -Force | Out-Null
        Get-ChildItem -LiteralPath $existing -Force | Copy-Item -Destination $saved -Recurse -Force
    }
}

if (Test-Path -LiteralPath $distRoot) {
    Remove-Item -LiteralPath $distRoot -Recurse -Force
}
New-Item -ItemType Directory -Path $distRoot -Force | Out-Null

if (-not $SkipTests) {
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Tests failed.' }
}

$exePath = Join-Path $distRoot 'IC-SDR-Go.exe'
& go build -trimpath -ldflags '-s -w -H=windowsgui' -o $exePath .
if ($LASTEXITCODE -ne 0) { throw 'Could not compile IC-SDR-Go.exe.' }

$copies = @(
    @{ Source = 'ORIGEN\IC_SDR\runtime\windows-x64'; Destination = 'DATA\runtime\windows-x64' },
    @{ Source = 'ORIGEN\IC_SDR\tools\dmr\runtime'; Destination = 'DATA\tools\dmr\runtime' },
    @{ Source = 'ORIGEN\IC_SDR\tools\digital_voice\runtime'; Destination = 'DATA\tools\digital_voice\runtime' },
    @{ Source = 'ORIGEN\IC_SDR\tools\rtl_433\runtime'; Destination = 'DATA\tools\rtl_433\runtime' },
    @{ Source = 'ORIGEN\IC_SDR\tools\radiosonde\runtime'; Destination = 'DATA\tools\radiosonde\runtime' },
	@{ Source = 'ORIGEN\IC_SDR\tools\ais\runtime'; Destination = 'DATA\tools\ais\runtime' },
	@{ Source = 'ORIGEN\IC_SDR\tools\aircraft\runtime'; Destination = 'DATA\tools\aircraft\runtime' },
    @{ Source = 'ORIGEN\IC_SDR\tools\aprs\runtime'; Destination = 'DATA\tools\aprs\runtime' },
    @{ Source = 'ORIGEN\IC_SDR\tools\aprs\config'; Destination = 'DATA\tools\aprs\config' },
	@{ Source = 'ORIGEN\IC_SDR\tools\sstv\runtime'; Destination = 'DATA\tools\sstv\runtime' },
	@{ Source = 'ORIGEN\IC_SDR\tools\tetra\runtime'; Destination = 'DATA\tools\tetra\runtime' },
    @{ Source = 'ORIGEN\IC_SDR\data\ic-sdr-settings.json'; Destination = 'DATA\data\ic-sdr-settings.json' }
)

foreach ($copy in $copies) {
    $source = Join-Path $projectRoot $copy.Source
    $destination = Join-Path $distRoot $copy.Destination
    if (-not (Test-Path -LiteralPath $source)) { throw "Missing required resource: $source" }
    $parent = if ((Get-Item -LiteralPath $source).PSIsContainer) { Split-Path $destination -Parent } else { Split-Path $destination -Parent }
    New-Item -ItemType Directory -Path $parent -Force | Out-Null
    Copy-Item -LiteralPath $source -Destination $destination -Recurse -Force
}

# SoapySDR and rtlsdrSupport are built with MSVC. Bundling their runtime keeps
# the folder genuinely portable on Windows installations without VC++ installed.
$runtimeBin = Join-Path $distRoot 'DATA\runtime\windows-x64\bin'
$vcRuntimeFiles = @('MSVCP140.dll', 'VCRUNTIME140.dll', 'VCRUNTIME140_1.dll')
foreach ($name in $vcRuntimeFiles) {
    $bundled = Join-Path $projectRoot (Join-Path 'ORIGEN\IC_SDR\runtime\windows-x64\bin' $name)
    if (-not (Test-Path -LiteralPath $bundled)) {
        $systemCopy = Join-Path $env:SystemRoot (Join-Path 'System32' $name)
        if (-not (Test-Path -LiteralPath $systemCopy)) {
            throw "Missing $name. Install the Microsoft Visual C++ Redistributable x64 before creating the portable package."
        }
        Copy-Item -LiteralPath $systemCopy -Destination $bundled -Force
    }
    Copy-Item -LiteralPath $bundled -Destination (Join-Path $runtimeBin $name) -Force
}

foreach ($name in $mutableData) {
    $saved = Join-Path $projectRoot (Join-Path 'DATA' $name)
    if (Test-Path -LiteralPath $saved) {
        $destination = Join-Path $distRoot (Join-Path 'DATA' $name)
        New-Item -ItemType Directory -Path $destination -Force | Out-Null
        Get-ChildItem -LiteralPath $saved -Force | Copy-Item -Destination $destination -Recurse -Force
    }
}

Copy-Item -LiteralPath (Join-Path $projectRoot 'DISTRIBUTION.md') -Destination (Join-Path $distRoot 'README.txt')

$size = (Get-ChildItem -LiteralPath $distRoot -Recurse -File | Measure-Object Length -Sum).Sum
Write-Host ("Distribution ready: {0} ({1:N1} MB)" -f $distRoot, ($size / 1MB)) -ForegroundColor Green
