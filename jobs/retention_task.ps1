# Cria Task diária 02:00 para retenção (só executa se ENABLE_RETENTION=true)
$PythonExe = "C:\scripts\system-monitor\venv\Scripts\python.exe"
$Script = "C:\scripts\system-monitor\jobs\retention.py"
$Action = New-ScheduledTaskAction -Execute $PythonExe -Argument $Script -WorkingDirectory "C:\scripts\system-monitor"
$Trigger = New-ScheduledTaskTrigger -Daily -At 02:00
$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable
$Principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType S4U -RunLevel Limited
Unregister-ScheduledTask -TaskName "SystemMonitor-Retention" -Confirm:$false -ErrorAction SilentlyContinue
Register-ScheduledTask -TaskName "SystemMonitor-Retention" -Action $Action -Trigger $Trigger -Settings $Settings -Principal $Principal -Description "Retenção opcional system_monitor (só roda se ENABLE_RETENTION=true)" | Out-Null
Write-Host "Task SystemMonitor-Retention criada (02:00 diário, inativa se ENABLE_RETENTION=false)"
