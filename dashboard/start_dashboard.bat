@echo off
"C:\scripts\system-monitor\venv\Scripts\python.exe" -m streamlit run "C:\scripts\system-monitor\dashboard\app.py" --server.port 8501 --server.headless true
pause
