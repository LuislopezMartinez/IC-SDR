[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$projectRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$runtimeRoot = Join-Path $projectRoot 'DATA\tools\pocsag\runtime'
$binRoot = Join-Path $runtimeRoot 'bin'
$sourceRoot = Join-Path $runtimeRoot 'source'
$release = '1.6.1'
$archiveUrl = "https://github.com/EliasOenal/multimon-ng/releases/download/$release/multimon-ng-win64.zip"
$archiveSha256 = '115946771798e110c76b9dc40eedf285692c5ac0a37386509586d904d92244a0'
$executableSha256 = 'a9e31708caf7b85852aef822d1b4b4d0fb77965b4c527f25343348a5c1350009'
$temporaryRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('ic-sdr-pocsag-' + [guid]::NewGuid().ToString('N'))
$archivePath = Join-Path $temporaryRoot 'multimon-ng-win64.zip'
$extractRoot = Join-Path $temporaryRoot 'extracted'

try {
    New-Item -ItemType Directory -Path $temporaryRoot, $extractRoot, $binRoot, $sourceRoot -Force | Out-Null
    Invoke-WebRequest -Uri $archiveUrl -OutFile $archivePath
    $actualArchiveHash = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualArchiveHash -ne $archiveSha256) {
        throw "El archivo multimon-ng descargado no coincide con el SHA-256 esperado: $actualArchiveHash"
    }
    Expand-Archive -LiteralPath $archivePath -DestinationPath $extractRoot
    $sourceExe = Join-Path $extractRoot 'multimon-ng.exe'
    $actualExeHash = (Get-FileHash -LiteralPath $sourceExe -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualExeHash -ne $executableSha256) {
        throw "multimon-ng.exe no coincide con el SHA-256 esperado: $actualExeHash"
    }
    Copy-Item -LiteralPath $sourceExe -Destination (Join-Path $binRoot 'multimon-ng.exe') -Force
    Invoke-WebRequest -Uri "https://raw.githubusercontent.com/EliasOenal/multimon-ng/$release/COPYING" -OutFile (Join-Path $runtimeRoot 'COPYING')
    Invoke-WebRequest -Uri "https://raw.githubusercontent.com/EliasOenal/multimon-ng/$release/README.md" -OutFile (Join-Path $sourceRoot 'README.md')
    @{
        upstream = 'EliasOenal/multimon-ng'
        release = $release
        executable = 'bin/multimon-ng.exe'
        executableSha256 = $executableSha256
        archiveSha256 = $archiveSha256
        license = 'GPL-2.0-or-later'
        source = "https://github.com/EliasOenal/multimon-ng/releases/tag/$release"
    } | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $runtimeRoot 'runtime-version.json') -Encoding utf8
    Write-Host "Runtime POCSAG instalado en $runtimeRoot"
} finally {
    if (Test-Path -LiteralPath $temporaryRoot) {
        Remove-Item -LiteralPath $temporaryRoot -Recurse -Force
    }
}
