Start-ScheduledTask -TaskName "SystemMonitor-User" 2>&1
Start-Sleep 2
Get-ScheduledTaskInfo -TaskName "SystemMonitor-User" | Format-List LastRunTime, LastTaskResult, NextRunTime
Get-Process pythonw -ErrorAction SilentlyContinue | Format-Table Id, ProcessName
Get-Content "$PSScriptRoot\logs\monitor.log" -Tail 20
