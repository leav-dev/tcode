# tcode installer for Windows.
#
# Builds tcode.exe to %USERPROFILE%\.tcode\bin and adds that directory to the
# USER PATH (registry 'User' scope, case-insensitive dedupe; never touches the
# system PATH). Needs Go 1.25+ on the machine (see go.mod).
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts/install.ps1
#   powershell -ExecutionPolicy Bypass -File scripts/install.ps1 -Uninstall
#
# Advanced / tests:
#   -InstallDir <dir>   install (or remove) a custom directory
#   -NoPath             skip the registry PATH update (safe for sandbox tests)

param(
    [string]$InstallDir = "",
    [switch]$NoPath,
    [switch]$Uninstall
)

$ErrorActionPreference = "Stop"
$TcodeHome   = Join-Path $HOME ".tcode"
$InstallPath = if ($InstallDir) { $InstallDir } else { Join-Path $TcodeHome "bin" }

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
    $newPath = ($parts -join ';')
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
    Write-Host "PATH updated (User scope): $InstallPath"
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

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Error "Go is required to build tcode (go.mod requires Go 1.25+)."
    exit 1
}

$scriptDir   = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot    = Split-Path -Parent $scriptDir

New-Item -ItemType Directory -Force -Path $InstallPath | Out-Null
Write-Host "Building tcode into $InstallPath ..."
Push-Location $repoRoot
try {
    go build -o (Join-Path $InstallPath "tcode.exe") .
} finally {
    Pop-Location
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