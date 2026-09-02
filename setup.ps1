# Bootstrap idempotente: dependencias, banco, coleta e dashboard no boot.
param(
    [string]$PythonCmd = "python",
    [string]$DbName = "system_monitor",
    [string]$DbUser = "postgres",
    [string]$DbHost = "localhost",
    [int]$DbPort = 5432,
    [switch]$Elevated
)

$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot
$isAdmin = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "Solicitando permissao de administrador..." -ForegroundColor Yellow
    $process = Start-Process powershell.exe -Verb RunAs -Wait -PassThru -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`" -Elevated"
    exit $process.ExitCode
}

function Read-Setting([string]$Label, [string]$Default) {
    $value = Read-Host "$Label [$Default]"
    if ([string]::IsNullOrWhiteSpace($value)) { return $Default }
    return $value.Trim()
}

function Read-Decimal([string]$Label, [string]$Default) {
    $raw = (Read-Setting $Label $Default).Replace(',', '.')
    $value = 0.0
    if (-not [double]::TryParse($raw, [Globalization.NumberStyles]::Float, [Globalization.CultureInfo]::InvariantCulture, [ref]$value)) {
        throw "$Label deve ser um numero."
    }
    return $value
}

function Set-EnvSetting([string]$Path, [string]$Name, [string]$Value) {
    $lines = @(Get-Content $Path)
    if ($lines -match "^$Name=") {
        $lines = $lines -replace "^$Name=.*$", "$Name=$Value"
    } else {
        $lines += "$Name=$Value"
    }
    Set-Content -Path $Path -Value $lines -Encoding utf8
}

function Invoke-Psql([string[]]$Arguments, [string]$Failure) {
    $previous = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $output = & $Psql @Arguments 2>&1
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = $previous
    if ($exitCode -ne 0) { throw "$Failure`n$($output | Out-String)" }
    return $output
}

Write-Host "== System Monitor Setup ==" -ForegroundColor Cyan
Write-Host "Projeto: $Root"
Write-Host "Configuracao PostgreSQL (Enter mantem o valor entre colchetes)" -ForegroundColor Cyan
$DbHost = Read-Setting "Host" $DbHost
$DbPort = [int](Read-Setting "Porta" $DbPort)
$DbName = Read-Setting "Banco" $DbName
$DbUser = Read-Setting "Usuario" $DbUser
$DbPassword = [System.Net.NetworkCredential]::new('', (Read-Host "Senha (Enter usa pgpass.conf)" -AsSecureString)).Password
if ($DbName -notmatch '^[A-Za-z_][A-Za-z0-9_]*$' -or $DbUser -notmatch '^[A-Za-z_][A-Za-z0-9_]*$') {
    throw "Banco e usuario devem conter apenas letras, numeros e _."
}

Write-Host "[1/6] Python e ambiente virtual" -ForegroundColor Cyan
try { & $PythonCmd --version | Out-Host } catch { throw "Python nao encontrado. Instale Python 3.10+ e adicione-o ao PATH." }
$VenvDir = Join-Path $Root ".venv"
$VenvPython = Join-Path $VenvDir "Scripts\python.exe"
$VenvPip = Join-Path $VenvDir "Scripts\pip.exe"
if (Get-Command uv -ErrorAction SilentlyContinue) {
    & uv --no-cache sync --frozen --project $Root
} else {
    Write-Host "Aviso: uv nao encontrado; usando pip sem lock." -ForegroundColor Yellow
    if (-not (Test-Path $VenvPython)) { & $PythonCmd -m venv $VenvDir }
    & $VenvPython -m pip install --upgrade pip
    & $VenvPip install -r (Join-Path $Root "requirements.txt")
}

Write-Host "[2/6] Configuracao" -ForegroundColor Cyan
$EnvFile = Join-Path $Root ".env"
if (-not (Test-Path $EnvFile)) { Copy-Item (Join-Path $Root ".env.example") $EnvFile }
$DbPasswordUrl = if ($DbPassword) { ":$([uri]::EscapeDataString($DbPassword))" } else { "" }
$DatabaseUrl = "postgresql://$([uri]::EscapeDataString($DbUser))${DbPasswordUrl}@$DbHost`:$DbPort/$([uri]::EscapeDataString($DbName))"
Set-EnvSetting $EnvFile "DATABASE_URL" $DatabaseUrl
Set-EnvSetting $EnvFile "DASHBOARD_HOST" "127.0.0.1"
Set-EnvSetting $EnvFile "DASHBOARD_PORT" "8501"
Write-Host "Estimativa de energia (valores podem ser recalibrados depois no .env)" -ForegroundColor Cyan
$PowerBaseline = Read-Decimal "Consumo auxiliar estimado em watts" "30"
$PowerEfficiency = Read-Decimal "Eficiencia da fonte (0 a 1)" "0.90"
if ($PowerBaseline -lt 0 -or $PowerEfficiency -le 0 -or $PowerEfficiency -gt 1) {
    throw "Baseline deve ser >= 0 e eficiencia deve estar entre 0 (exclusivo) e 1."
}
$PowerGpuIdle = Read-Host "GPU em repouso, W (opcional; Enter desativa modelo GPU)"
$PowerGpuMax = ""
if (-not [string]::IsNullOrWhiteSpace($PowerGpuIdle)) {
    $PowerGpuMax = Read-Host "GPU em carga maxima, W"
    $PowerGpuIdle = $PowerGpuIdle.Replace(',', '.')
    $PowerGpuMax = $PowerGpuMax.Replace(',', '.')
    $gpuIdleNumber = 0.0
    $gpuMaxNumber = 0.0
    if (-not [double]::TryParse($PowerGpuIdle, [Globalization.NumberStyles]::Float, [Globalization.CultureInfo]::InvariantCulture, [ref]$gpuIdleNumber) -or
        -not [double]::TryParse($PowerGpuMax, [Globalization.NumberStyles]::Float, [Globalization.CultureInfo]::InvariantCulture, [ref]$gpuMaxNumber) -or
        $gpuIdleNumber -lt 0 -or $gpuMaxNumber -lt $gpuIdleNumber) {
        throw "Potencias da GPU devem ser numeros, com maxima >= repouso >= 0."
    }
}
Set-EnvSetting $EnvFile "DASHBOARD_TIMEZONE" "America/Sao_Paulo"
Set-EnvSetting $EnvFile "POWER_AUX_BASELINE_W" $PowerBaseline.ToString([Globalization.CultureInfo]::InvariantCulture)
Set-EnvSetting $EnvFile "POWER_PSU_EFFICIENCY" $PowerEfficiency.ToString([Globalization.CultureInfo]::InvariantCulture)
Set-EnvSetting $EnvFile "POWER_GPU_IDLE_W" $PowerGpuIdle
Set-EnvSetting $EnvFile "POWER_GPU_MAX_W" $PowerGpuMax

Write-Host "[3/6] Dependencias de hardware" -ForegroundColor Cyan
$LhmDir = "C:\tools\LibreHardwareMonitor"
if (-not (Test-Path (Join-Path $LhmDir "LibreHardwareMonitorLib.dll"))) {
    New-Item -ItemType Directory -Force -Path "C:\tools" | Out-Null
    $zip = "C:\tools\LibreHardwareMonitor.zip"
    Invoke-WebRequest "https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/releases/download/v0.9.6/LibreHardwareMonitor.zip" -OutFile $zip
    Expand-Archive -Path $zip -DestinationPath $LhmDir -Force
}
if (-not (Test-Path "C:\Program Files\smartmontools\bin\smartctl.exe")) {
    Write-Host "Aviso: smartmontools ausente; instale com winget para dados SMART." -ForegroundColor Yellow
}

Write-Host "[4/6] PostgreSQL e schema" -ForegroundColor Cyan
$Psql = "C:\Program Files\PostgreSQL\18\bin\psql.exe"
if (-not (Test-Path $Psql)) { $Psql = "psql" }
if ($DbPassword) { $env:PGPASSWORD = $DbPassword }
$exists = Invoke-Psql -Arguments @("-U", $DbUser, "-h", $DbHost, "-p", $DbPort, "-d", "postgres", "-tAc", "SELECT 1 FROM pg_database WHERE datname = '$DbName'") -Failure "Nao foi possivel conectar ao PostgreSQL em $DbHost`:$DbPort."
if (-not ($exists -match '1')) {
    Write-Host "Criando banco $DbName..." -ForegroundColor Yellow
    Invoke-Psql -Arguments @("-U", $DbUser, "-h", $DbHost, "-p", $DbPort, "-d", "postgres", "-c", "CREATE DATABASE $DbName OWNER $DbUser;") -Failure "Nao foi possivel criar o banco $DbName." | Out-Host
}
Invoke-Psql -Arguments @("-U", $DbUser, "-h", $DbHost, "-p", $DbPort, "-d", $DbName, "-f", (Join-Path $Root "sql\schema.sql")) -Failure "Falha ao aplicar sql\schema.sql." | Out-Host

Write-Host "[5/6] Teste de coleta" -ForegroundColor Cyan
$ErrorActionPreference = "Continue"
$dryRunOutput = & $VenvPython (Join-Path $Root "monitor_pkg\main.py") --dry-run 2>&1
$dryRunExit = $LASTEXITCODE
$ErrorActionPreference = "Stop"
$dryRunOutput | Select-String "\[DRY" | Select-Object -First 3 | Out-Host
if ($dryRunExit -ne 0) { throw "Dry-run do monitor falhou (codigo $dryRunExit)." }

Write-Host "[6/6] Tarefas de inicializacao" -ForegroundColor Cyan
& (Join-Path $Root "scripts\install_tasks.ps1")
$dashboardReady = $false
for ($attempt = 0; $attempt -lt 15; $attempt++) {
    Start-Sleep -Seconds 1
    try {
        if ((Invoke-WebRequest "http://127.0.0.1:8501/api/health" -UseBasicParsing -TimeoutSec 2).StatusCode -eq 200) { $dashboardReady = $true; break }
    } catch {}
}
if (-not $dashboardReady) { throw "Dashboard nao respondeu ao healthcheck em 15 segundos." }

Write-Host "`nInstalacao concluida." -ForegroundColor Green
Write-Host "Coleta e dashboard iniciam automaticamente com o Windows."
Write-Host "Dashboard: http://127.0.0.1:8501"
