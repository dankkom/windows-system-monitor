# Instalacao one-click sem Inno Setup — gera config.toml interativo, instala PG/opcionais e registra tasks.
# Uso: powershell -ExecutionPolicy Bypass -File installer/install.ps1
#      Com args: .\install.ps1 -DbHost 192.168.1.10 -DbPassword "xxx" -InstallPostgres -WithSmartTools
param(
    [string]$InstallDir = "C:\Program Files\system-monitor",
    [string]$DbHost,
    [string]$DbPort = "5432",
    [string]$DbUser = "postgres",
    [string]$DbPassword,
    [string]$DbName = "system_monitor",
    [switch]$InstallPostgres,
    [switch]$WithSmartTools,
    [switch]$WithPawnIO,
    [switch]$SkipBuild,
    [switch]$NonInteractive
)
$ErrorActionPreference = "Stop"
if (-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "Execute como Administrador."
}
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

function Read-IfEmpty($current, $prompt, $default, $secret) {
    if (-not [string]::IsNullOrWhiteSpace($current)) { return $current }
    if ($NonInteractive) { return $default }
    if ($secret) {
        $sec = Read-Host -Prompt "$prompt (default: $default, vazio=gerar se InstallPostgres)" -AsSecureString
        $plain = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($sec))
        if ([string]::IsNullOrWhiteSpace($plain)) { return $default }
        return $plain
    } else {
        $val = Read-Host -Prompt "$prompt (default: $default)"
        if ([string]::IsNullOrWhiteSpace($val)) { return $default }
        return $val
    }
}

# Coleta interativa se não vier por param
if (-not $DbHost) { $DbHost = Read-IfEmpty $DbHost "PostgreSQL host" "localhost" $false }
if (-not $DbPort) { $DbPort = Read-IfEmpty $DbPort "PostgreSQL porta" "5432" $false }
if (-not $DbUser) { $DbUser = Read-IfEmpty $DbUser "PostgreSQL usuário" "postgres" $false }
# Senha: se InstallPostgres e vazio, gera; senão pede
if (-not $DbPassword) {
    if ($InstallPostgres -and -not $NonInteractive) {
        $DbPassword = Read-IfEmpty $DbPassword "PostgreSQL senha (vazio = gerar aleatória para instalação)" "" $true
        if ([string]::IsNullOrWhiteSpace($DbPassword)) {
            $DbPassword = -join ((48..57)+(65..90)+(97..122) | Get-Random -Count 16 | ForEach-Object { [char]$_ })
            Write-Host "Senha gerada: $DbPassword" -ForegroundColor Yellow
        }
    } elseif (-not $NonInteractive) {
        $DbPassword = Read-IfEmpty $DbPassword "PostgreSQL senha" "" $true
    }
}
if (-not $DbName) { $DbName = Read-IfEmpty $DbName "PostgreSQL banco" "system_monitor" $false }

# Pergunta opcionais se não veio por flag e não é non-interactive
if (-not $PSBoundParameters.ContainsKey('WithSmartTools') -and -not $NonInteractive) {
    $ans = Read-Host -Prompt "Instalar smartmontools (smartctl) via winget/choco? (S/n)"
    if ($ans -notmatch "^[nN]") { $WithSmartTools = $true }
}
if (-not $PSBoundParameters.ContainsKey('WithPawnIO') -and -not $NonInteractive) {
    $ans = Read-Host -Prompt "Tentar instalar PawnIO driver (309 sensores)? (s/N)"
    if ($ans -match "^[sSyY]") { $WithPawnIO = $true }
}
if (-not $PSBoundParameters.ContainsKey('InstallPostgres') -and -not $NonInteractive) {
    $ans = Read-Host -Prompt "Instalar PostgreSQL automaticamente se não estiver instalado? (S/n)"
    if ($ans -match "^[nN]") { $InstallPostgres = $false } else { $InstallPostgres = $true }
}

$GoSrc = Join-Path $Root "go\system-monitor.exe"
$LhmSrcDir = Join-Path $Root "build\lhm-dump"
if (-not $SkipBuild) {
    Write-Host "Building system-monitor.exe..." -ForegroundColor Cyan
    $goBin = "C:\Program Files\Go\bin\go.exe"; if (-not (Test-Path $goBin)) { $goBin = "go" }
    & $goBin build -o $GoSrc ./go/cmd/monitor
    if ($LASTEXITCODE -ne 0) { throw "go build falhou" }
    Write-Host "Publishing lhm-dump..." -ForegroundColor Cyan
    & dotnet publish (Join-Path $Root "go\lhm-dump\lhm-dump.csproj") -c Release -o $LhmSrcDir 2>&1 | Out-Null
    if ($LASTEXITCODE -ne 0) { Write-Warning "dotnet publish falhou, helper pode ficar ausente (sensores -> no_sensor)" }
}

New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
Copy-Item $GoSrc (Join-Path $InstallDir "system-monitor.exe") -Force
if (Test-Path $LhmSrcDir) {
    Copy-Item (Join-Path $LhmSrcDir "lhm-dump.exe") $InstallDir -Force -ErrorAction SilentlyContinue
    Copy-Item (Join-Path $LhmSrcDir "*.dll") $InstallDir -Force -ErrorAction SilentlyContinue
} elseif (Test-Path "C:\tools\lhm-dump\lhm-dump.exe") {
    Copy-Item "C:\tools\lhm-dump\lhm-dump.exe" $InstallDir -Force -ErrorAction SilentlyContinue
    Copy-Item "C:\tools\lhm-dump\*.dll" $InstallDir -Force -ErrorAction SilentlyContinue
}
# Config
Copy-Item (Join-Path $Root "config.toml.example") (Join-Path $InstallDir "config.toml.example") -Force -ErrorAction SilentlyContinue

# Gera config.toml com senha em texto plano (decisão do usuário)
$encUser = [Uri]::EscapeDataString($DbUser)
$encPass = [Uri]::EscapeDataString($DbPassword)
$encHost = $DbHost
$databaseUrl = "postgresql://${encUser}:${encPass}@${encHost}:${DbPort}/${DbName}"
$configToml = @"
# Gerado por installer/install.ps1 em $(Get-Date -Format "yyyy-MM-dd HH:mm:ss")
[db]
url = "$databaseUrl"
connect_timeout = 10
retry_seconds = 30
buffer_max_bytes = 2147483648

[dashboard]
host = "127.0.0.1"
port = 8501
timezone = "America/Sao_Paulo"

[intervals]
cpu = 10
memory = 10
disk_io = 10
disk_usage = 60
disk_physical = 300
disk_smart = 300
network = 10
gpu = 10
sensors = 15
processes = 30
connections = 30
services = 60
system = 60
eventlog = 60

[collector]
top_processes = 50
hostname = ""

[power]
aux_baseline_w = 24
psu_efficiency = 0.90
gpu_idle_w = 10
gpu_max_w = 160

[retention]
enabled = false
batch_limit = 50000
batch_sleep = 0.1
processes = "30 days"
connections = "7 days"
sensors = "90 days"
cpu = "90 days"
memory = "90 days"
gpu = "90 days"
heartbeat = "30 days"
eventlog = "30 days"
disk_io = "90 days"
net_io = "90 days"

[log]
level = "INFO"
"@
$configPath = Join-Path $InstallDir "config.toml"
$configToml | Set-Content -Path $configPath -Encoding utf8
Write-Host "Gerado $configPath" -ForegroundColor Green
Write-Host "DATABASE_URL=$databaseUrl" -ForegroundColor Gray

New-Item -ItemType Directory -Path (Join-Path $InstallDir "sql") -Force | Out-Null
Copy-Item (Join-Path $Root "sql\schema.sql") (Join-Path $InstallDir "sql\schema.sql") -Force
Copy-Item (Join-Path $Root "scripts\install_tasks_go.ps1") (Join-Path $InstallDir "install_tasks_go.ps1") -Force
Copy-Item (Join-Path $Root "scripts\install_retention_go.ps1") (Join-Path $InstallDir "install_retention_go.ps1") -Force
Copy-Item (Join-Path $Root "installer\install_postgres.ps1") (Join-Path $InstallDir "install_postgres.ps1") -Force -ErrorAction SilentlyContinue
Copy-Item (Join-Path $Root "installer\install_optional.ps1") (Join-Path $InstallDir "install_optional.ps1") -Force -ErrorAction SilentlyContinue

if ($InstallPostgres) {
    Write-Host "Verificando/instalando PostgreSQL (versão mais recente)..." -ForegroundColor Cyan
    & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $InstallDir "install_postgres.ps1") -SuperPassword $DbPassword -Port ([int]$DbPort) 2>&1 | Write-Host
}

if ($WithSmartTools -or $WithPawnIO) {
    Write-Host "Instalando opcionais..." -ForegroundColor Cyan
    $args = @()
    if ($WithSmartTools) { $args += "-SmartTools" }
    if ($WithPawnIO) { $args += "-PawnIO" }
    & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $InstallDir "install_optional.ps1") @args 2>&1 | Write-Host
}

Write-Host "Aplicando schema (system-monitor --init)..." -ForegroundColor Cyan
& (Join-Path $InstallDir "system-monitor.exe") --init 2>&1 | Out-String | Write-Host
if ($LASTEXITCODE -ne 0) { Write-Warning "init falhou - verifique config.toml DATABASE_URL e se PostgreSQL está rodando" }

Write-Host "Registrando tarefas SYSTEM..." -ForegroundColor Cyan
& (Join-Path $InstallDir "install_tasks_go.ps1")
& (Join-Path $InstallDir "install_retention_go.ps1") 2>&1 | Write-Host
Write-Host "Instalação concluída em $InstallDir" -ForegroundColor Green
Write-Host "Dashboard: http://localhost:8501  (SystemMonitor / SystemMonitor-Dashboard)"
Write-Host "Config: $configPath  (senha em texto plano)"
