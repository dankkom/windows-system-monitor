# Registra as tarefas persistentes para o binario Go; requer PowerShell elevado.
$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$isAdmin = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) { throw "Execute este script como Administrador." }

$GoExe = Join-Path $Root "go\system-monitor.exe"
if (-not (Test-Path $GoExe)) {
    Write-Host "Compilando system-monitor.exe..." -ForegroundColor Cyan
    $goBin = "C:\Program Files\Go\bin\go.exe"
    if (-not (Test-Path $goBin)) { $goBin = "go" }
    & $goBin build -o $GoExe ./go/cmd/monitor
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path $GoExe)) { throw "Falha ao compilar Go binary." }
}

# Verifica schema
$env:PGPASSWORD = $null
# tenta via psql se disponivel, senao via Go --once com dry
$ErrorActionPreference = "Continue"
& $GoExe --once 2>&1 | Select-Object -First 2 | Out-Null
$ErrorActionPreference = "Stop"

$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit (New-TimeSpan -Seconds 0) -MultipleInstances IgnoreNew
$trigger = New-ScheduledTaskTrigger -AtStartup
$principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest

# Go binary: collector loop (default) e dashboard (--serve) como tarefas separadas
$monitorAction = New-ScheduledTaskAction -Execute $GoExe -WorkingDirectory $Root
$dashboardAction = New-ScheduledTaskAction -Execute $GoExe -Argument "--serve" -WorkingDirectory $Root

"SystemMonitor", "SystemMonitor-Dashboard", "SystemMonitor-Go", "SystemMonitor-Go-Dashboard" | ForEach-Object {
    $existing = Get-ScheduledTask -TaskName $_ -ErrorAction SilentlyContinue
    if ($existing -and $existing.State -eq "Running") { Stop-ScheduledTask -TaskName $_ }
}

# Registra com nomes novos (SystemMonitor); mantém limpeza de antigos Go para migração
Register-ScheduledTask -TaskName "SystemMonitor" -Action $monitorAction -Trigger $trigger -Settings $settings -Principal $principal -Description "Coleta de telemetria no boot" -Force | Out-Null
Register-ScheduledTask -TaskName "SystemMonitor-Dashboard" -Action $dashboardAction -Trigger $trigger -Settings $settings -Principal $principal -Description "Dashboard no boot" -Force | Out-Null

Start-ScheduledTask -TaskName "SystemMonitor"
Start-ScheduledTask -TaskName "SystemMonitor-Dashboard"
Write-Host "Tarefas SystemMonitor e SystemMonitor-Dashboard registradas e iniciadas." -ForegroundColor Green
Write-Host "Tarefas antigas SystemMonitor-Go/SystemMonitor-Go-Dashboard foram removidas se existiam."
