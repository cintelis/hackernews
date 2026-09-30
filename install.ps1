# cintelis installer for Windows: downloads the latest release, checks it
# against the release's checksums.txt, installs it and adds it to your PATH.
#   irm https://raw.githubusercontent.com/cintelis/hackernews/main/install.ps1 | iex
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repo = 'cintelis/hackernews'
$dir = if ($env:CINTELIS_INSTALL_DIR) { $env:CINTELIS_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'Programs\cintelis' }
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }

# the releases/latest page redirects to .../tag/vX.Y.Z
$req = [Net.WebRequest]::Create("https://github.com/$repo/releases/latest")
$req.AllowAutoRedirect = $false
$res = $req.GetResponse()
$location = $res.Headers['Location']
$res.Close()
if ($location -notmatch '/tag/v(\d+\.\d+\.\d+)$') { throw "couldn't find the latest cintelis release" }
$version = $Matches[1]

$asset = "cintelis_${version}_windows_$arch.zip"
$base = "https://github.com/$repo/releases/download/v$version"
$tmp = Join-Path ([IO.Path]::GetTempPath()) ("cintelis-" + [guid]::NewGuid())
New-Item -ItemType Directory $tmp | Out-Null
try {
    Write-Host "downloading cintelis $version (windows/$arch) ..."
    Invoke-WebRequest "$base/$asset" -OutFile "$tmp\$asset" -UseBasicParsing
    Invoke-WebRequest "$base/checksums.txt" -OutFile "$tmp\checksums.txt" -UseBasicParsing

    $want = Get-Content "$tmp\checksums.txt" |
        ForEach-Object { $f = $_ -split '\s+'; if ($f[1] -eq $asset) { $f[0] } } |
        Select-Object -First 1
    $got = (Get-FileHash "$tmp\$asset" -Algorithm SHA256).Hash.ToLower()
    if (-not $want -or $want -ne $got) { throw "checksum mismatch for $asset - not installing" }

    Expand-Archive "$tmp\$asset" -DestinationPath "$tmp\x"
    New-Item -ItemType Directory -Force $dir | Out-Null
    Copy-Item "$tmp\x\cintelis.exe" (Join-Path $dir 'cintelis.exe') -Force
} finally {
    Remove-Item -Recurse -Force $tmp
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (-not $userPath) { $userPath = '' }
if (($userPath -split ';') -notcontains $dir) {
    [Environment]::SetEnvironmentVariable('Path', (($userPath.TrimEnd(';'), $dir) -join ';').TrimStart(';'), 'User')
    Write-Host "added $dir to your PATH - open a new terminal to use it"
}
Write-Host "installed: $(Join-Path $dir 'cintelis.exe')"
