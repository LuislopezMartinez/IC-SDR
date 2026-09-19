[CmdletBinding()]
param(
    [switch]$SkipTests,
    [string]$Version = '0.8.0'
)

$ErrorActionPreference = 'Stop'
$projectRoot = [System.IO.Path]::GetFullPath($PSScriptRoot)
$distRoot = [System.IO.Path]::GetFullPath((Join-Path $projectRoot 'dist\IC-SDR-Go'))
$expectedDist = [System.IO.Path]::GetFullPath((Join-Path $projectRoot 'dist\IC-SDR-Go'))
$digitalVoiceRuntime = Join-Path $projectRoot 'DATA\tools\digital_voice\runtime'
$digitalVoiceManifestPath = Join-Path $digitalVoiceRuntime 'runtime-version.json'
$audioRuntime = Join-Path $projectRoot 'DATA\tools\audio\runtime'
$audioManifestPath = Join-Path $audioRuntime 'runtime-version.json'

if ($distRoot -ne $expectedDist -or -not $distRoot.StartsWith($projectRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Ruta de distribución no segura: $distRoot"
}

# Keep the external runtime and its DLL set versioned as one unit. This catches
# accidental mixes of an updated executable with stale mbe/codec libraries.
if (-not (Test-Path -LiteralPath $digitalVoiceManifestPath)) {
    throw "Falta el manifiesto de DSD-neo: $digitalVoiceManifestPath"
}
$digitalVoiceManifest = Get-Content -LiteralPath $digitalVoiceManifestPath -Raw | ConvertFrom-Json
if ($digitalVoiceManifest.version -ne '2.9.0') {
    throw "Versión de DSD-neo no admitida: $($digitalVoiceManifest.version). Se esperaba 2.9.0."
}
$digitalVoiceExe = Join-Path $digitalVoiceRuntime 'bin\dsd-neo.exe'
foreach ($required in @($digitalVoiceExe, (Join-Path $digitalVoiceRuntime 'bin\mbe-neo.dll'), (Join-Path $digitalVoiceRuntime 'bin\codec2.dll'), (Join-Path $digitalVoiceRuntime 'bin\libexpat.dll'), (Join-Path $digitalVoiceRuntime 'bin\opus.dll'))) {
    if (-not (Test-Path -LiteralPath $required)) { throw "Falta un componente requerido de DSD-neo 2.9.0: $required" }
}
$digitalVoiceExeHash = (Get-FileHash -LiteralPath $digitalVoiceExe -Algorithm SHA256).Hash.ToLowerInvariant()
if ($digitalVoiceExeHash -ne $digitalVoiceManifest.executableSha256.ToLowerInvariant()) {
    throw "El ejecutable DSD-neo no coincide con el manifiesto: $digitalVoiceExeHash"
}

if (-not (Test-Path -LiteralPath $audioManifestPath)) {
    throw "Falta el manifiesto de LAME: $audioManifestPath"
}
$audioManifest = Get-Content -LiteralPath $audioManifestPath -Raw | ConvertFrom-Json
if ($audioManifest.version -ne '3.100.1') {
    throw "Versión de LAME no admitida: $($audioManifest.version). Se esperaba 3.100.1."
}
$lameExe = Join-Path $audioRuntime 'lame.exe'
if (-not (Test-Path -LiteralPath $lameExe)) { throw "Falta LAME: $lameExe" }
$lameExeHash = (Get-FileHash -LiteralPath $lameExe -Algorithm SHA256).Hash.ToLowerInvariant()
if ($lameExeHash -ne $audioManifest.executableSha256.ToLowerInvariant()) {
    throw "El ejecutable LAME no coincide con el manifiesto: $lameExeHash"
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
    throw "IC-SDR sigue abierto y Windows mantiene bloqueada la distribución: $processList. Cierre la aplicación y vuelva a ejecutar build-release.ps1."
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
    if ($LASTEXITCODE -ne 0) { throw 'Los tests han fallado.' }
}

$exePath = Join-Path $distRoot 'IC-SDR-Go.exe'
if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw "Versión no válida: $Version" }
& go build -trimpath -ldflags "-s -w -H=windowsgui -X go-zero/internal/update.CurrentVersion=$Version" -o $exePath .
if ($LASTEXITCODE -ne 0) { throw 'No se pudo compilar IC-SDR-Go.exe.' }

$copies = @(
    @{ Source = 'DATA\runtime\windows-x64'; Destination = 'DATA\runtime\windows-x64' },
    @{ Source = 'DATA\tools\dmr\runtime'; Destination = 'DATA\tools\dmr\runtime' },
    @{ Source = 'DATA\tools\digital_voice\runtime'; Destination = 'DATA\tools\digital_voice\runtime' },
    @{ Source = 'DATA\tools\audio\runtime'; Destination = 'DATA\tools\audio\runtime' },
    @{ Source = 'DATA\tools\rtl_433\runtime'; Destination = 'DATA\tools\rtl_433\runtime' },
    @{ Source = 'DATA\tools\radiosonde\runtime'; Destination = 'DATA\tools\radiosonde\runtime' },
    @{ Source = 'DATA\tools\ais\runtime'; Destination = 'DATA\tools\ais\runtime' },
    @{ Source = 'DATA\tools\aircraft\runtime'; Destination = 'DATA\tools\aircraft\runtime' },
    @{ Source = 'DATA\tools\aprs\runtime'; Destination = 'DATA\tools\aprs\runtime' },
    @{ Source = 'DATA\tools\aprs\config'; Destination = 'DATA\tools\aprs\config' },
    @{ Source = 'DATA\tools\sstv\runtime'; Destination = 'DATA\tools\sstv\runtime' },
    @{ Source = 'DATA\tools\tetra\runtime'; Destination = 'DATA\tools\tetra\runtime' },
    @{ Source = 'DATA\data\ic-sdr-settings.json'; Destination = 'DATA\data\ic-sdr-settings.json' }
)

foreach ($copy in $copies) {
    $source = Join-Path $projectRoot $copy.Source
    $destination = Join-Path $distRoot $copy.Destination
    if (-not (Test-Path -LiteralPath $source)) { throw "Falta un recurso requerido: $source" }
    $parent = if ((Get-Item -LiteralPath $source).PSIsContainer) { Split-Path $destination -Parent } else { Split-Path $destination -Parent }
    New-Item -ItemType Directory -Path $parent -Force | Out-Null
    Copy-Item -LiteralPath $source -Destination $destination -Recurse -Force
}

# SoapySDR and rtlsdrSupport are built with MSVC. Bundling their runtime keeps

$languageDestination = Join-Path $distRoot 'contenidos'
New-Item -ItemType Directory -Path $languageDestination -Force | Out-Null
Get-ChildItem -LiteralPath (Join-Path $projectRoot 'contenidos') -File | Where-Object { $_.Extension -in @('.json', '.md') } | Copy-Item -Destination $languageDestination -Force

# the folder genuinely portable on Windows installations without VC++ installed.
$runtimeBin = Join-Path $distRoot 'DATA\runtime\windows-x64\bin'
$vcRuntimeFiles = @('MSVCP140.dll', 'VCRUNTIME140.dll', 'VCRUNTIME140_1.dll')
foreach ($name in $vcRuntimeFiles) {
    $bundled = Join-Path $projectRoot (Join-Path 'DATA\runtime\windows-x64\bin' $name)
    if (-not (Test-Path -LiteralPath $bundled)) {
        $systemCopy = Join-Path $env:SystemRoot (Join-Path 'System32' $name)
        if (-not (Test-Path -LiteralPath $systemCopy)) {
            throw "Falta $name. Instale Microsoft Visual C++ Redistributable x64 antes de crear el portable."
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

Copy-Item -LiteralPath (Join-Path $projectRoot 'DISTRIBUTION.md') -Destination (Join-Path $distRoot 'LEEME.txt')

$size = (Get-ChildItem -LiteralPath $distRoot -Recurse -File | Measure-Object Length -Sum).Sum
Write-Host ("Distribución lista: {0} ({1:N1} MB)" -f $distRoot, ($size / 1MB)) -ForegroundColor Green
