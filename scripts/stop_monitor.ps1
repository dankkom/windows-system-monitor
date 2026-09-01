"SystemMonitor", "SystemMonitor-Dashboard" | ForEach-Object { Stop-ScheduledTask -TaskName $_ -ErrorAction SilentlyContinue }
