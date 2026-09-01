# setup.ps1 - Bootstrap reproduzível idempotente
# Uso: powershell -ExecutionPolicy Bypass -File .\setup.ps1
param(
    [string]$PythonCmd = "python",
    [string]$DbName = "system_monitor",
    [string]$DbUser = "postgres",
    [string]$DbHost = "localhost",
    [string]$PgPassFile = "$env:APPDATA\postgresql\pgpass.conf"
)
$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot

Write-Host "== System Monitor Setup ==" -ForegroundColor Cyan
Write-Host "Root: $Root"

# 1. Python
try { & $PythonCmd --version | Out-Host } catch { Write-Error "Python não encontrado. Instale Python 3.10+ e adicione ao PATH."; exit 1 }

# 2. Venv
$VenvDir = Join-Path $Root "venv"
$VenvPython = Join-Path $VenvDir "Scripts\python.exe"
$VenvPip = Join-Path $VenvDir "Scripts\pip.exe"
$VenvPythonw = Join-Path $VenvDir "Scripts\pythonw.exe"
if (-not (Test-Path $VenvPython)) {
    Write-Host "Criando venv..." -ForegroundColor Yellow
    & $PythonCmd -m venv $VenvDir
} else { Write-Host "venv já existe: $VenvDir" -ForegroundColor Green }
if (-not (Test-Path $VenvPythonw) -and (Test-Path (Join-Path $VenvDir "Scripts\python.exe"))) {
    Copy-Item (Join-Path $VenvDir "Scripts\python.exe") $VenvPythonw -Force -ErrorAction SilentlyContinue
}

# 3. Deps
Write-Host "Instalando requirements..." -ForegroundColor Yellow
& $VenvPython -m pip install --upgrade pip
& $VenvPip install -r (Join-Path $Root "requirements.txt")
& $VenvPip install -r (Join-Path $Root "dashboard\requirements_light.txt")
Write-Host "Deps OK (Flask leve, sem Streamlit)" -ForegroundColor Green

# 4. .env
$EnvFile = Join-Path $Root ".env"
$EnvExample = Join-Path $Root ".env.example"
if (-not (Test-Path $EnvFile)) {
    Copy-Item $EnvExample $EnvFile
    Write-Host ".env criado a partir de .env.example - AJUSTE DATABASE_URL" -ForegroundColor Yellow
} else { Write-Host ".env já existe" -ForegroundColor Green }

# 5. LibreHardwareMonitor
$LhmDir = "C:\tools\LibreHardwareMonitor"
$LhmDll = Join-Path $LhmDir "LibreHardwareMonitorLib.dll"
if (-not (Test-Path $LhmDll)) {
    Write-Host "Baixando LibreHardwareMonitor 0.9.6..." -ForegroundColor Yellow
    New-Item -ItemType Directory -Force -Path "C:\tools" | Out-Null
    $zip = "C:\tools\LibreHardwareMonitor.zip"
    Invoke-WebRequest -Uri "https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/releases/download/v0.9.6/LibreHardwareMonitor.zip" -OutFile $zip -UseBasicParsing
    Expand-Archive -Path $zip -DestinationPath $LhmDir -Force
    Write-Host "LHM extraído em $LhmDir" -ForegroundColor Green
} else { Write-Host "LHM já existe: $LhmDll" -ForegroundColor Green }

# 6. smartmontools
$SmartCtl = "C:\Program Files\smartmontools\bin\smartctl.exe"
if (-not (Test-Path $SmartCtl)) {
    Write-Host "Instalando smartmontools via winget..." -ForegroundColor Yellow
    try { winget install --id smartmontools.smartmontools --silent --accept-package-agreements --accept-source-agreements | Out-Host } catch { Write-Host "winget falhou, instale manualmente: https://www.smartmontools.org/" -ForegroundColor Yellow }
} else { Write-Host "smartmontools já existe" -ForegroundColor Green }

# 7. PawnIO (requer admin - apenas avisa)
try { sc.exe query PawnIO | Out-Null; if ($LASTEXITCODE -eq 0) { Write-Host "PawnIO já instalado (RUNNING)" -ForegroundColor Green } else { throw } } catch {
    $PawnSetup = "C:\tools\PawnIO_setup.exe"
    if (Test-Path $PawnSetup) { Write-Host "PawnIO encontrado mas não instalado. Rode como admin: Start-Process $PawnSetup -ArgumentList '/S' -Verb RunAs" -ForegroundColor Yellow }
    else {
        Write-Host "Baixando PawnIO_setup.exe..." -ForegroundColor Yellow
        Invoke-WebRequest -Uri "https://github.com/namazso/PawnIO.Setup/releases/download/2.2.0/PawnIO_setup.exe" -OutFile $PawnSetup -UseBasicParsing
        Write-Host "PawnIO baixado. Instale como admin para liberar temps SuperIO: Start-Process $PawnSetup -Verb RunAs" -ForegroundColor Yellow
    }
}

# 8. PostgreSQL DB + schema
$Psql = "C:\Program Files\PostgreSQL\18\bin\psql.exe"
if (-not (Test-Path $Psql)) { $Psql = "psql" }
Write-Host "Verificando PostgreSQL..." -ForegroundColor Yellow
try {
    # tenta criar DB (ignora se já existe)
    $env:PGPASSWORD = $null
    # lê senha de .env ou pgpass
    $dbUrl = (Get-Content $EnvFile | Where-Object { $_ -match "DATABASE_URL" } | ForEach-Object { $_ -replace ".*postgresql://[^:]+:([^@]+)@.*","`$1" })
    if ($dbUrl) { $env:PGPASSWORD = $dbUrl.Trim() }
    & $Psql -U $DbUser -h $DbHost -d postgres -c "CREATE DATABASE $DbName OWNER $DbUser;" 2>&1 | Out-String | Select-Object -First 5 | Out-Host
    & $Psql -U $DbUser -h $DbHost -d $DbName -f (Join-Path $Root "sql\schema.sql") 2>&1 | Select-Object -Last 5 | Out-Host
    & $Psql -U $DbUser -h $DbHost -d $DbName -f (Join-Path $Root "sql\disk_extra.sql") 2>&1 | Select-Object -Last 5 | Out-Host
    Write-Host "DB $DbName OK" -ForegroundColor Green
} catch { Write-Host "Aviso DB: $_" -ForegroundColor Yellow; Write-Host "Crie manualmente: psql -U postgres -h localhost -d postgres -c 'CREATE DATABASE system_monitor OWNER postgres;'" -ForegroundColor Yellow }

# 9. Teste dry-run
Write-Host "Teste dry-run..." -ForegroundColor Yellow
& $VenvPython (Join-Path $Root "monitor.py") --dry-run 2>&1 | Select-String "DRY" | Select-Object -First 5 | Out-Host

# 10. Logs dir
New-Item -ItemType Directory -Force -Path (Join-Path $Root "logs") | Out-Null
if (-not (Test-Path (Join-Path $Root "logs\.gitkeep"))) { New-Item -ItemType File -Path (Join-Path $Root "logs\.gitkeep") -Force | Out-Null }

Write-Host "`n== Setup concluído ==" -ForegroundColor Cyan
Write-Host "Próximos passos:"
Write-Host "  1. Ajuste .env DATABASE_URL se necessário"
Write-Host "  2. .\venv\Scripts\python.exe monitor.py --once   # teste real"
Write-Host "  3. Start-Process .\venv\Scripts\pythonw.exe -ArgumentList 'C:\scripts\system-monitor\monitor.py' -Verb RunAs  # elevado 309 sensores"
Write-Host "  4. .\venv\Scripts\waitress-serve.exe --port=8501 --host=0.0.0.0 dashboard.app:app  # dashboard Flask leve"
Write-Host "  5. Como admin: .\install_task_elevated.ps1  +  powershell -File .\setup_autostart.ps1"
