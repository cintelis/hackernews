# cintelis installer for Windows: downloads the latest release, checks that its
# checksums.txt is signed with the Cintelis release key and that the archive
# matches it, installs it and adds it to your PATH. Uses the OpenSSH client
# built into Windows 10 (1809+) and 11 to check the signature.
#   irm https://raw.githubusercontent.com/cintelis/hackernews/main/install.ps1 | iex
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repo = 'cintelis/hackernews'
# the release signing key; also in install.sh and internal/update/release_key.pub
$releaseKey = 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOCB3IaMc3Lc4JvPv6rWCVLpTjmvvPrhFFPST0NSsypP'
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
$archive = Join-Path $tmp $asset
$sums = Join-Path $tmp 'checksums.txt'
$sig = Join-Path $tmp 'checksums.txt.sig'
$signers = Join-Path $tmp 'allowed_signers'
try {
    Write-Host "downloading cintelis $version (windows/$arch) ..."
    Invoke-WebRequest "$base/$asset" -OutFile $archive -UseBasicParsing
    Invoke-WebRequest "$base/checksums.txt" -OutFile $sums -UseBasicParsing
    Invoke-WebRequest "$base/checksums.txt.sig" -OutFile $sig -UseBasicParsing

    $builtin = Join-Path $env:SystemRoot 'System32\OpenSSH\ssh-keygen.exe'
    $keygen = @((Get-Command ssh-keygen.exe -ErrorAction Ignore).Source, $builtin) |
        Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
    if (-not $keygen) { throw "ssh-keygen (OpenSSH 8.1+) is needed to check the release signature - install the Windows OpenSSH client" }
    [IO.File]::WriteAllText($signers, "cintelis-release namespaces=`"cintelis-release`" $releaseKey`n")
    # stdin must be the file's exact bytes: a PowerShell pipe would re-encode it
    $check = Start-Process -FilePath $keygen -NoNewWindow -Wait -PassThru `
        -ArgumentList @('-Y', 'verify', '-f', "`"$signers`"", '-I', 'cintelis-release', '-n', 'cintelis-release', '-s', "`"$sig`"") `
        -RedirectStandardInput $sums `
        -RedirectStandardOutput (Join-Path $tmp 'verify.out') -RedirectStandardError (Join-Path $tmp 'verify.err')
    if ($check.ExitCode -ne 0) { throw "v$version is not signed with the Cintelis release key - not installing" }

    $want = Get-Content $sums |
        ForEach-Object { $f = $_ -split '\s+'; if ($f[1] -eq $asset) { $f[0] } } |
        Select-Object -First 1
    $got = (Get-FileHash $archive -Algorithm SHA256).Hash.ToLower()
    if (-not $want -or $want -ne $got) { throw "checksum mismatch for $asset - not installing" }

    $unpacked = Join-Path $tmp 'x'
    Expand-Archive $archive -DestinationPath $unpacked
    New-Item -ItemType Directory -Force $dir | Out-Null
    Copy-Item (Join-Path $unpacked 'cintelis.exe') (Join-Path $dir 'cintelis.exe') -Force
} finally {
    Remove-Item -Recurse -Force $tmp
}

# A Start menu entry, so the app can be found and reopened from Start. It
# points at the installed exe, which `cintelis update` replaces in place.
$shortcut = Join-Path ([Environment]::GetFolderPath('Programs')) 'CISO AI - Hacker News.lnk'
try {
    $link = (New-Object -ComObject WScript.Shell).CreateShortcut($shortcut)
    $link.TargetPath = Join-Path $dir 'cintelis.exe'
    $link.IconLocation = (Join-Path $dir 'cintelis.exe') + ',0' # the CISO AI mark built into the exe
    $link.WorkingDirectory = $env:USERPROFILE
    $link.Description = 'CISO AI - Hacker News, in the terminal (cintelis)'
    $link.Save()
    Write-Host "added to the Start menu: CISO AI - Hacker News"
} catch {
    Write-Host "couldn't add a Start menu shortcut ($_) - run cintelis from a terminal instead"
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (-not $userPath) { $userPath = '' }
if (($userPath -split ';') -notcontains $dir) {
    [Environment]::SetEnvironmentVariable('Path', (($userPath.TrimEnd(';'), $dir) -join ';').TrimStart(';'), 'User')
    Write-Host "added $dir to your PATH - open a new terminal to use it"
}
Write-Host "installed: $(Join-Path $dir 'cintelis.exe')"
