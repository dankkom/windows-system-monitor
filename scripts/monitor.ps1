# Gerencia as tarefas agendadas do monitor.
# Uso: .\scripts\monitor.ps1 [start|stop|status]
param(
    [ValidateSet("start","stop","status")]
    [string]$Action = "status"
)

$Tasks = "SystemMonitor", "SystemMonitor-Dashboard"

switch ($Action) {
    "start" {
        $Tasks | ForEach-Object { Start-ScheduledTask -TaskName $_ }
        Start-Sleep 2
    }
    "stop" {
        $Tasks | ForEach-Object { Stop-ScheduledTask -TaskName $_ -ErrorAction SilentlyContinue }
    }
}

$Tasks | ForEach-Object { Get-ScheduledTaskInfo -TaskName $_ -ErrorAction SilentlyContinue } |
    Format-Table TaskName, LastRunTime, LastTaskResult -AutoSize
