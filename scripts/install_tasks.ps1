# Registra as tarefas persistentes; requer PowerShell elevado.
$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$isAdmin = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) { throw "Execute setup.ps1 ou este script como Administrador." }

$VenvPython = Join-Path $Root ".venv\Scripts\python.exe"
$MonitorScript = Join-Path $Root "monitor.py"
$Dashboard = Join-Path $Root "dashboard\serve.py"
foreach ($path in $VenvPython, $MonitorScript, $Dashboard) { if (-not (Test-Path $path)) { throw "Arquivo obrigatorio ausente: $path" } }

$ErrorActionPreference = "Continue"
& $VenvPython -c "import sys, os; sys.path.insert(0, r'$Root'); os.chdir(r'$Root'); from db import ensure_schema; ensure_schema()" 2>$null
$dbExit = $LASTEXITCODE
$ErrorActionPreference = "Stop"
if ($dbExit -ne 0) { throw "Banco indisponivel ou schema monitor ausente." }

$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit (New-TimeSpan -Seconds 0) -MultipleInstances IgnoreNew
$trigger = New-ScheduledTaskTrigger -AtStartup
$principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
$monitorAction = New-ScheduledTaskAction -Execute $VenvPython -Argument "monitor.py" -WorkingDirectory $Root
$dashboardAction = New-ScheduledTaskAction -Execute $VenvPython -Argument "-m dashboard.serve" -WorkingDirectory $Root
"SystemMonitor", "SystemMonitor-Dashboard" | ForEach-Object {
    $existing = Get-ScheduledTask -TaskName $_ -ErrorAction SilentlyContinue
    if ($existing -and $existing.State -eq "Running") {
        Stop-ScheduledTask -TaskName $_
    }
}
Register-ScheduledTask -TaskName "SystemMonitor" -Action $monitorAction -Trigger $trigger -Settings $settings -Principal $principal -Description "Coleta de telemetria no boot" -Force | Out-Null
Register-ScheduledTask -TaskName "SystemMonitor-Dashboard" -Action $dashboardAction -Trigger $trigger -Settings $settings -Principal $principal -Description "Dashboard local no boot" -Force | Out-Null
"SystemMonitor-User", "SystemMonitor-Dashboard-Flask" | ForEach-Object { Unregister-ScheduledTask -TaskName $_ -Confirm:$false -ErrorAction SilentlyContinue }
Start-ScheduledTask -TaskName "SystemMonitor"
Start-ScheduledTask -TaskName "SystemMonitor-Dashboard"
Write-Host "Tarefas SystemMonitor e SystemMonitor-Dashboard registradas e iniciadas." -ForegroundColor Green
