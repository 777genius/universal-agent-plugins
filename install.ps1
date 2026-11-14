# Install the native agentplugins CLI from verified GitHub Release assets.
# The CLI itself does not require Node.js. Individual plugins may.

[CmdletBinding()]
param(
    [string]$Version = $(if ($env:AGENTPLUGINS_VERSION) { $env:AGENTPLUGINS_VERSION } else { "latest" }),
    [string]$BinDir = $(if ($env:AGENTPLUGINS_BIN_DIR) { $env:AGENTPLUGINS_BIN_DIR } else { Join-Path $HOME ".local\bin" }),
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$Command
)

$ErrorActionPreference = "Stop"
$Repository = if ($env:AGENTPLUGINS_REPOSITORY) { $env:AGENTPLUGINS_REPOSITORY } else { "777genius/universal-agent-plugins" }
$ApiBase = if ($env:AGENTPLUGINS_API_BASE) { $env:AGENTPLUGINS_API_BASE.TrimEnd("/") } else { "https://api.github.com" }
$ReleaseHost = if ($env:AGENTPLUGINS_RELEASE_BASE_URL) {
    $env:AGENTPLUGINS_RELEASE_BASE_URL.TrimEnd("/")
} elseif ($ApiBase -eq "https://api.github.com") {
    "https://github.com"
} else {
    $ApiBase -replace "/api/v3$", "" -replace "/api$", ""
}

function Fail([string]$Message) {
    throw "agentplugins installer: $Message"
}

function Get-Headers {
    $headers = @{ Accept = "application/vnd.github+json" }
    if ($env:GITHUB_TOKEN) {
        $headers.Authorization = "Bearer $($env:GITHUB_TOKEN)"
    }
    return $headers
}

function Normalize-Tag([string]$InputVersion) {
    switch -Regex ($InputVersion) {
        "^(|latest)$" { return "" }
        "^agentplugins-v" { return $InputVersion }
        "^v" { return "agentplugins-$InputVersion" }
        default { return "agentplugins-v$InputVersion" }
    }
}

function Assert-StableTag([string]$Tag) {
    if ($Tag -notmatch '^agentplugins-v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') {
        Fail "invalid stable version or tag: $Tag"
    }
}

function Resolve-Architecture {
    $architecture = if ($env:PROCESSOR_ARCHITEW6432) {
        $env:PROCESSOR_ARCHITEW6432
    } else {
        $env:PROCESSOR_ARCHITECTURE
    }
    switch ($architecture.ToUpperInvariant()) {
        "AMD64" { return "amd64" }
        "X86_64" { return "amd64" }
        "ARM64" { return "arm64" }
        default { Fail "unsupported architecture: $architecture" }
    }
}

function Invoke-Download([string]$Uri, [string]$OutFile) {
    $parameters = @{
        Uri = $Uri
        OutFile = $OutFile
        Headers = (Get-Headers)
        MaximumRedirection = 10
    }
    if ($PSVersionTable.PSVersion.Major -le 5) {
        $parameters["UseBasicParsing"] = $true
    }
    Invoke-WebRequest @parameters
}

$Tag = Normalize-Tag $Version
if (-not $Tag) {
    $latest = Invoke-RestMethod -Uri "$ApiBase/repos/$Repository/releases/latest" -Headers (Get-Headers)
    $Tag = [string]$latest.tag_name
    if (-not $Tag) {
        Fail "latest release response did not contain tag_name"
    }
}
Assert-StableTag $Tag
$ResolvedVersion = $Tag.Substring("agentplugins-v".Length)
$Architecture = Resolve-Architecture
$AssetName = "agentplugins_${ResolvedVersion}_windows_${Architecture}.exe"
$DownloadBase = "$ReleaseHost/$Repository/releases/download/$Tag"
$TempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("agentplugins-" + [Guid]::NewGuid().ToString("N"))
$InstallTemp = $null

New-Item -ItemType Directory -Path $TempRoot | Out-Null
try {
    $ChecksumsPath = Join-Path $TempRoot "checksums.txt"
    $BinaryPath = Join-Path $TempRoot $AssetName
    Invoke-Download "$DownloadBase/checksums.txt" $ChecksumsPath

    $escapedAsset = [Regex]::Escape($AssetName)
    $checksumLines = @(Get-Content -LiteralPath $ChecksumsPath | Where-Object { $_ -match "^[0-9a-f]{64}\s+$escapedAsset$" })
    if ($checksumLines.Count -ne 1) {
        Fail "checksums.txt must contain exactly one valid entry for $AssetName"
    }
    $hashMatch = [Regex]::Match($checksumLines[0], "^(?<hash>[0-9a-f]{64})\s+")
    if (-not $hashMatch.Success) {
        Fail "checksums.txt contains an invalid SHA-256 for $AssetName"
    }
    $ExpectedHash = $hashMatch.Groups["hash"].Value

    Invoke-Download "$DownloadBase/$AssetName" $BinaryPath
    if ((Get-Item -LiteralPath $BinaryPath).Length -le 0) {
        Fail "downloaded binary is empty: $AssetName"
    }
    $ActualHash = (Get-FileHash -LiteralPath $BinaryPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($ActualHash -ne $ExpectedHash) {
        Fail "checksum mismatch for $AssetName"
    }

    $ObservedVersion = (& $BinaryPath version 2>&1 | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $ObservedVersion -ne "agentplugins $ResolvedVersion") {
        Fail "downloaded binary reported unexpected version: $ObservedVersion"
    }

    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    $Destination = Join-Path $BinDir "agentplugins.exe"
    if (Test-Path -LiteralPath $Destination -PathType Container) {
        Fail "install destination is a directory: $Destination"
    }
    $InstallTemp = Join-Path $BinDir (".agentplugins-" + [Guid]::NewGuid().ToString("N") + ".exe")
    Copy-Item -LiteralPath $BinaryPath -Destination $InstallTemp
    if (Test-Path -LiteralPath $Destination -PathType Leaf) {
        [System.IO.File]::Replace($InstallTemp, $Destination, $null, $true)
    } else {
        [System.IO.File]::Move($InstallTemp, $Destination)
    }
    $InstallTemp = $null

    Write-Output "Installed agentplugins $ResolvedVersion"
    Write-Output "Path: $Destination"
    Write-Output "SHA-256: $ActualHash"
    if (($env:PATH -split [IO.Path]::PathSeparator) -notcontains $BinDir) {
        Write-Output "PATH hint: add $BinDir to PATH"
    }

    if ($env:AGENTPLUGINS_OUTPUT_FILE) {
        @(
            "version=$ResolvedVersion"
            "tag=$Tag"
            "path=$Destination"
            "asset=$AssetName"
            "sha256=$ActualHash"
        ) | Add-Content -LiteralPath $env:AGENTPLUGINS_OUTPUT_FILE
    }

    if ($Command.Count -gt 0) {
        & $Destination @Command
        if ($LASTEXITCODE -ne 0) {
            exit $LASTEXITCODE
        }
    }
} finally {
    if ($InstallTemp -and (Test-Path -LiteralPath $InstallTemp)) {
        Remove-Item -LiteralPath $InstallTemp -Force
    }
    if (Test-Path -LiteralPath $TempRoot) {
        Remove-Item -LiteralPath $TempRoot -Recurse -Force
    }
}
