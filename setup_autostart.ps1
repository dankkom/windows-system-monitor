# setup_autostart.ps1 - Configura inicio automatico ao ligar o PC
# Requer PowerShell como Administrador para SystemMonitor (SYSTEM)
# Uso: powershell -ExecutionPolicy Bypass -File .\setup_autostart.ps1

param([switch]$NoAdmin)

$ErrorActionPreference = "Continue"
$VenvPythonw = "C:\scripts\system-monitor\venv\Scripts\pythonw.exe"
$VenvPython = "C:\scripts\system-monitor\venv\Scripts\python.exe"
$MonitorScript = "C:\scripts\system-monitor\monitor.py"
$DashboardDir = "C:\scripts\system-monitor\dashboard"
$VenvWaitress = "C:\scripts\system-monitor\venv\Scripts\waitress-serve.exe"

Write-Host "== Configurando autostart ==" -ForegroundColor Cyan
Write-Host "Venv Pythonw: $VenvPythonw (existe: $(Test-Path $VenvPythonw))"
$isAdminCheck = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
Write-Host "Modo admin: $isAdminCheck"

# 1. Startup fallback (sem admin)
$StartupBat = "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Startup\system-monitor.bat"
$StartupContent = "@echo off`r`nREM System Monitor - fallback Startup (venv)`r`nstart `"`" `"$VenvPythonw`" `"$MonitorScript`""
Set-Content -Path $StartupBat -Value $StartupContent -Encoding ASCII
Write-Host "OK Startup fallback criado: $StartupBat" -ForegroundColor Green

# 2. Dashboard Task (usuario atual, sem admin) - Flask leve
Write-Host ""
Write-Host "-- Dashboard Task (SystemMonitor-Dashboard) --" -ForegroundColor Yellow
try {
    $argDash = "--port=8501 --host=0.0.0.0 dashboard.app:app"
    $ActionDash = New-ScheduledTaskAction -Execute $VenvWaitress -Argument $argDash -WorkingDirectory "C:\scripts\system-monitor"
    $TriggerDash = New-ScheduledTaskTrigger -AtLogOn -User "$env:USERNAME"
    $SettingsDash = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit 0
    $PrincipalDash = New-ScheduledTaskPrincipal -UserId "$env:USERNAME" -LogonType Interactive -RunLevel Limited
    Unregister-ScheduledTask -TaskName "SystemMonitor-Dashboard" -Confirm:$false -ErrorAction SilentlyContinue
    Unregister-ScheduledTask -TaskName "SystemMonitor-Dashboard-Flask" -Confirm:$false -ErrorAction SilentlyContinue
    Register-ScheduledTask -TaskName "SystemMonitor-Dashboard" -Action $ActionDash -Trigger $TriggerDash -Settings $SettingsDash -Principal $PrincipalDash -Description "Dashboard Flask leve 8501 - inicia ao logon" | Out-Null
    Write-Host "OK Dashboard Task criada (Flask AtLogOn $env:USERNAME)" -ForegroundColor Green
    Start-ScheduledTask -TaskName "SystemMonitor-Dashboard" -ErrorAction SilentlyContinue
    Write-Host "Dashboard iniciado em http://localhost:8501" -ForegroundColor Green
} catch {
    Write-Host "ERRO Dashboard Task: $_" -ForegroundColor Red
}

# 3. Monitor Task (precisa admin)
Write-Host ""
Write-Host "-- Monitor Task (SystemMonitor) --" -ForegroundColor Yellow
$isAdmin = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin -and -not $NoAdmin) {
    Write-Host "Nao e admin - tentando elevar..." -ForegroundColor Yellow
    $arg = "-ExecutionPolicy Bypass -File `"$PSCommandPath`""
    try { Start-Process "powershell.exe" -ArgumentList $arg -Verb RunAs -WindowStyle Normal -Wait; Write-Host "Relancado como admin, verifique tarefas" -ForegroundColor Yellow; exit } catch { Write-Host "Falha ao elevar: $_" -ForegroundColor Red }
}
if ($isAdmin) {
    try {
        $ActionMon = New-ScheduledTaskAction -Execute $VenvPythonw -Argument "`"$MonitorScript`"" -WorkingDirectory "C:\scripts\system-monitor"
        $Trigger1 = New-ScheduledTaskTrigger -AtStartup
        $Trigger2 = New-ScheduledTaskTrigger -AtLogOn
        $SettingsMon = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit 0 -MultipleInstances IgnoreNew
        $PrincipalMon = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
        Unregister-ScheduledTask -TaskName "SystemMonitor" -Confirm:$false -ErrorAction SilentlyContinue
        Unregister-ScheduledTask -TaskName "SystemMonitor-User" -Confirm:$false -ErrorAction SilentlyContinue
        Register-ScheduledTask -TaskName "SystemMonitor" -Action $ActionMon -Trigger $Trigger1,$Trigger2 -Settings $SettingsMon -Principal $PrincipalMon -Description "Coleta maxima Windows -> PostgreSQL (venv 309 sensores)" | Out-Null
        Write-Host "OK Monitor Task criada (SYSTEM AtStartup+AtLogOn Highest)" -ForegroundColor Green
        Start-ScheduledTask -TaskName "SystemMonitor" -ErrorAction SilentlyContinue
        Start-Sleep 3
        Get-ScheduledTaskInfo -TaskName "SystemMonitor" | Format-List LastRunTime,LastTaskResult,NextRunTime | Out-Host
        Get-ScheduledTask -TaskName "SystemMonitor" | Format-List TaskName,State,Triggers | Out-Host
    } catch {
        Write-Host "ERRO Monitor Task: $_" -ForegroundColor Red
        Write-Host $_.ScriptStackTrace -ForegroundColor Red
    }
} else {
    Write-Host "Sem admin: criado Startup + Dashboard. Para coleta completa rode como admin:" -ForegroundColor Yellow
    Write-Host "  powershell -ExecutionPolicy Bypass -File C:\scripts\system-monitor\setup_autostart.ps1" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "== Verificacao ==" -ForegroundColor Cyan
Get-ScheduledTask -TaskName "SystemMonitor*" -ErrorAction SilentlyContinue | Format-Table TaskName,State -AutoSize | Out-Host
Get-Process pythonw,python -ErrorAction SilentlyContinue | Format-Table Id,ProcessName,StartTime -AutoSize | Out-Host
Write-Host "Dashboard: http://localhost:8501 | Logs: C:\scripts\system-monitor\logs\monitor.log" -ForegroundColor Cyan
