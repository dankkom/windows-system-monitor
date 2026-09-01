# Mantem a coleta viva enquanto o PostgreSQL termina de iniciar no boot.
$Root = $PSScriptRoot
$Python = Join-Path $Root ".venv\Scripts\python.exe"
$Monitor = Join-Path $Root "monitor.py"
while ($true) {
    & $Python $Monitor
    Start-Sleep -Seconds 30
}
