param(
    [string]$InstallDir = ""
)
$ErrorActionPreference = "Stop"
$isAdmin = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) { throw "Execute este script como Administrador." }

# Resolve diretorio do app e do executavel
if ($InstallDir -and (Test-Path $InstallDir)) {
    $WorkingDir = (Resolve-Path $InstallDir).Path
    $GoExe = Join-Path $WorkingDir "system-monitor.exe"
} elseif (Test-Path (Join-Path $PSScriptRoot "system-monitor.exe")) {
    $WorkingDir = (Resolve-Path $PSScriptRoot).Path
    $GoExe = Join-Path $WorkingDir "system-monitor.exe"
} elseif (Test-Path (Join-Path $PSScriptRoot "..\system-monitor.exe")) {
    $WorkingDir = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
    $GoExe = Join-Path $WorkingDir "system-monitor.exe"
} else {
    $WorkingDir = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
    $GoExe = Join-Path $WorkingDir "go\system-monitor.exe"
}

if (-not (Test-Path $GoExe)) {
    $goBin = "C:\Program Files\Go\bin\go.exe"
    if (-not (Test-Path $goBin)) { $goBin = (Get-Command go -ErrorAction SilentlyContinue).Source }
    if ($goBin -and (Test-Path $goBin)) {
        Write-Host "Compilando system-monitor.exe..." -ForegroundColor Cyan
        & $goBin build -o $GoExe (Join-Path $WorkingDir "go\cmd\monitor")
    }
    if (-not (Test-Path $GoExe)) {
        throw "Executável system-monitor.exe não encontrado em '$GoExe'. Instale ou compile primeiro."
    }
}

# Valida DATABASE_URL e schema antes de registrar
& $GoExe --init 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) { Write-Warning "Aviso: --init falhou, verifique DATABASE_URL em config.toml" }

$Action = New-ScheduledTaskAction -Execute $GoExe -Argument "--retention" -WorkingDirectory $WorkingDir
$Trigger = New-ScheduledTaskTrigger -Daily -At 02:00
$Settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable
$Principal = New-ScheduledTaskPrincipal -UserId "SYSTEM" -LogonType ServiceAccount -RunLevel Highest
Unregister-ScheduledTask -TaskName "SystemMonitor-Go-Retention" -Confirm:$false -ErrorAction SilentlyContinue
Unregister-ScheduledTask -TaskName "SystemMonitor-Retention" -Confirm:$false -ErrorAction SilentlyContinue
Register-ScheduledTask -TaskName "SystemMonitor-Retention" -Action $Action -Trigger $Trigger -Settings $Settings -Principal $Principal -Description "Retencao opcional system_monitor (so roda se ENABLE_RETENTION=true)" | Out-Null
Write-Host "Task SystemMonitor-Retention criada (02:00 diario, inativa se ENABLE_RETENTION=false)" -ForegroundColor Green
Write-Host "Teste: system-monitor --retention-dry-run  ou  system-monitor --retention"
