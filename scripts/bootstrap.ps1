[CmdletBinding()]
param(
    [ValidateSet("bootstrap", "deps", "build", "test", "ui-smoke", "pack", "dockerize", "publish", "deploy", "dev", "dev-down", "compose-up", "compose-down", "clean")]
    [string]$Task = "bootstrap",

    [string]$ImageName = "hostswitch-backend",
    [string]$ImageTag = "local",
    [string]$Registry = "",
    [int]$BackendPort = 18080,
    [int]$FrontendPort = 5173,
    [switch]$SkipDocker,
    [switch]$CleanInstall,
    [switch]$ForceDeps,
    [switch]$RefreshHostsPreview
)

$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
$BackendDir = Join-Path $Root "backend"
$FrontendDir = Join-Path $Root "frontend"
$DistDir = Join-Path $Root "dist"
$NpmCacheDir = Join-Path $Root ".npm-cache"
$DataDir = Join-Path $Root "data"
$HostsPreviewPath = Join-Path $DataDir "hosts.preview"
$ImageRef = "${ImageName}:${ImageTag}"

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Test-Command {
    param([string]$Name)
    return $null -ne (Get-Command $Name -ErrorAction SilentlyContinue)
}

function Invoke-Checked {
    param(
        [string]$FilePath,
        [string[]]$Arguments,
        [string]$WorkingDirectory = $Root
    )

    Write-Host "+ $FilePath $($Arguments -join ' ')" -ForegroundColor DarkGray
    Push-Location $WorkingDirectory
    try {
        & $FilePath @Arguments
        if ($LASTEXITCODE -ne 0) {
            throw "$FilePath failed with exit code $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }
}

function Assert-Node {
    if (-not (Test-Command "npm")) {
        throw "npm is required. Install Node.js 20+ and rerun this script."
    }
    if (-not (Test-Path $NpmCacheDir)) {
        New-Item -ItemType Directory -Path $NpmCacheDir | Out-Null
    }
    $env:npm_config_cache = $NpmCacheDir
}

function Test-FrontendDependencies {
    $required = @(
        (Join-Path $FrontendDir "node_modules\typescript\bin\tsc"),
        (Join-Path $FrontendDir "node_modules\vite\bin\vite.js"),
        (Join-Path $FrontendDir "node_modules\react\index.js")
    )
    foreach ($path in $required) {
        if (-not (Test-Path $path)) {
            return $false
        }
    }
    return $true
}

function Assert-Docker {
    if ($SkipDocker) {
        throw "Docker is required for this task unless Go is installed. Remove -SkipDocker or install Go."
    }
    if (-not (Test-Command "docker")) {
        throw "Docker is required for this task. Install Docker Desktop and rerun this script."
    }
}

function Install-Dependencies {
    Write-Step "Installing frontend dependencies"
    Assert-Node
    $nodeModules = Join-Path $FrontendDir "node_modules"
    if ((Test-Path $nodeModules) -and (Test-FrontendDependencies) -and -not $CleanInstall -and -not $ForceDeps) {
        Write-Host "frontend/node_modules already exists; skipping npm install. Use -ForceDeps to refresh or -CleanInstall for npm ci." -ForegroundColor Yellow
    } elseif ($CleanInstall -and (Test-Path (Join-Path $FrontendDir "package-lock.json"))) {
        Invoke-Checked "npm" @("ci") $FrontendDir
    } else {
        Invoke-Checked "npm" @("install") $FrontendDir
    }

    Write-Step "Preparing backend dependencies"
    if (Test-Command "go") {
        Invoke-Checked "go" @("mod", "tidy") $BackendDir
    } else {
        Assert-Docker
        Write-Host "Go is not installed locally; backend dependencies will be resolved during Docker build." -ForegroundColor Yellow
    }
}

function Build-Frontend {
    Write-Step "Building frontend"
    Assert-Node
    Invoke-Checked "npm" @("run", "build") $FrontendDir
}

function Build-Backend {
    Write-Step "Building backend"
    if (Test-Command "go") {
        Invoke-Checked "go" @("build", "-o", (Join-Path $DistDir "hostswitch.exe"), "./cmd/hostswitch") $BackendDir
    } else {
        Assert-Docker
        Invoke-DockerBuild
    }
}

function Invoke-Tests {
    Write-Step "Running frontend checks"
    Assert-Node
    Invoke-Checked "npm" @("run", "build") $FrontendDir

    Write-Step "Running backend checks"
    if (Test-Command "go") {
        Invoke-Checked "go" @("test", "./...") $BackendDir
    } else {
        Assert-Docker
        Invoke-DockerBuild
    }
}

function Invoke-DockerBuild {
    Write-Step "Building Docker image $ImageRef"
    Assert-Docker
    Invoke-Checked "docker" @("build", "-f", "backend/Dockerfile", "-t", $ImageRef, ".") $Root
}

function New-Package {
    Write-Step "Packing release artifact"
    if (-not (Test-Path $DistDir)) {
        New-Item -ItemType Directory -Path $DistDir | Out-Null
    }

    Build-Frontend
    if (Test-Command "go") {
        Build-Backend
    } else {
        Invoke-DockerBuild
    }

    $packagePath = Join-Path $DistDir "hostswitch-package.zip"
    if (Test-Path $packagePath) {
        Remove-Item -LiteralPath $packagePath -Force
    }

    $items = @(
        (Join-Path $Root "README.md"),
        (Join-Path $Root "docker-compose.yml"),
        (Join-Path $Root ".dockerignore"),
        (Join-Path $Root "backend"),
        (Join-Path $Root "frontend\dist")
    )
    Compress-Archive -Path $items -DestinationPath $packagePath -Force
    Write-Host "Package created: $packagePath" -ForegroundColor Green
}

function Publish-Image {
    if ([string]::IsNullOrWhiteSpace($Registry)) {
        throw "Provide -Registry, for example: ./scripts/bootstrap.ps1 -Task publish -Registry ghcr.io/your-org"
    }

    Invoke-DockerBuild
    $remoteRef = "$Registry/$ImageRef"
    Write-Step "Publishing $remoteRef"
    Invoke-Checked "docker" @("tag", $ImageRef, $remoteRef) $Root
    Invoke-Checked "docker" @("push", $remoteRef) $Root
}

function Deploy-Compose {
    Write-Step "Deploying with Docker Compose"
    Assert-Docker
    Invoke-Checked "docker" @("compose", "up", "-d", "--build") $Root
}

function Stop-Compose {
    Write-Step "Stopping Docker Compose stack"
    Assert-Docker
    Invoke-Checked "docker" @("compose", "down") $Root
}

function Get-SystemHostsPath {
    if ($IsWindows -or $env:OS -eq "Windows_NT") {
        return Join-Path $env:SystemRoot "System32\drivers\etc\hosts"
    }
    return "/etc/hosts"
}

function Sync-HostsPreview {
    if (-not (Test-Path $DataDir)) {
        New-Item -ItemType Directory -Path $DataDir | Out-Null
    }

    $systemHosts = Get-SystemHostsPath
    if (-not (Test-Path $systemHosts)) {
        Write-Host "System hosts file not found at $systemHosts; using existing preview if available." -ForegroundColor Yellow
        return
    }

    if ((Test-Path $HostsPreviewPath) -and -not $RefreshHostsPreview) {
        Write-Host "Using existing hosts preview at $HostsPreviewPath. Add -RefreshHostsPreview to copy the current system hosts file again." -ForegroundColor Yellow
        return
    }

    Copy-Item -LiteralPath $systemHosts -Destination $HostsPreviewPath -Force
    Write-Host "Seeded hosts preview from $systemHosts -> $HostsPreviewPath" -ForegroundColor Green
}

function Start-Dev {
    Write-Step "Starting backend container on localhost:$BackendPort"
    Assert-Docker
    Sync-HostsPreview

    $containerName = "hostswitch-backend-local-$BackendPort"
    $existing = docker ps -aq --filter "name=^/$containerName$"
    if ($existing) {
        Invoke-Checked "docker" @("rm", "-f", $containerName) $Root
    }

    Invoke-DockerBuild
    Invoke-Checked "docker" @(
        "run", "--rm", "-d",
        "--name", $containerName,
        "-p", "${BackendPort}:8080",
        "-v", "${DataDir}:/data",
        "-e", "HOSTSWITCH_ADDR=:8080",
        "-e", "HOSTSWITCH_DB_PATH=/data/hostswitch.db",
        "-e", "HOSTSWITCH_DATA_DIR=/data",
        "-e", "HOSTSWITCH_HOSTS_PATH=/data/hosts.preview",
        "-e", "HOSTSWITCH_ALLOWED_ORIGIN=http://localhost:$FrontendPort",
        $ImageRef
    ) $Root

    Write-Step "Starting frontend on localhost:$FrontendPort"
    Assert-Node
    $env:VITE_API_BASE = "http://localhost:$BackendPort"
    Invoke-Checked "npm" @("run", "dev", "--", "--port", "$FrontendPort") $FrontendDir
}

function Stop-Dev {
    Write-Step "Stopping local dev backend"
    Assert-Docker
    $containerName = "hostswitch-backend-local-$BackendPort"
    $existing = docker ps -aq --filter "name=^/$containerName$"
    if ($existing) {
        Invoke-Checked "docker" @("rm", "-f", $containerName) $Root
    } else {
        Write-Host "No dev backend container found for port $BackendPort."
    }
}

function Assert-PortAvailable {
    param([int]$Port)
    $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, $Port)
    try {
        $listener.Start()
    } catch {
        throw "Port $Port is already in use. Stop the process using it or choose another smoke-test port."
    } finally {
        $listener.Stop()
    }
}

function Invoke-UISmoke {
    Write-Step "Running UI smoke test"
    Assert-Docker
    Assert-Node

    $smokeDataDir = Join-Path $Root "data-ui-test"
    if (-not (Test-Path $smokeDataDir)) {
        New-Item -ItemType Directory -Path $smokeDataDir | Out-Null
    }
    Remove-Item -LiteralPath (Join-Path $smokeDataDir "hostswitch.db") -Force -ErrorAction SilentlyContinue
    @"
127.0.0.1 voxera.local whisper.local
192.168.1.20 grafana.local
127.0.0.1 localhost
"@ | Set-Content -LiteralPath (Join-Path $smokeDataDir "hosts.preview") -NoNewline

    $backendName = "hostswitch-ui-smoke"
    $frontendPortSmoke = 5174
    $backendPortSmoke = 18081
    $frontendProcess = $null

    docker rm -f $backendName 2>$null | Out-Null
    Assert-PortAvailable $frontendPortSmoke
    Assert-PortAvailable $backendPortSmoke
    Invoke-DockerBuild
    try {
        Invoke-Checked "docker" @(
            "run", "--rm", "-d",
            "--name", $backendName,
            "-p", "${backendPortSmoke}:8080",
            "-v", "${smokeDataDir}:/data",
            "-e", "HOSTSWITCH_ADDR=:8080",
            "-e", "HOSTSWITCH_DB_PATH=/data/hostswitch.db",
            "-e", "HOSTSWITCH_DATA_DIR=/data",
            "-e", "HOSTSWITCH_HOSTS_PATH=/data/hosts.preview",
            "-e", "HOSTSWITCH_ALLOWED_ORIGIN=http://localhost:$frontendPortSmoke",
            $ImageRef
        ) $Root

        $env:VITE_API_BASE = "http://localhost:$backendPortSmoke"
        $frontendProcess = Start-Process -FilePath "npm.cmd" -ArgumentList @("run", "dev", "--", "--port", "$frontendPortSmoke") -WorkingDirectory $FrontendDir -PassThru -WindowStyle Hidden
        Invoke-Checked "node" @((Join-Path $Root "scripts\ui-smoke.mjs")) $Root
    } finally {
        if ($frontendProcess -and -not $frontendProcess.HasExited) {
            Stop-Process -Id $frontendProcess.Id -Force
        }
        docker rm -f $backendName 2>$null | Out-Null
    }
}

function Clear-Artifacts {
    Write-Step "Cleaning generated artifacts"
    $targets = @(
        $DistDir,
        (Join-Path $FrontendDir "dist"),
        (Join-Path $FrontendDir "tsconfig.tsbuildinfo"),
        $NpmCacheDir
    )
    foreach ($target in $targets) {
        $resolved = Resolve-Path $target -ErrorAction SilentlyContinue
        if ($resolved -and $resolved.Path.StartsWith($Root.Path)) {
            Remove-Item -LiteralPath $resolved.Path -Recurse -Force
            Write-Host "Removed $($resolved.Path)"
        }
    }
}

switch ($Task) {
    "bootstrap" {
        Install-Dependencies
        Invoke-Tests
        if (Test-Command "go") {
            Invoke-DockerBuild
        }
    }
    "deps" { Install-Dependencies }
    "build" {
        if (-not (Test-Path $DistDir)) {
            New-Item -ItemType Directory -Path $DistDir | Out-Null
        }
        Build-Frontend
        Build-Backend
    }
    "test" { Invoke-Tests }
    "ui-smoke" { Invoke-UISmoke }
    "pack" { New-Package }
    "dockerize" { Invoke-DockerBuild }
    "publish" { Publish-Image }
    "deploy" { Deploy-Compose }
    "compose-up" { Deploy-Compose }
    "compose-down" { Stop-Compose }
    "dev" { Start-Dev }
    "dev-down" { Stop-Dev }
    "clean" { Clear-Artifacts }
}

Write-Host ""
Write-Host "Done: $Task" -ForegroundColor Green
