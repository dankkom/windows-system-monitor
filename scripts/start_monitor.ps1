"SystemMonitor", "SystemMonitor-Dashboard" | ForEach-Object { Start-ScheduledTask -TaskName $_ }
Start-Sleep 2
Get-ScheduledTaskInfo -TaskName "SystemMonitor", "SystemMonitor-Dashboard" | Format-List TaskName, LastRunTime, LastTaskResult
