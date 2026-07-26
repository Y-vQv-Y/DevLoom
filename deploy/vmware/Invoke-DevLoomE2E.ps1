[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$VMIP,
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path,
    [string]$PrivateKey = (Join-Path (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path '.vmware\keys\id_ed25519')
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path $RepoRoot).Path
$ssh = @('-i', $PrivateKey, '-o', 'StrictHostKeyChecking=no', '-o', 'UserKnownHostsFile=NUL', "devloom@$VMIP")
$archive = Join-Path $env:TEMP 'devloom-source-e2e.tar.gz'
$fileList = Join-Path $env:TEMP 'devloom-source-e2e-files.txt'

$files = @(& git -C $repo ls-files --cached --others --exclude-standard)
if ($LASTEXITCODE -ne 0 -or $files.Count -eq 0) { throw 'failed to enumerate source files' }
[IO.File]::WriteAllLines($fileList, $files, [Text.UTF8Encoding]::new($false))
tar.exe -czf $archive -C $repo -T $fileList
if ($LASTEXITCODE -ne 0) { throw 'source archive creation failed' }
scp.exe -i $PrivateKey -o StrictHostKeyChecking=no -o UserKnownHostsFile=NUL $archive "devloom@${VMIP}:/tmp/devloom-source-e2e.tar.gz"
if ($LASTEXITCODE -ne 0) { throw 'source upload failed' }

& ssh.exe @ssh @"
set -Eeuo pipefail
sudo find /opt/devloom-src -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +
sudo chown -R devloom:devloom /opt/devloom-src
tar -xzf /tmp/devloom-source-e2e.tar.gz -C /opt/devloom-src
cd /opt/devloom-src
chmod +x deploy/e2e/run-linux.sh
if ! docker buildx version >/dev/null 2>&1; then
  sudo apt-get update
  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y docker-buildx
fi
docker buildx version
pull_image() { local image=`$1 path=`$1; [[ "`$path" == */* ]] || path="library/`$path"; local mirror="docker.m.daocloud.io/`$path"; if ! docker image inspect "`$image" >/dev/null 2>&1; then docker pull "`$mirror"; docker tag "`$mirror" "`$image"; fi; }
pull_image golang:1.25.8-bookworm
pull_image node:22.22.1-alpine3.23
sudo mkdir -p /opt/devloom-cache/go-mod /opt/devloom-cache/go-build
sudo chown -R devloom:devloom /opt/devloom-cache
docker run --rm --dns 223.5.5.5 -e GOPROXY=https://goproxy.cn,direct -v /opt/devloom-src:/repo -v /opt/devloom-cache/go-mod:/go/pkg/mod -v /opt/devloom-cache/go-build:/root/.cache/go-build -w /repo/backend golang:1.25.8-bookworm go test ./...
docker run --rm --dns 223.5.5.5 -e COREPACK_NPM_REGISTRY=https://registry.npmmirror.com -e NPM_REGISTRY=https://registry.npmmirror.com -v /opt/devloom-src:/repo node:22.22.1-alpine3.23 sh /repo/deploy/e2e/test-frontend.sh
sudo deploy/e2e/run-linux.sh '$VMIP'
"@
if ($LASTEXITCODE -ne 0) { throw 'full deployment failed in VMware guest' }

[pscustomobject]@{ VMIP=$VMIP; Source='/opt/devloom-src'; URL="http://${VMIP}:8080"; DeploymentReady=$true }
