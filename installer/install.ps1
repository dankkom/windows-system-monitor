# Instalacao one-click sem Inno Setup - copia para Program Files e registra tarefas.
# Uso: powershell -ExecutionPolicy Bypass -File installer/install.ps1
# Requer: Go 1.27+ e .NET SDK 8 (para build) e PostgreSQL externo.
param(
    [string]$InstallDir = "C:\Program Files\system-monitor",
    [switch]$SkipBuild
)
$ErrorActionPreference = "Stop"
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "Execute como Administrador."
}
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$GoSrc = Join-Path $Root "go\monitor-go.exe"
$LhmSrcDir = Join-Path $Root "build\lhm-dump"
if (-not $SkipBuild) {
    Write-Host "Building monitor-go.exe..." -ForegroundColor Cyan
    $goBin = "C:\Program Files\Go\bin\go.exe"; if (-not (Test-Path $goBin)) { $goBin = "go" }
    & $goBin build -o $GoSrc ./go/cmd/monitor
    if ($LASTEXITCODE -ne 0) { throw "go build falhou" }
    Write-Host "Publishing lhm-dump..." -ForegroundColor Cyan
    $dotnet = "dotnet"; & $dotnet publish (Join-Path $Root "go\lhm-dump\lhm-dump.csproj") -c Release -o $LhmSrcDir 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) { Write-Warning "dotnet publish falhou, helper pode ficar ausente (sensores -> no_sensor)" }
}
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Copy-Item $GoSrc (Join-Path $InstallDir "monitor-go.exe") -Force
if (Test-Path $LhmSrcDir) {
    Copy-Item (Join-Path $LhmSrcDir "lhm-dump.exe") $InstallDir -Force -ErrorAction SilentlyContinue
    Copy-Item (Join-Path $LhmSrcDir "*.dll") $InstallDir -Force -ErrorAction SilentlyContinue
} elseif (Test-Path (Join-Path $Root "C:\tools\lhm-dump\lhm-dump.exe")) {
    Copy-Item "C:\tools\lhm-dump\lhm-dump.exe" $InstallDir -Force -ErrorAction SilentlyContinue
    Copy-Item "C:\tools\lhm-dump\*.dll" $InstallDir -Force -ErrorAction SilentlyContinue
}
Copy-Item (Join-Path $Root ".env.example") (Join-Path $InstallDir ".env.example") -Force -ErrorAction SilentlyContinue
if (-not (Test-Path (Join-Path $InstallDir ".env"))) {
    Copy-Item (Join-Path $InstallDir ".env.example") (Join-Path $InstallDir ".env") -Force
    Write-Host "Criado $InstallDir\.env - EDITE DATABASE_URL antes de iniciar!" -ForegroundColor Yellow
}
New-Item -ItemType Directory -Path (Join-Path $InstallDir "sql") -Force | Out-Null
Copy-Item (Join-Path $Root "sql\schema.sql") (Join-Path $InstallDir "sql\schema.sql") -Force
Copy-Item (Join-Path $Root "scripts\install_tasks_go.ps1") (Join-Path $InstallDir "install_tasks_go.ps1") -Force
Copy-Item (Join-Path $Root "scripts\install_retention_go.ps1") (Join-Path $InstallDir "install_retention_go.ps1") -Force

Write-Host "Aplicando schema (monitor-go --init)..." -ForegroundColor Cyan
& (Join-Path $InstallDir "monitor-go.exe") --init 2>&1 | Out-String | Write-Host
if ($LASTEXITCODE -ne 0) { Write-Warning "init falhou - verifique DATABASE_URL em $InstallDir\.env e se PostgreSQL esta rodando" }

Write-Host "Registrando tarefas SYSTEM..." -ForegroundColor Cyan
& (Join-Path $InstallDir "install_tasks_go.ps1")
& (Join-Path $InstallDir "install_retention_go.ps1") 2>&1 | Write-Host
Write-Host "Instalacao concluida em $InstallDir" -ForegroundColor Green
Write-Host "Dashboard: http://localhost:8501  (SystemMonitor-Go / SystemMonitor-Go-Dashboard)"
