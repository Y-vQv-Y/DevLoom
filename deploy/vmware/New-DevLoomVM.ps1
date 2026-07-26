[CmdletBinding()]
param(
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path,
    [string]$VMwareRoot = 'D:\VMware\VMware Workstation',
    [string]$VMName = 'devloom-test',
    [int]$MemoryMB = 12288,
    [int]$CPUs = 6,
    [string]$DiskSize = '120GB'
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path $RepoRoot).Path
$stateRoot = Join-Path $repo '.vmware'
$image = Join-Path $stateRoot 'images\ubuntu-noble-server-cloudimg-amd64.ova'
$vmDir = Join-Path $stateRoot $VMName
$vmx = Join-Path $vmDir "$VMName.vmx"
$keyDir = Join-Path $stateRoot 'keys'
$privateKey = Join-Path $keyDir 'id_ed25519'
$publicKey = "$privateKey.pub"
$ovfTool = Join-Path $VMwareRoot 'OVFTool\ovftool.exe'
$diskManager = Join-Path $VMwareRoot 'vmware-vdiskmanager.exe'
$vmrun = Join-Path $VMwareRoot 'vmrun.exe'

foreach ($required in @($image, $ovfTool, $diskManager, $vmrun)) {
    if (-not (Test-Path -LiteralPath $required)) { throw "Missing required file: $required" }
}

$sumsContent = (Invoke-WebRequest -Uri 'https://cloud-images.ubuntu.com/noble/20260705/SHA256SUMS' -UseBasicParsing).Content
$sums = if ($sumsContent -is [byte[]]) { [Text.Encoding]::UTF8.GetString($sumsContent) } else { [string]$sumsContent }
$match = [regex]::Match($sums, '(?m)^([a-f0-9]{64})\s+\*?noble-server-cloudimg-amd64\.ova\s*$')
if (-not $match.Success) { throw 'Ubuntu OVA checksum was not found in the official SHA256SUMS file' }
$expected = $match.Groups[1].Value
$actual = (Get-FileHash -LiteralPath $image -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actual -ne $expected) { throw "Ubuntu OVA checksum mismatch: expected $expected, got $actual" }

New-Item -ItemType Directory -Force -Path $keyDir | Out-Null
if (-not (Test-Path -LiteralPath $privateKey)) {
    & ssh-keygen.exe -q -t ed25519 -N '""' -C 'devloom-vm' -f $privateKey
    if ($LASTEXITCODE -ne 0) { throw 'ssh-keygen failed' }
}
$authorizedKey = (Get-Content -LiteralPath $publicKey -Raw).Trim()

if (-not (Test-Path -LiteralPath $vmx)) {
    New-Item -ItemType Directory -Force -Path $vmDir | Out-Null
    & $ovfTool --acceptAllEulas --name=$VMName $image $vmx
    if ($LASTEXITCODE -ne 0) { throw 'ovftool import failed' }
}

$disk = Get-ChildItem -LiteralPath $vmDir -Filter '*.vmdk' | Where-Object Name -NotLike '*-flat.vmdk' | Select-Object -First 1
if (-not $disk) { throw "No VMDK found in $vmDir" }
& $diskManager -x $DiskSize $disk.FullName
if ($LASTEXITCODE -ne 0) { throw 'virtual disk expansion failed' }

$userData = @"
#cloud-config
hostname: $VMName
manage_etc_hosts: true
users:
  - default
  - name: devloom
    gecos: DevLoom Test
    groups: [adm, sudo, docker]
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - $authorizedKey
ssh_pwauth: false
package_update: true
package_upgrade: false
write_files:
  - path: /etc/docker/daemon.json
    permissions: '0644'
    content: |
      {"registry-mirrors":["https://docker.m.daocloud.io","https://docker.1ms.run"],"dns":["223.5.5.5","119.29.29.29"]}
  - path: /etc/sysctl.d/99-devloom-ipv4.conf
    permissions: '0644'
    content: |
      net.ipv6.conf.all.disable_ipv6=1
      net.ipv6.conf.default.disable_ipv6=1
packages:
  - ca-certificates
  - curl
  - docker.io
  - docker-buildx
  - docker-compose-v2
  - git
  - jq
  - make
  - open-vm-tools
  - openssl
  - python3
  - rsync
  - unzip
runcmd:
  - [sysctl, --system]
  - [systemctl, enable, --now, docker]
  - [systemctl, enable, --now, open-vm-tools]
  - [usermod, -aG, docker, devloom]
  - [mkdir, -p, /opt/devloom-src]
  - [chown, -R, 'devloom:devloom', /opt/devloom-src]
  - [sh, -c, 'docker version > /var/log/devloom-docker-version.log 2>&1']
final_message: DevLoom VMware guest is ready
"@
$metaData = "instance-id: $VMName-$(Get-Date -Format yyyyMMddHHmmss)`nlocal-hostname: $VMName`n"
$userData64 = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($userData))
$metaData64 = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($metaData))

function Set-VmxValue([string]$Path, [string]$Key, [string]$Value) {
    $lines = @(Get-Content -LiteralPath $Path)
    $replacement = "$Key = `"$Value`""
    $found = $false
    $updated = foreach ($line in $lines) {
        if ($line -match ('^' + [regex]::Escape($Key) + '\s*=')) { $found = $true; $replacement } else { $line }
    }
    if (-not $found) { $updated += $replacement }
    Set-Content -LiteralPath $Path -Value $updated -Encoding ascii
}

Set-VmxValue $vmx 'memsize' ([string]$MemoryMB)
Set-VmxValue $vmx 'numvcpus' ([string]$CPUs)
Set-VmxValue $vmx 'guestinfo.userdata' $userData64
Set-VmxValue $vmx 'guestinfo.userdata.encoding' 'base64'
Set-VmxValue $vmx 'guestinfo.metadata' $metaData64
Set-VmxValue $vmx 'guestinfo.metadata.encoding' 'base64'

$running = & $vmrun list
if ($running -notcontains $vmx) {
    & $vmrun start $vmx nogui
    if ($LASTEXITCODE -ne 0) { throw 'VMware failed to start the guest' }
}

$deadline = (Get-Date).AddMinutes(15)
$ip = $null
while ((Get-Date) -lt $deadline) {
    try { $candidate = (& $vmrun getGuestIPAddress $vmx -wait 2>$null).Trim(); if ($candidate -match '^\d+\.\d+\.\d+\.\d+$') { $ip = $candidate; break } } catch {}
    Start-Sleep -Seconds 5
}
if (-not $ip) { throw 'Timed out waiting for the VMware guest IP address' }

$sshArgs = @('-i', $privateKey, '-o', 'StrictHostKeyChecking=no', '-o', 'UserKnownHostsFile=NUL', '-o', 'ConnectTimeout=5', "devloom@$ip")
$deadline = (Get-Date).AddMinutes(20)
do {
    & ssh.exe @sshArgs 'cloud-init status --wait >/dev/null 2>&1 && docker version >/dev/null 2>&1'
    if ($LASTEXITCODE -eq 0) { break }
    Start-Sleep -Seconds 10
} while ((Get-Date) -lt $deadline)
if ($LASTEXITCODE -ne 0) { throw 'Guest cloud-init or Docker initialization did not complete' }

[pscustomobject]@{ VMX=$vmx; IP=$ip; User='devloom'; PrivateKey=$privateKey; DockerReady=$true }
