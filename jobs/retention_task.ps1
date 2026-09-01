# Cria Task diária 02:00 para retenção (só executa se ENABLE_RETENTION=true)
$Root = Split-Path $PSScriptRoot -Parent
$PythonExe = Join-Path $Root "venv\Scripts\python.exe"
$Script = Join-Path $Root "jobs\retention.py"
if (-not (Test-Path $PythonExe)) { throw "Python do ambiente virtual não encontrado: $PythonExe. Execute setup.ps1 primeiro." }
& $PythonExe -c "from db import ensure_schema; ensure_schema()" 2>$null
if ($LASTEXITCODE -ne 0) { throw "Banco indisponível ou schema monitor ausente. Execute setup.ps1 e confira DATABASE_URL em .env." }
$Action = New-ScheduledTaskAction -Execute $PythonExe -Argument $Script -WorkingDirectory $Root
$Trigger = New-ScheduledTaskTrigger -Daily -At 02:00
$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable
$Principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType S4U -RunLevel Limited
Unregister-ScheduledTask -TaskName "SystemMonitor-Retention" -Confirm:$false -ErrorAction SilentlyContinue
Register-ScheduledTask -TaskName "SystemMonitor-Retention" -Action $Action -Trigger $Trigger -Settings $Settings -Principal $Principal -Description "Retenção opcional system_monitor (só roda se ENABLE_RETENTION=true)" | Out-Null
Write-Host "Task SystemMonitor-Retention criada (02:00 diário, inativa se ENABLE_RETENTION=false)"
