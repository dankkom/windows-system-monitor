# Task para iniciar dashboard automaticamente no logon
param([int]$Port=8501)
$PythonExe = "C:\scripts\system-monitor\venv\Scripts\python.exe"
$Action = New-ScheduledTaskAction -Execute $PythonExe -Argument " -m streamlit run C:\scripts\system-monitor\dashboard\app.py --server.port $Port --server.headless true" -WorkingDirectory "C:\scripts\system-monitor\dashboard"
$Trigger = New-ScheduledTaskTrigger -AtLogOn -User "$env:USERNAME"
$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
$Principal = New-ScheduledTaskPrincipal -UserId "$env:USERNAME" -LogonType Interactive -RunLevel Limited
Unregister-ScheduledTask -TaskName "SystemMonitor-Dashboard" -Confirm:$false -ErrorAction SilentlyContinue
Register-ScheduledTask -TaskName "SystemMonitor-Dashboard" -Action $Action -Trigger $Trigger -Settings $Settings -Principal $Principal -Description "Dashboard Streamlit system_monitor porta $Port" | Out-Null
Write-Host "Task SystemMonitor-Dashboard criada. Inicie com: Start-ScheduledTask -TaskName SystemMonitor-Dashboard"
Write-Host "Acesse: http://localhost:$Port"
