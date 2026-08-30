# Instala Task Scheduler para rodar monitor.py 24/7
param(
    [string]$TaskName = "SystemMonitor",
    [string]$PythonExe = "C:\scripts\system-monitor\venv\Scripts\pythonw.exe",
    [string]$ScriptPath = "C:\scripts\system-monitor\monitor.py"
)
$ErrorActionPreference = "Stop"

if (-not (Test-Path $PythonExe)) {
    $PythonExe = (Get-Command python).Source -replace "python.exe","pythonw.exe"
    if (-not (Test-Path $PythonExe)) { $PythonExe = (Get-Command python).Source }
}
Write-Host "Usando Python: $PythonExe"
Write-Host "Script: $ScriptPath"

$Action = New-ScheduledTaskAction -Execute $PythonExe -Argument "`"$ScriptPath`"" -WorkingDirectory "C:\scripts\system-monitor"
$Trigger1 = New-ScheduledTaskTrigger -AtStartup
$Trigger2 = New-ScheduledTaskTrigger -AtLogOn
$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit 0
$Principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType S4U -RunLevel Highest

# Remove existente
Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue

Register-ScheduledTask -TaskName $TaskName -Action $Action -Trigger $Trigger1,$Trigger2 -Settings $Settings -Principal $Principal -Description "Coleta contínua métricas Windows -> PostgreSQL system_monitor" | Out-Null
Write-Host "Task $TaskName registrada."

# Inicia agora
Start-ScheduledTask -TaskName $TaskName
Write-Host "Task iniciada. Verifique:"
Write-Host "  Get-ScheduledTask -TaskName $TaskName | Get-ScheduledTaskInfo"
Write-Host "  Get-Content C:\scripts\system-monitor\logs\monitor.log -Tail 30"
Write-Host "  psql -U postgres -h localhost -d system_monitor -c 'SELECT * FROM monitor.heartbeat ORDER BY ts DESC LIMIT 5;'"
