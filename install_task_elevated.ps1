# Requer PowerShell como Administrador - recria task com privilégio Highest para liberar CPU temps/fans
param([string]$TaskName = "SystemMonitor")
$ErrorActionPreference = "Stop"
$PythonExe = "C:\scripts\system-monitor\venv\Scripts\pythonw.exe"
$ScriptPath = "C:\scripts\system-monitor\monitor.py"
if (-not (Test-Path $PythonExe)) { $PythonExe = "C:\Program Files\Python314\pythonw.exe" }
Write-Host "Criando task elevada $TaskName com $PythonExe"
$Action = New-ScheduledTaskAction -Execute $PythonExe -Argument "`"$ScriptPath`"" -WorkingDirectory "C:\scripts\system-monitor"
$Trigger1 = New-ScheduledTaskTrigger -AtStartup
$Trigger2 = New-ScheduledTaskTrigger -AtLogOn
$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit 0
$Principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
try { Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue } catch {}
try { Unregister-ScheduledTask -TaskName "SystemMonitor-User" -Confirm:$false -ErrorAction SilentlyContinue } catch {}
Register-ScheduledTask -TaskName $TaskName -Action $Action -Trigger $Trigger1,$Trigger2 -Settings $Settings -Principal $Principal -Description "Monitor elevado - temps completos via LibreHardwareMonitor" | Out-Null
Start-ScheduledTask -TaskName $TaskName
Write-Host "Task elevada criada e iniciada. Verifique: Get-ScheduledTaskInfo -TaskName $TaskName"
