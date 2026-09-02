# Registra task diaria 02:00 para retencao Go; requer PowerShell elevado.
# So executa DELETE se ENABLE_RETENTION=true em .env (monitor-go --retention verifica).
$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$isAdmin = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) { throw "Execute este script como Administrador." }

$GoExe = Join-Path $Root "go\monitor-go.exe"
if (-not (Test-Path $GoExe)) {
    Write-Host "Compilando monitor-go.exe..." -ForegroundColor Cyan
    $goBin = "C:\Program Files\Go\bin\go.exe"
    if (-not (Test-Path $goBin)) { $goBin = "go" }
    & $goBin build -o $GoExe ./go/cmd/monitor
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path $GoExe)) { throw "Falha ao compilar Go binary." }
}

# Valida DATABASE_URL e schema antes de registrar
& $GoExe --init 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) { Write-Warning "Aviso: --init falhou, verifique DATABASE_URL em .env" }

$Action = New-ScheduledTaskAction -Execute $GoExe -Argument "--retention" -WorkingDirectory $Root
$Trigger = New-ScheduledTaskTrigger -Daily -At 02:00
$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable
$Principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
Unregister-ScheduledTask -TaskName "SystemMonitor-Go-Retention" -Confirm:$false -ErrorAction SilentlyContinue
Unregister-ScheduledTask -TaskName "SystemMonitor-Retention" -Confirm:$false -ErrorAction SilentlyContinue
Register-ScheduledTask -TaskName "SystemMonitor-Go-Retention" -Action $Action -Trigger $Trigger -Settings $Settings -Principal $Principal -Description "Retencao opcional system_monitor Go (so roda se ENABLE_RETENTION=true)" | Out-Null
Write-Host "Task SystemMonitor-Go-Retention criada (02:00 diario, inativa se ENABLE_RETENTION=false)" -ForegroundColor Green
Write-Host "Teste: monitor-go --retention-dry-run  ou  monitor-go --retention"
