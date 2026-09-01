# System Monitor — Windows → PostgreSQL + Dashboard

Coleta **contínua e máxima** de telemetria do PC Windows (AMD Ryzen 7 5700X + RTX 4060 Ti) e persiste no PostgreSQL local `system_monitor` com histórico permanente. Dashboard Flask leve em `http://localhost:8501` (~50 MB vs 211 MB Streamlit).

## Métricas (16 coletores, intervalos configuráveis)

| Domínio | Tabela | Intervalo padrão | Fonte | Dados |
|---|---|---|---|---|
| **CPU** | `monitor.cpu` | 10s | `psutil` | total %, per-core %, freq MHz, cores lógicos/físicos, ctx_switches, interrupts |
| **Memória** | `monitor.memory` | 10s | `psutil` | total/available/used %, free, swap, pagefile, buffers/cached |
| **Disco uso** | `monitor.disk_usage` | 60s | `psutil` | por volume `C,E,F,G,J` → total/used/free %, fstype, mount |
| **Disco IO** | `monitor.disk_io` | 10s | `psutil` | read/write bytes/count/time, busy_time por `PhysicalDrive0-3` + throughput MB/s |
| **Disco físico** | `monitor.physical_disk` | 300s | `Get-PhysicalDisk` | `FriendlyName`, `MediaType` SSD/HDD/NVMe, `BusType` SATA/NVMe, `HealthStatus`, `Size`, `Serial` |
| **SMART** | `monitor.disk_smart` | 300s | `smartctl -a -j` | `temperature_c` (HDD 40-42C, NVMe 49C), `power_on_hours` (6678-20038h), `power_cycle_count`, `percentage_used` (NVMe wear 2%), `available_spare`, `media_errors`, `reallocated/pending`, `host_reads/writes`, `data_units`, `total_lbas`, `smart_passed` + `raw JSON` |
| **Rede IO** | `monitor.net_io` | 10s | `psutil` | por iface (Wi-Fi, ZeroTier, WSL) → bytes/packets sent/recv, err/drop, speed, is_up, throughput KB/s |
| **Rede addrs** | `monitor.net_addr` | 60s | `psutil` | iface, family, address, netmask, broadcast |
| **GPU** | `monitor.gpu` | 10s | `nvidia-smi` + `pynvml` | util %, mem total/used/free, temp 48-49C / hot spot 57C, power 10W/160W, fan, clocks 210/405 MHz, PCIe |
| **Sensores** | `monitor.sensors` | 15s | `LibreHardwareMonitorLib.dll` via `pythonnet` (PawnIO) | **309 sensores** quando elevado: CPU `Tctl 62C`/`CCD1 50C`, `SuperIO Nuvoton NCT6687D: CPU 62C/System 48.5/VRM 49`, GPU, `Storage: Life 98%`, `Data Read 15TB`, `Power On Hours`, fan RPM, voltage, load, clock, power |
| **Processos** | `monitor.processes` | 30s | `psutil` | top 50 por CPU+mem → pid/ppid, name, exe, cmdline, user, cpu/mem %, rss/vms, threads, handles, io |
| **Conexões** | `monitor.connections` | 30s | `psutil` | TCP/UDP → laddr/raddr ip:port, status, pid, fd |
| **Serviços** | `monitor.services` | 60s | `WMI Win32_Service` | name, display, status, start_type, pid |
| **Sistema** | `monitor.system_info` | 60s | `platform` + `WMI` | boot_time, uptime, OS build, arch, manufacturer/model, ram 96GB, cpu_name, users, battery |
| **EventLog** | `monitor.eventlog` | 60s | `win32evtlog` | System/Application → Error/Warning/Info, event_id, provider, count |
| **Heartbeat** | `monitor.heartbeat` | por coleta | interno | collector, duration_ms, rows, success, error + view `v_last_heartbeat` |

**Dependências externas:** `C:\tools\LibreHardwareMonitor` (v0.9.6 portable, 6.3 MB) + `C:\tools\LibreHardwareMonitorLib.dll` via `pythonnet`; `PawnIO 2.2.0` driver (para `SuperIO` temps sem elevação total); `smartmontools 7.5` (`C:\Program Files\smartmontools\bin\smartctl.exe`).

## Arquitetura

```
C:\scripts\system-monitor/
├── venv/                     # Python 3.14.5 isolado (não polui sistema)
├── collectors/               # 10 módulos (cpu, memory, disk, disk_smart, network, gpu, sensors, processes, connections, services, system)
├── dashboard/                # Flask + Chart.js leve (~50 MB, waitress)
│   ├── app.py                # Flask 9 rotas /api/* + / (Chart.js, polling preserva aba)
│   ├── queries_light.py      # SQL parametrizado sem pandas
│   ├── templates/index.html  # tabs CSS + Chart.js 4.4 CDN
│   └── static/app.js/style.css
├── sql/
│   ├── schema.sql            # 14 tabelas + índices + view
│   ├── disk_extra.sql        # physical_disk + disk_smart
│   └── queries.sql           # 10 queries úteis
├── jobs/
│   ├── retention.py          # DELETE opcional (ENABLE_RETENTION=false por padrão → histórico permanente)
│   └── retention_task.ps1
├── config.py / db.py / monitor.py # loop 1s, batch inserts, heartbeat, failed_batches.jsonl
├── .env.example → .env       # DATABASE_URL, INTERVAL_*, ENABLE_RETENTION
├── requirements.txt + dashboard/requirements_light.txt # Flask/waitress (sem pandas/plotly)
├── install_task.ps1 / install_task_elevated.ps1 # Task Scheduler (S4U / SYSTEM Highest)
├── setup.ps1                 # bootstrap reproduzível
└── logs/ (RotatingFileHandler 10MB)
```

**Fluxo:** `monitor.py` loop → `collectors/*.collect(hostname)` → `db.insert_batch` (`psycopg3 executemany`) → `heartbeat` → `logs/monitor.log`. `dashboard` lê `psycopg` direto. Histórico nunca apagado (`DELETE` só se `ENABLE_RETENTION=true`).

## Requisitos

- Windows 11 Pro 10.0.26200, Python 3.14.5, PostgreSQL 18.4 em `F:\postgresql18` (`port 5432`), `pgpass.conf` com `localhost:5432:system_monitor:postgres:***`
- `C:\tools\LibreHardwareMonitor` extraído, `PawnIO` serviço `RUNNING`, `smartmontools` instalado

Check:
```powershell
python --version; psql --version
Get-Service postgresql-x64-18 | Format-Table Status
sc.exe query PawnIO
smartctl --version
Test-Path C:\tools\LibreHardwareMonitor\LibreHardwareMonitorLib.dll
```

## Instalação reproduzível (com venv)

```powershell
# 1. Clone / copie pasta
cd C:\scripts\system-monitor

# 2. Bootstrap (cria venv, instala deps, cria DB + schema)
powershell -ExecutionPolicy Bypass -File .\setup.ps1
# ou manual:
python -m venv venv
.\venv\Scripts\Activate.ps1
pip install -r requirements.txt
pip install -r dashboard\requirements_light.txt
copy .env.example .env  # ajuste DATABASE_URL
# DB (requer PGPASSWORD ou pgpass)
$env:PGPASSWORD="sua_senha"; psql -U postgres -h localhost -d postgres -c "CREATE DATABASE system_monitor OWNER postgres;"
psql -U postgres -h localhost -d system_monitor -f sql\schema.sql
psql -U postgres -h localhost -d system_monitor -f sql\disk_extra.sql

# 3. Dependências externas (portáteis - já inclusas em C:\tools se usou setup.ps1)
# LibreHardwareMonitor 0.9.6: https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/releases/download/v0.9.6/LibreHardwareMonitor.zip -> C:\tools\LibreHardwareMonitor
# PawnIO 2.2.0: C:\tools\PawnIO_setup.exe /S (requer admin)
# smartmontools 7.5: winget install smartmontools.smartmontools --silent

# 4. Teste sem inserir
.\venv\Scripts\python.exe monitor.py --dry-run
.\venv\Scripts\python.exe monitor.py --once
Get-Content logs\monitor.log -Tail 20
psql -U postgres -h localhost -d system_monitor -c "SELECT count(*) FROM monitor.sensors; SELECT device,temperature_c FROM monitor.disk_smart ORDER BY ts DESC LIMIT 4;"

# 5. Execução contínua (requer admin para temps completos 309 sensores)
# Opção A: manual elevado (recomendado para 309 sensores)
Start-Process .\venv\Scripts\pythonw.exe -ArgumentList "C:\scripts\system-monitor\monitor.py" -Verb RunAs
# Opção B: Task Scheduler persistente
powershell -ExecutionPolicy Bypass -File .\install_task_elevated.ps1  # SYSTEM Highest, AtStartup+AtLogOn (requer admin)
# Opção C: usuário (sem admin, 127 sensores)
powershell -ExecutionPolicy Bypass -File .\install_task.ps1

# 6. Dashboard Flask leve (~50 MB)
.\venv\Scripts\waitress-serve.exe --port=8501 --host=0.0.0.0 dashboard.app:app
# ou Task:
powershell -ExecutionPolicy Bypass -File .\setup_autostart.ps1  # recria SystemMonitor-Dashboard
# Acesse http://localhost:8501 (Chart.js, polling preserva aba, sem pandas/plotly)

# 7. Retenção opcional (desabilitada por padrão)
# .env ENABLE_RETENTION=true e:
powershell -ExecutionPolicy Bypass -File .\jobs\retention_task.ps1
.\venv\Scripts\python.exe jobs\retention.py --dry
```

`setup.ps1` faz tudo acima idempotente (testa `venv`, `pip`, `psql`, `schema`, `LibreHardwareMonitor.zip`, `smartctl`).

## Uso diário

```powershell
# logs
Get-Content logs\monitor.log -Tail 50
Get-Content logs\monitor_error.log -Tail 20
# DB
psql -U postgres -h localhost -d system_monitor -c "SELECT * FROM monitor.v_last_heartbeat WHERE success=false;"
psql -U postgres -h localhost -d system_monitor -c "SELECT device,model,temperature_c,power_on_hours FROM monitor.disk_smart ORDER BY ts DESC LIMIT 4;"
# dashboard health
Invoke-WebRequest http://localhost:8501/api/health -UseBasicParsing
# parar
Get-Process pythonw | Stop-Process -Force  # ou .\stop_monitor.ps1 (requer Bypass)
# iniciar
.\start_monitor.ps1  # ou Start-ScheduledTask -TaskName SystemMonitor
```

## Retenção e tamanho

- Padrão `ENABLE_RETENTION=false` → histórico permanente. `jobs/retention.py` só apaga se `true`.
- Atual `43 MB` em `1h13` → projeção `~800 MB/dia` (sensors 309/15s + processes 50/30s + connections 300/30s). Ajuste `INTERVAL_*` em `.env` ou ative retenção `30/7/90 dias`.
- Particionamento futuro: `sql/migrate_timescale.sql` (TimescaleDB hypertable) pronto mas não aplicado.

## Troubleshooting

| Sintoma | Causa | Solução |
|---|---|---|
| `sensors` só 1 linha `no_sensor` | Não elevado + sem LibreHardwareMonitor | `C:\tools\PawnIO_setup.exe /S` + `RunAs` → 309 sensores |
| `disk_smart` vazio | `smartctl` não no PATH | `C:\Program Files\smartmontools\bin\smartctl.exe` hard-coded, verifique `smartctl --scan` |
| `psql` timeout | `pgpass.conf` sem `system_monitor` | `localhost:5432:system_monitor:postgres:senha` + `127.0.0.1:5432:*:postgres:senha` |
| `Task LastTaskResult 1` | `SYSTEM` sem permissão pasta | Use `install_task_elevated.ps1` como admin, ou `Start-Process -Verb RunAs` manual |
| Dashboard volta à Overview | `meta refresh` Streamlit | Trocado por Flask `fetch` + `localStorage` (preserva aba) |

## Projeto

- **Estrutura** ver `tree` acima. `venv` isolado (~50 MB Flask vs 211 MB Streamlit), `.gitignore` exclui `venv/`, `logs/*.log`, `.env`.
- **Versionado** com Git (`git log --oneline`). Reprodutível via `setup.ps1` + `.env.example` + `requirements*.txt` + `sql/*.sql`.
- **Licença** MPL-2.0 (LibreHardwareMonitor) + MIT para código próprio.
