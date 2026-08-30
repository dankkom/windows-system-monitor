param([int]$Port=8501)
$VenvPython = "C:\scripts\system-monitor\venv\Scripts\python.exe"
if (-not (Test-Path $VenvPython)) { $VenvPython = "python" }
Write-Host "Iniciando dashboard Streamlit na porta $Port ..."
& $VenvPython -m streamlit run "C:\scripts\system-monitor\dashboard\app.py" --server.port $Port --server.headless true
