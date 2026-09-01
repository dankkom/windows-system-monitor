# Changelog

## 1.0.0 - 2026-08-30
- Monitor contínuo Windows → PostgreSQL `system_monitor` (16 coletores, venv isolado)
- LibreHardwareMonitor 0.9.6 via `pythonnet` (309 sensores elevados, PawnIO 2.2.0)
- smartmontools 7.5 (`smartctl --scan`, `physical_disk` + `disk_smart`)
- Dashboard Streamlit 1.62.0 + Plotly (9 tabs, fragment auto-refresh preserva aba)
- Histórico permanente (`ENABLE_RETENTION=false`), job opcional `jobs/retention.py`
- Task Scheduler elevado (`SYSTEM Highest`) + `SystemMonitor-Dashboard`
- Documentação reproduzível (`setup.ps1`, `.env.example`, `README.md`)
