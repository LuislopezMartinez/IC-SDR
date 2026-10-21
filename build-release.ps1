[CmdletBinding()]
param(
    [switch]$SkipTests,
    [string]$Version = '0.19.0'
)

$ErrorActionPreference = 'Stop'
$projectRoot = [System.IO.Path]::GetFullPath($PSScriptRoot)
$distRoot = [System.IO.Path]::GetFullPath((Join-Path $projectRoot 'dist\IC-SDR-Go'))
$expectedDist = [System.IO.Path]::GetFullPath((Join-Path $projectRoot 'dist\IC-SDR-Go'))
$digitalVoiceRuntime = Join-Path $projectRoot 'DATA\tools\digital_voice\runtime'
$digitalVoiceManifestPath = Join-Path $digitalVoiceRuntime 'runtime-version.json'
$audioRuntime = Join-Path $projectRoot 'DATA\tools\audio\runtime'
$audioManifestPath = Join-Path $audioRuntime 'runtime-version.json'
$tetrapolNativeDLL = Join-Path $projectRoot 'DATA\tools\tetrapol\native\bin\lilipol_native_x64.dll'
$omniRigRuntime = Join-Path $projectRoot 'DATA\runtime\windows-x64\omnirig'
$omniRigManifestPath = Join-Path $omniRigRuntime 'runtime-version.json'
$voacapRuntime = Join-Path $projectRoot 'DATA\tools\voacap\runtime'
$voacapManifestPath = Join-Path $voacapRuntime 'runtime-version.json'
$dabManifestPath = Join-Path $projectRoot 'DATA\tools\dab\runtime-version.json'
$datvRuntime = Join-Path $projectRoot 'DATA\tools\datv\runtime'
$datvManifestPath = Join-Path $datvRuntime 'runtime-version.json'

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

if (-not (Test-Path -LiteralPath $tetrapolNativeDLL)) {
    throw "Falta el motor nativo TETRAPOL x64: $tetrapolNativeDLL. Ejecute runtime\tetrapol-native\build.ps1."
}

if (-not (Test-Path -LiteralPath $omniRigManifestPath)) {
    throw "Falta el manifiesto de OmniRig portable: $omniRigManifestPath"
}
$omniRigManifest = Get-Content -LiteralPath $omniRigManifestPath -Raw | ConvertFrom-Json
if ($omniRigManifest.version -ne '1.20') {
    throw "Versión de OmniRig no admitida: $($omniRigManifest.version). Se esperaba 1.20."
}
$omniRigExe = Join-Path $omniRigRuntime $omniRigManifest.executable
$omniRigProfiles = Join-Path $omniRigRuntime 'Rigs'
foreach ($required in @($omniRigExe, (Join-Path $omniRigRuntime 'LICENSE.txt'), $omniRigProfiles)) {
    if (-not (Test-Path -LiteralPath $required)) { throw "Falta un componente requerido de OmniRig: $required" }
}
if (@(Get-ChildItem -LiteralPath $omniRigProfiles -Filter '*.ini' -File).Count -eq 0) {
    throw "OmniRig portable no contiene perfiles de radio en $omniRigProfiles"
}
$omniRigExeHash = (Get-FileHash -LiteralPath $omniRigExe -Algorithm SHA256).Hash.ToLowerInvariant()
if ($omniRigExeHash -ne $omniRigManifest.executableSha256.ToLowerInvariant()) {
    throw "OmniRig.exe no coincide con el manifiesto: $omniRigExeHash"
}

if (-not (Test-Path -LiteralPath $voacapManifestPath)) {
    throw "Falta el manifiesto del motor VOACAP portable: $voacapManifestPath"
}
$voacapManifest = Get-Content -LiteralPath $voacapManifestPath -Raw | ConvertFrom-Json
if ($voacapManifest.version -ne '08.0121W') {
    throw "Versión de VOACAP no admitida: $($voacapManifest.version). Se esperaba 08.0121W."
}
$voacapExe = Join-Path $voacapRuntime $voacapManifest.executable
$voacapDll = Join-Path $voacapRuntime $voacapManifest.runtimeDll
foreach ($required in @($voacapExe, $voacapDll, (Join-Path $voacapRuntime $voacapManifest.notice), (Join-Path $voacapRuntime 'coeffs'), (Join-Path $voacapRuntime 'antennas'))) {
    if (-not (Test-Path -LiteralPath $required)) { throw "Falta un componente requerido de VOACAP: $required" }
}
$voacapExeHash = (Get-FileHash -LiteralPath $voacapExe -Algorithm SHA256).Hash.ToLowerInvariant()
$voacapDllHash = (Get-FileHash -LiteralPath $voacapDll -Algorithm SHA256).Hash.ToLowerInvariant()
if ($voacapExeHash -ne $voacapManifest.executableSha256.ToLowerInvariant()) {
    throw "Voacapw.exe no coincide con el manifiesto: $voacapExeHash"
}
if ($voacapDllHash -ne $voacapManifest.runtimeDllSha256.ToLowerInvariant()) {
    throw "SALFLIBC.DLL de VOACAP no coincide con el manifiesto: $voacapDllHash"
}

if (-not (Test-Path -LiteralPath $dabManifestPath)) {
    throw "Falta el manifiesto del runtime DAB: $dabManifestPath"
}
$dabManifest = Get-Content -LiteralPath $dabManifestPath -Raw | ConvertFrom-Json
if ($dabManifest.upstreamVersion -ne '2.7' -or $dabManifest.protocolVersion -ne 1) {
    throw "Versión del runtime DAB no admitida: welle.io $($dabManifest.upstreamVersion), protocolo $($dabManifest.protocolVersion)."
}
$dabExe = Join-Path (Split-Path $dabManifestPath -Parent) $dabManifest.executable
if (-not (Test-Path -LiteralPath $dabExe)) { throw "Falta el runtime DAB: $dabExe" }
$dabExeHash = (Get-FileHash -LiteralPath $dabExe -Algorithm SHA256).Hash.ToLowerInvariant()
if ($dabExeHash -ne $dabManifest.executableSha256.ToLowerInvariant()) {
    throw "El runtime DAB no coincide con el manifiesto: $dabExeHash"
}

if (-not (Test-Path -LiteralPath $datvManifestPath)) {
    throw "Falta el manifiesto del runtime DATV: $datvManifestPath"
}
$datvManifest = Get-Content -LiteralPath $datvManifestPath -Raw | ConvertFrom-Json
if ($datvManifest.leanCommit -ne 'c005645edfa0dfdc45ffab3a5e6297b21ede26bf' -or
    $datvManifest.ldpcCommit -ne '6ada4aac6d853835eeaefb7e12a0136481647b01' -or
    -not $datvManifest.dvbS2Extensions) {
    throw 'La versión del runtime DATV no coincide con la versión validada.'
}
$datvExe = Join-Path $datvRuntime $datvManifest.executable
$datvLDPC = Join-Path $datvRuntime $datvManifest.ldpcHelper
foreach ($required in @($datvExe, $datvLDPC, (Join-Path $datvRuntime 'source\leandvb-LICENSE.txt'), (Join-Path $datvRuntime 'source\ldpc-LICENSE.txt'))) {
    if (-not (Test-Path -LiteralPath $required)) { throw "Falta un componente requerido del runtime DATV: $required" }
}
if ((Get-FileHash -LiteralPath $datvExe -Algorithm SHA256).Hash.ToLowerInvariant() -ne $datvManifest.executableSha256.ToLowerInvariant()) {
    throw 'LeanDVB no coincide con el manifiesto DATV.'
}
if ((Get-FileHash -LiteralPath $datvLDPC -Algorithm SHA256).Hash.ToLowerInvariant() -ne $datvManifest.ldpcHelperSha256.ToLowerInvariant()) {
    throw 'ldpc_tool no coincide con el manifiesto DATV.'
}

# Windows locks the executable, runtime DLLs and startup.log while IC-SDR is
# running. Close only instances launched from this distribution before
# rebuilding it so a release attempt cannot leave a half-rebuilt portable
# directory.
$runningFromDist = @(Get-Process -ErrorAction SilentlyContinue | Where-Object {
    try {
        $_.Path -and [System.IO.Path]::GetFullPath($_.Path).StartsWith($distRoot, [System.StringComparison]::OrdinalIgnoreCase)
    } catch {
        $false
    }
})
if ($runningFromDist.Count -gt 0) {
    $processList = ($runningFromDist | ForEach-Object { "{0} (PID {1})" -f $_.ProcessName, $_.Id }) -join ', '
    Write-Host "Cerrando IC-SDR antes de compilar: $processList"

    foreach ($process in $runningFromDist) {
        if ($process.MainWindowHandle -ne 0) {
            [void]$process.CloseMainWindow()
        }
    }

    $deadline = [DateTime]::UtcNow.AddSeconds(3)
    do {
        Start-Sleep -Milliseconds 100
        $stillRunning = @($runningFromDist | Where-Object { Get-Process -Id $_.Id -ErrorAction SilentlyContinue })
    } while ($stillRunning.Count -gt 0 -and [DateTime]::UtcNow -lt $deadline)

    if ($stillRunning.Count -gt 0) {
        $forcedList = ($stillRunning | ForEach-Object { "{0} (PID {1})" -f $_.ProcessName, $_.Id }) -join ', '
        Write-Host "La instancia no respondió al cierre normal; forzando cierre: $forcedList"
        $stillRunning | Stop-Process -Force -ErrorAction Stop
    }

    $deadline = [DateTime]::UtcNow.AddSeconds(5)
    do {
        Start-Sleep -Milliseconds 100
        $lockedProcesses = @($runningFromDist | Where-Object { Get-Process -Id $_.Id -ErrorAction SilentlyContinue })
    } while ($lockedProcesses.Count -gt 0 -and [DateTime]::UtcNow -lt $deadline)
    if ($lockedProcesses.Count -gt 0) {
        $lockedList = ($lockedProcesses | ForEach-Object { "{0} (PID {1})" -f $_.ProcessName, $_.Id }) -join ', '
        throw "No se pudo cerrar IC-SDR antes de compilar: $lockedList."
    }
}

$mutableData = @('cache', 'captures', 'config', 'exports', 'logs', 'recordings')

if (Test-Path -LiteralPath $distRoot) {
    Remove-Item -LiteralPath $distRoot -Recurse -Force
}
New-Item -ItemType Directory -Path $distRoot -Force | Out-Null

if (-not $SkipTests) {
    & go test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Los tests han fallado.' }
}

$exePath = Join-Path $distRoot 'IC-SDR-Go.exe'
$updaterPath = Join-Path $distRoot 'IC-SDR-Updater.exe'
if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw "Versión no válida: $Version" }
$Revision = 'unknown'
$gitCommand = Get-Command git -ErrorAction SilentlyContinue
if ($null -ne $gitCommand) {
    $detectedRevision = (& $gitCommand.Source -C $projectRoot rev-parse --short=12 HEAD 2>$null)
    if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($detectedRevision)) {
        $Revision = $detectedRevision
    }
}
$Revision = $Revision.Trim()
$BuildDate = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
$applicationLdFlags = "-s -w -H=windowsgui -X go-zero/internal/buildinfo.Version=$Version -X go-zero/internal/buildinfo.Revision=$Revision -X go-zero/internal/buildinfo.BuildDate=$BuildDate"
& go build -trimpath -ldflags $applicationLdFlags -o $exePath .
if ($LASTEXITCODE -ne 0) { throw 'No se pudo compilar IC-SDR-Go.exe.' }
& go build -trimpath -ldflags "-s -w -H=windowsgui" -o $updaterPath ./cmd/updater
if ($LASTEXITCODE -ne 0) { throw 'No se pudo compilar IC-SDR-Updater.exe.' }

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
	@{ Source = 'DATA\tools\ft8\runtime'; Destination = 'DATA\tools\ft8\runtime' },
    @{ Source = 'DATA\tools\dab\runtime'; Destination = 'DATA\tools\dab\runtime' },
    @{ Source = 'DATA\tools\dab\runtime-version.json'; Destination = 'DATA\tools\dab\runtime-version.json' },
    @{ Source = 'DATA\tools\datv\runtime'; Destination = 'DATA\tools\datv\runtime' },
    @{ Source = 'DATA\tools\tetra\runtime'; Destination = 'DATA\tools\tetra\runtime' },
    @{ Source = 'DATA\tools\tetrapol\native'; Destination = 'DATA\tools\tetrapol\native' },
    @{ Source = 'DATA\tools\voacap\runtime'; Destination = 'DATA\tools\voacap\runtime' },
    @{ Source = 'DATA\config\feedback.json'; Destination = 'DATA\config\feedback.json' },
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

# A public package must never include a user's receiver settings, API tokens,
# logbook, captures or recordings.  Keep the writable structure empty; the
# application creates its defaults at first start and the local workspace data
# remains untouched.
foreach ($name in $mutableData) {
    New-Item -ItemType Directory -Path (Join-Path $distRoot (Join-Path 'DATA' $name)) -Force | Out-Null
}

Copy-Item -LiteralPath (Join-Path $projectRoot 'DISTRIBUTION.md') -Destination (Join-Path $distRoot 'LEEME.txt')

# Keep a single portable directory at the root of the archive.  This layout is
# accepted by all released updater versions, including the one in v0.17.0.
# The two files below are also checked explicitly so a release cannot be
# published with an archive that the updater cannot install.
$packagePath = Join-Path $projectRoot 'dist\IC-SDR-Go-windows-x64.zip'
$checksumPath = "$packagePath.sha256"
Remove-Item -LiteralPath $packagePath -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $checksumPath -Force -ErrorAction SilentlyContinue
Add-Type -AssemblyName System.IO.Compression.FileSystem
[System.IO.Compression.ZipFile]::CreateFromDirectory(
    $distRoot,
    $packagePath,
    [System.IO.Compression.CompressionLevel]::Optimal,
    $true
)
$archive = [System.IO.Compression.ZipFile]::OpenRead($packagePath)
try {
    $requiredEntries = @('IC-SDR-Go/IC-SDR-Go.exe', 'IC-SDR-Go/IC-SDR-Updater.exe')
    # ZipArchive preserves the directory separator supplied by
    # CreateFromDirectory.  On Windows that can be a backslash even though ZIP
    # tools conventionally display forward slashes, so compare normalized
    # entry names rather than rejecting a valid package.
    $archiveEntries = @($archive.Entries | ForEach-Object { $_.FullName.Replace('\', '/') })
    foreach ($required in $requiredEntries) {
        if (-not ($archiveEntries -contains $required)) {
            throw "El paquete no contiene el archivo requerido: $required"
        }
    }
} finally {
    $archive.Dispose()
}
$hash = (Get-FileHash -LiteralPath $packagePath -Algorithm SHA256).Hash.ToLowerInvariant()
[System.IO.File]::WriteAllText($checksumPath, "$hash  IC-SDR-Go-windows-x64.zip`n", [System.Text.UTF8Encoding]::new($false))

$size = (Get-ChildItem -LiteralPath $distRoot -Recurse -File | Measure-Object Length -Sum).Sum
Write-Host ("Distribución lista: {0} ({1:N1} MB)" -f $distRoot, ($size / 1MB)) -ForegroundColor Green
Write-Host "Paquete de actualización: $packagePath" -ForegroundColor Green
