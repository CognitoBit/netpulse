# netpulse installer for Windows.
#   irm https://raw.githubusercontent.com/PLACEHOLDER/netpulse/main/install.ps1 | iex
$ErrorActionPreference = 'Stop'

$Repo = 'PLACEHOLDER/netpulse'

# Windows PowerShell 5.1 defaults to TLS 1.0 for Invoke-WebRequest.
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { throw "unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
}

$asset = "netpulse_windows_$arch.zip"
$url = "https://github.com/$Repo/releases/latest/download/$asset"

$installDir = if ($env:NETPULSE_INSTALL_DIR) { $env:NETPULSE_INSTALL_DIR }
              else { Join-Path $env:LOCALAPPDATA 'Programs\netpulse' }

Write-Host "downloading $asset ..."
$tmp = Join-Path $env:TEMP "netpulse-install-$([guid]::NewGuid().ToString('N').Substring(0,8))"
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
try {
    $zip = Join-Path $tmp $asset
    Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing
    Expand-Archive -Path $zip -DestinationPath $tmp -Force

    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    Copy-Item (Join-Path $tmp 'netpulse.exe') (Join-Path $installDir 'netpulse.exe') -Force
    Write-Host "installed to $installDir\netpulse.exe"

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $installDir) {
        [Environment]::SetEnvironmentVariable('Path', "$userPath;$installDir", 'User')
        $env:Path = "$env:Path;$installDir"
        Write-Host "added $installDir to your user PATH (new terminals pick it up automatically)"
    }

    & (Join-Path $installDir 'netpulse.exe') --version
    Write-Host 'run: netpulse'
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
