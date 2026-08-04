$ErrorActionPreference = "Stop"

$Repository = "gabrielchantayan/paper"
$Version = if ($env:PAPER_VERSION) { $env:PAPER_VERSION } elseif ($args.Count -gt 0) { $args[0] } else { $null }
$InstallDir = if ($env:PAPER_INSTALL_DIR) { $env:PAPER_INSTALL_DIR } else { Join-Path $HOME ".local\bin" }

if (-not $Version) {
    $Release = Invoke-RestMethod "https://api.github.com/repos/$Repository/releases/latest"
    $Version = $Release.tag_name
}

if ($Version -match '^v(\d+\.\d+\.\d+)$') {
    $Tag = $Version
    $ReleaseVersion = $Matches[1]
} elseif ($Version -match '^(\d+\.\d+\.\d+)$') {
    $Tag = "v$Version"
    $ReleaseVersion = $Version
} else {
    throw "Invalid version $Version; expected v1.2.3 or 1.2.3"
}

$Architecture = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
    "X64" { "amd64" }
    "Arm64" { "arm64" }
    default { throw "Unsupported architecture: $_" }
}

$Archive = "paper_${ReleaseVersion}_windows_${Architecture}.zip"
$ReleaseUrl = "https://github.com/$Repository/releases/download/$Tag"
$Temporary = Join-Path ([System.IO.Path]::GetTempPath()) ("paper-" + [guid]::NewGuid())

try {
    New-Item -ItemType Directory -Path $Temporary | Out-Null
    Invoke-WebRequest "$ReleaseUrl/$Archive" -OutFile (Join-Path $Temporary $Archive)
    Invoke-WebRequest "$ReleaseUrl/checksums.txt" -OutFile (Join-Path $Temporary "checksums.txt")

    $ChecksumLine = Get-Content (Join-Path $Temporary "checksums.txt") |
        Where-Object { $_ -match "\s\./$([regex]::Escape($Archive))$" } |
        Select-Object -First 1
    if (-not $ChecksumLine) { throw "No checksum found for $Archive" }
    $Expected = ($ChecksumLine -split '\s+')[0].ToLowerInvariant()
    $Actual = (Get-FileHash (Join-Path $Temporary $Archive) -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($Actual -ne $Expected) { throw "Checksum mismatch for $Archive" }

    $Package = Join-Path $Temporary "package"
    Expand-Archive (Join-Path $Temporary $Archive) -DestinationPath $Package
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item (Join-Path $Package "paper.exe") (Join-Path $InstallDir "paper.exe") -Force

    Write-Host "Installed paper $ReleaseVersion at $(Join-Path $InstallDir 'paper.exe')"
    if (($env:PATH -split ';') -notcontains $InstallDir) {
        Write-Host "Add $InstallDir to PATH before running paper."
    }
} finally {
    if (Test-Path $Temporary) { Remove-Item -Recurse -Force $Temporary }
}
