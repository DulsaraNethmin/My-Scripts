<#
.SYNOPSIS
    Install spinup on Windows.

.DESCRIPTION
    Downloads the release archive for this machine, checks it against the
    release's checksums.txt, and installs one binary. The stack catalog is
    compiled into that binary, so there is nothing else to place.

.PARAMETER Version
    Install a specific release, e.g. v1.1.0. Defaults to the latest, or to
    $env:SPINUP_VERSION.

.PARAMETER Dir
    Install into this directory. Defaults to $env:LOCALAPPDATA\spinup\bin, or
    to $env:SPINUP_INSTALL_DIR.

.EXAMPLE
    irm https://raw.githubusercontent.com/DulsaraNethmin/spinup/main/install.ps1 | iex

.NOTES
    $env:SPINUP_REPO and $env:SPINUP_API point the script at another repository
    or API.
#>

[CmdletBinding()]
param(
    [string]$Version = $env:SPINUP_VERSION,
    [string]$Dir = $env:SPINUP_INSTALL_DIR
)

$ErrorActionPreference = 'Stop'

# Windows PowerShell 5.1 still negotiates TLS 1.0 by default, which GitHub
# refuses. PowerShell 7 ignores this.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repo = if ($env:SPINUP_REPO) { $env:SPINUP_REPO } else { 'DulsaraNethmin/spinup' }
$api = if ($env:SPINUP_API) { $env:SPINUP_API.TrimEnd('/') } else { 'https://api.github.com' }
if (-not $Version) { $Version = 'latest' }
if (-not $Dir) { $Dir = Join-Path $env:LOCALAPPDATA 'spinup\bin' }

# PROCESSOR_ARCHITEW6432 is set when a 32-bit PowerShell is running on 64-bit
# Windows, where PROCESSOR_ARCHITECTURE would say x86 and send us to a build
# that does not exist.
$machine = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }

$arch = switch ($machine) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { throw "unsupported architecture: $machine (spinup ships amd64 and arm64)" }
}

$releaseUrl = if ($Version -eq 'latest') {
    "$api/repos/$repo/releases/latest"
}
else {
    "$api/repos/$repo/releases/tags/$Version"
}

Write-Host "looking up $repo $Version"
try {
    $release = Invoke-RestMethod -Uri $releaseUrl -Headers @{ 'User-Agent' = 'spinup' }
}
catch {
    throw "cannot read ${releaseUrl}: $($_.Exception.Message)"
}

$tag = $release.tag_name
if (-not $tag) { throw "$repo has no release for '$Version'" }

# The archive names have no leading v; the tags do.
$number = $tag -replace '^v', ''
$name = "spinup_${number}_windows_$arch.zip"

$asset = $release.assets | Where-Object { $_.name -eq $name } | Select-Object -First 1
if (-not $asset) { throw "$tag has no $name — spinup may not ship a build for windows/$arch yet" }

$sums = $release.assets | Where-Object { $_.name -eq 'checksums.txt' } | Select-Object -First 1
if (-not $sums) { throw "$tag has no checksums.txt, so the download cannot be verified" }

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("spinup-" + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp -Force | Out-Null

try {
    $zip = Join-Path $tmp $name
    Write-Host "downloading $name ($tag)"
    Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $zip -UseBasicParsing
    Invoke-WebRequest -Uri $sums.browser_download_url -OutFile (Join-Path $tmp 'checksums.txt') -UseBasicParsing

    $want = (Get-Content (Join-Path $tmp 'checksums.txt') |
        Where-Object { $_ -match "\s$([regex]::Escape($name))$" } |
        Select-Object -First 1) -split '\s+' | Select-Object -First 1
    if (-not $want) { throw "checksums.txt has no entry for $name" }

    $got = (Get-FileHash -Path $zip -Algorithm SHA256).Hash
    if ($got -ne $want.ToUpper()) {
        throw "$name does not match its checksum`n  got  $got`n  want $($want.ToUpper())"
    }

    Expand-Archive -Path $zip -DestinationPath $tmp -Force
    $exe = Join-Path $tmp 'spin.exe'
    if (-not (Test-Path $exe)) { throw 'the archive has no spin.exe in it' }
    $alias = Join-Path $tmp 'spinup.exe'
    if (-not (Test-Path $alias)) { throw 'the archive has no spinup.exe in it' }

    New-Item -ItemType Directory -Path $Dir -Force | Out-Null
    Move-Item -Path $exe -Destination (Join-Path $Dir 'spin.exe') -Force
    Move-Item -Path $alias -Destination (Join-Path $Dir 'spinup.exe') -Force
}
finally {
    Remove-Item -Path $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host ""
Write-Host "spin $tag installed at $(Join-Path $Dir 'spin.exe'), with spinup.exe beside it"

# Put the directory on the user's PATH if it is not there already. This edits
# the user environment, not the machine's, so it needs no administrator.
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (($userPath -split ';') -notcontains $Dir) {
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$Dir".Trim(';'), 'User')
    $env:Path = "$env:Path;$Dir"
    Write-Host "added $Dir to your PATH — open a new terminal for it to take effect"
}

Write-Host ""
Write-Host "Next:"
Write-Host "  spin doctor          check Docker is ready"
Write-Host "  spin list            the stack catalog"
Write-Host "  spin up postgres     Postgres 16 + pgAdmin"
Write-Host ""
Write-Host "Shell completion: spin completion powershell --help"
