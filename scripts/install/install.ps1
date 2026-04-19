param(
    [string]$Version = $env:VERSION,
    [string]$InstallDir = $env:INSTALL_DIR
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$Repo = if ($env:REPO) { $env:REPO } else { "agentclash/agentclash" }
$Binary = if ($env:BINARY) { $env:BINARY } else { "agentclash" }

if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    throw "This installer is for Windows. Use scripts/install/install.sh on Linux or macOS."
}

if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA "agentclash\bin"
}

try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12 -bor [Net.SecurityProtocolType]::Tls13
} catch {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
}

function Get-AgentClashArch {
    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
    switch ($arch) {
        "x64" { return "amd64" }
        "arm64" { return "arm64" }
        default { throw "Unsupported architecture: $arch" }
    }
}

function Invoke-Download {
    param(
        [Parameter(Mandatory = $true)][string]$Uri,
        [Parameter(Mandatory = $true)][string]$OutFile
    )

    Invoke-WebRequest -Uri $Uri -OutFile $OutFile -UseBasicParsing -MaximumRedirection 10
}

function Test-ReleaseAsset {
    param([Parameter(Mandatory = $true)][string]$Uri)

    try {
        Invoke-WebRequest -Uri $Uri -Method Head -UseBasicParsing -MaximumRedirection 10 | Out-Null
    } catch {
        throw "Release asset not found: $Uri"
    }
}

function Read-ExpectedChecksum {
    param(
        [Parameter(Mandatory = $true)][string]$ChecksumsPath,
        [Parameter(Mandatory = $true)][string]$FileName
    )

    foreach ($line in Get-Content -Path $ChecksumsPath) {
        $parts = $line -split "\s+"
        if ($parts.Count -ge 2 -and $parts[1] -eq $FileName) {
            return $parts[0].ToLowerInvariant()
        }
    }

    throw "checksums.txt does not contain an entry for $FileName"
}

if (-not $Version) {
    $latest = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ "User-Agent" = "agentclash-installer" }
    $Version = $latest.tag_name
}

if (-not $Version -or -not $Version.StartsWith("v")) {
    throw "Version must be a release tag like v0.1.2, got: $Version"
}

$Arch = Get-AgentClashArch
$FileName = "${Binary}_windows_${Arch}.zip"
$BaseUrl = "https://github.com/$Repo/releases/download/$Version"
$ArchiveUrl = "$BaseUrl/$FileName"
$ChecksumsUrl = "$BaseUrl/checksums.txt"

Write-Host "Installing $Binary $Version (windows/$Arch)..."

Test-ReleaseAsset -Uri $ArchiveUrl
Test-ReleaseAsset -Uri $ChecksumsUrl

$TempDir = Join-Path ([System.IO.Path]::GetTempPath()) ("agentclash-install-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $TempDir | Out-Null

try {
    $ArchivePath = Join-Path $TempDir $FileName
    $ChecksumsPath = Join-Path $TempDir "checksums.txt"
    $ExtractDir = Join-Path $TempDir "extract"

    Invoke-Download -Uri $ArchiveUrl -OutFile $ArchivePath
    Invoke-Download -Uri $ChecksumsUrl -OutFile $ChecksumsPath

    $ExpectedHash = Read-ExpectedChecksum -ChecksumsPath $ChecksumsPath -FileName $FileName
    $ActualHash = (Get-FileHash -Path $ArchivePath -Algorithm SHA256).Hash.ToLowerInvariant()

    if ($ExpectedHash -ne $ActualHash) {
        throw "Checksum mismatch for $FileName"
    }

    New-Item -ItemType Directory -Path $ExtractDir | Out-Null
    Expand-Archive -Path $ArchivePath -DestinationPath $ExtractDir -Force

    $SourceBinary = Join-Path $ExtractDir "${Binary}.exe"
    if (-not (Test-Path $SourceBinary)) {
        throw "Archive did not contain ${Binary}.exe"
    }

    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    $TargetBinary = Join-Path $InstallDir "${Binary}.exe"
    Copy-Item -Path $SourceBinary -Destination $TargetBinary -Force

    Write-Host "Installed $Binary $Version to $TargetBinary"

    $CurrentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $ProcessPath = [Environment]::GetEnvironmentVariable("Path", "Process")
    $KnownPaths = @($CurrentPath, $ProcessPath) -join ";"

    if (-not (($KnownPaths -split ";") -contains $InstallDir)) {
        Write-Host ""
        Write-Host "Add $InstallDir to your user PATH if PowerShell cannot find $Binary:"
        Write-Host "  [Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + ';$InstallDir', 'User')"
        Write-Host "Then open a new terminal."
    }

    Write-Host ""
    Write-Host "Get started:"
    Write-Host "  $Binary auth login"
    Write-Host "  $Binary --help"
    Write-Host ""
    Write-Host "Uninstall this script install:"
    Write-Host "  Remove-Item '$TargetBinary'"
} finally {
    Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
}
