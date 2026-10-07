# tcode installer for Windows.
#
# Default: downloads the prebuilt binary (tcode-windows-amd64.exe or
# tcode-windows-arm64.exe) from the latest GitHub release into
# %USERPROFILE%\.tcode\bin and adds that directory to the USER PATH
# (registry 'User' scope, case-insensitive dedupe; never touches system PATH).
# The checksum is verified against checksums.txt before installing.
#
#   -Preview       install the latest `preview-v*` pre-release (no Go needed)
#   -Build         compile from the current checkout instead (devs, needs Go)
#   -Uninstall     remove the binary and the PATH entry
#   -InstallDir    custom install directory (tests / power users)
#   -NoPath        skip the registry PATH update (safe for sandbox tests)
#
# Advanced: $env:TCODE_RELEASE_BASE overrides the release base URL (tests).

param(
    [string]$InstallDir = "",
    [switch]$Build,
    [switch]$Preview,
    [switch]$NoPath,
    [switch]$Uninstall
)

$ErrorActionPreference = "Stop"
$TcodeHome   = Join-Path $HOME ".tcode"
$InstallPath = if ($InstallDir) { $InstallDir } else { Join-Path $TcodeHome "bin" }
$ReleaseBase = if ($env:TCODE_RELEASE_BASE) { $env:TCODE_RELEASE_BASE } else { "https://github.com/leav-dev/tcode/releases/latest/download" }

if ($Preview -and $Build) {
    Write-Error "-Preview y -Build son incompatibles (preview descarga binario, build compila local)."
    exit 1
}

if ($Preview -and -not $env:TCODE_RELEASE_BASE) {
    $releasesUrl = "https://api.github.com/repos/leav-dev/tcode/releases"
    try {
        $releases = Invoke-RestMethod -Uri $releasesUrl -UseBasicParsing
    } catch {
        Write-Error "No se pudo consultar $releasesUrl (sin red o GitHub no responde): $($_.Exception.Message)"
        exit 1
    }
    $previewTag = ($releases | Where-Object { $_.tag_name -like 'preview-v*' } | Select-Object -ExpandProperty tag_name -First 1)
    if (-not $previewTag) {
        Write-Error "No hay tags preview-v* en $releasesUrl."
        exit 1
    }
    $ReleaseBase = "https://github.com/leav-dev/tcode/releases/download/$previewTag"
    Write-Host "Preview: $previewTag ($ReleaseBase)"
} elseif ($Preview -and $env:TCODE_RELEASE_BASE) {
    Write-Host "Preview pedido pero TCODE_RELEASE_BASE explícito gana: $ReleaseBase"
}

function Get-TcodePathEntry {
    param([string]$UserPath)
    if (-not $UserPath) { return $null }
    foreach ($entry in $UserPath.Split(';')) {
        if ($entry.Trim() -and $entry.Trim().TrimEnd('\') -eq $InstallPath.TrimEnd('\')) {
            return $entry
        }
    }
    return $null
}

function Update-UserPath {
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (Get-TcodePathEntry $userPath) {
        Write-Host "PATH already contains $InstallPath (User scope)"
        return
    }
    $parts = @($InstallPath)
    if ($userPath) { $parts += $userPath.Split(';') | Where-Object { $_ -ne '' } }
    [Environment]::SetEnvironmentVariable('Path', ($parts -join ';'), 'User')
    Write-Host "PATH updated (User scope): $InstallPath"
}

# Get-Sha256: hash with plain .NET - Get-FileHash is missing on some PS 5.1 hosts.
function Get-Sha256 {
    param([string]$Path)
    $sha = [System.Security.Cryptography.SHA256]::Create()
    $fs = [System.IO.File]::OpenRead($Path)
    try {
        $bytes = $sha.ComputeHash($fs)
    } finally {
        $fs.Dispose()
    }
    return ([System.BitConverter]::ToString($bytes)).Replace('-', '').ToLower()
}

function Remove-UserPathEntry {
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not $userPath) { return }
    $kept = @($userPath.Split(';') | Where-Object {
        $e = $_.Trim().TrimEnd('\')
        $e -ne '' -and $e -ne $InstallPath.TrimEnd('\')
    })
    [Environment]::SetEnvironmentVariable('Path', ($kept -join ';'), 'User')
}

if ($Uninstall) {
    if (Test-Path $InstallPath) {
        Remove-Item -Recurse -Force $InstallPath
        Write-Host "Removed $InstallPath"
    }
    if (-not $NoPath) {
        Remove-UserPathEntry
        Write-Host "Removed $InstallPath from the User PATH"
    }
    Write-Host "tcode uninstalled. Open a NEW terminal to drop it from the current session."
    exit 0
}

# build_local: compile from the checkout (requires Go 1.25+).
function Build-Local {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Error "Go is required to build tcode (go.mod requires Go 1.25+)."
        exit 1
    }
    $scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
    $repoRoot  = Split-Path -Parent $scriptDir
    New-Item -ItemType Directory -Force -Path $InstallPath | Out-Null
    Write-Host "Building tcode into $InstallPath ..."
    Push-Location $repoRoot
    try {
        go build -o (Join-Path $InstallPath "tcode.exe") .
    } finally {
        Pop-Location
    }
    Write-Host "Installed: $InstallPath\tcode.exe"
}

# release_install: download + verify checksum + install (no Go needed).
function Install-Release {
    $arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } elseif ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture -eq [System.Runtime.InteropServices.Architecture]::Arm64) { 'arm64' } else { 'amd64' }
    $asset = "tcode-windows-$arch.exe"

    New-Item -ItemType Directory -Force -Path $InstallPath | Out-Null
    $tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("tcode-install-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Force -Path $tmp | Out-Null
    try {
        Write-Host "Downloading $asset ($ReleaseBase) ..."
        Invoke-WebRequest -Uri "$ReleaseBase/$asset" -OutFile (Join-Path $tmp $asset) -UseBasicParsing
        Invoke-WebRequest -Uri "$ReleaseBase/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt') -UseBasicParsing

        $line = Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { $_ -like "*$asset" } | Select-Object -First 1
        if (-not $line) { throw "checksums.txt has no entry for $asset" }
        $expected = ($line -split '\s+')[0].ToLower()
        $actual = Get-Sha256 (Join-Path $tmp $asset)
        if ($actual -ne $expected) { throw "checksum mismatch for $asset (expected $expected, got $actual)" }

        Copy-Item (Join-Path $tmp $asset) (Join-Path $InstallPath 'tcode.exe') -Force
        Write-Host "Installed: $InstallPath\tcode.exe"
    } finally {
        Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
    }
}

if ($Build) {
    Build-Local
} else {
    Install-Release
}

if (-not $NoPath) {
    Update-UserPath
}

Write-Host ""
Write-Host "tcode installed:"
Write-Host "  binary: $InstallPath\tcode.exe"
Write-Host "  PATH:   $InstallPath (User scope)"
Write-Host ""
Write-Host "Open a NEW terminal and run: tcode"
Write-Host "To remove: powershell -ExecutionPolicy Bypass -File scripts/install.ps1 -Uninstall"