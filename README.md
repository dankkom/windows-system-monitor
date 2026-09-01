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
C:\code\windows-system-monitor/
├── .venv/                    # ambiente uv isolado
├── collectors/               # 11 módulos (cpu, memory, disk, disk_smart, network, gpu, sensors, processes, connections, services, system)
├── dashboard/                # Flask + Chart.js leve (~50 MB, waitress)
│   ├── app.py                # Flask rotas /api/* + / (Chart.js, polling preserva aba)
│   ├── queries_light.py      # SQL parametrizado sem pandas
│   ├── serve.py              # Entrypoint waitress
│   ├── templates/index.html  # tabs CSS + Chart.js 4.4 CDN
│   └── static/app.js / style.css
├── docs/
│   ├── hardware_sensors.md   # Notas LibreHardwareMonitor/PawnIO
│   └── CHANGELOG.md
├── jobs/
│   ├── retention.py          # DELETE opcional (ENABLE_RETENTION=false por padrão → histórico permanente)
│   └── retention_task.ps1
├── scripts/
│   ├── install_tasks.ps1     # Registra tarefas no Task Scheduler (SYSTEM Highest)
│   ├── start_monitor.ps1     # Inicia as tarefas agendadas
│   └── stop_monitor.ps1      # Para as tarefas agendadas
├── sql/
│   ├── schema.sql            # 16 tabelas + índices + view (inclui physical_disk e disk_smart)
│   └── queries.sql           # Queries úteis de referência
├── tests/                    # Pytest (dashboard, sensors, spool)
├── config.py / db.py / monitor.py / spool.py  # loop, buffer SQLite e heartbeat
├── .env.example → .env       # DATABASE_URL, INTERVAL_*, ENABLE_RETENTION
├── pyproject.toml + uv.lock  # dependências travadas; requirements.txt como fallback pip
├── setup.ps1                 # Bootstrap idempotente completo
└── logs/                     # RotatingFileHandler 10 MB
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

## Instalacao reproduzivel (com uv)

```powershell
# 1. Clone / copie pasta
cd C:\code\windows-system-monitor

# 2. Instalacao unica (pede elevacao, cria .venv/DB/schema e inicia as tarefas)
powershell -ExecutionPolicy Bypass -File .\setup.ps1
# ou manual:
uv sync --frozen
copy .env.example .env  # ajuste DATABASE_URL
# DB (requer PGPASSWORD ou pgpass)
$env:PGPASSWORD="sua_senha"; psql -U postgres -h localhost -d postgres -c "CREATE DATABASE system_monitor OWNER postgres;"
psql -U postgres -h localhost -d system_monitor -f sql\schema.sql

# 3. Dependências externas (portáteis - já inclusas em C:\tools se usou setup.ps1)
# LibreHardwareMonitor 0.9.6: https://github.com/LibreHardwareMonitor/LibreHardwareMonitor/releases/download/v0.9.6/LibreHardwareMonitor.zip -> C:\tools\LibreHardwareMonitor
# PawnIO 2.2.0: C:\tools\PawnIO_setup.exe /S (requer admin)
# smartmontools 7.5: winget install smartmontools.smartmontools --silent

# 4. Teste sem inserir
.\.venv\Scripts\python.exe monitor.py --dry-run
.\.venv\Scripts\python.exe monitor.py --once
Get-Content logs\monitor.log -Tail 20
psql -U postgres -h localhost -d system_monitor -c "SELECT count(*) FROM monitor.sensors; SELECT device,temperature_c FROM monitor.disk_smart ORDER BY ts DESC LIMIT 4;"

# 5. Apos o setup, as tarefas SystemMonitor e SystemMonitor-Dashboard
# iniciam no boot, mesmo sem logon. Acesse http://127.0.0.1:8501.
# Para reparar apenas as tarefas, execute como Administrador:
powershell -ExecutionPolicy Bypass -File .\scripts\install_tasks.ps1

# 6. Retenção opcional (desabilitada por padrão)
# .env ENABLE_RETENTION=true e:
powershell -ExecutionPolicy Bypass -File .\jobs\retention_task.ps1
.\.venv\Scripts\python.exe jobs\retention.py --dry
```

`setup.ps1` e idempotente: prepara dependencias, banco/schema, testa a coleta,
registra as tarefas de boot e confirma o healthcheck do dashboard local.

## Desenvolvimento

`uv` e o fluxo principal e cria `.venv` com versoes travadas em `uv.lock`:

```powershell
uv sync --group dev --frozen
.\.venv\Scripts\python.exe -m pytest -p no:cacheprovider
.\.venv\Scripts\python.exe -m ruff check .
```

Se PostgreSQL ficar indisponivel, a coleta grava lotes em `logs\pending_batches.sqlite3`
e tenta reenvia-los quando a conexao voltar. O buffer e limitado a 2 GB; ao atingir o
limite, as amostras pendentes mais antigas sao descartadas.

## Uso diário

```powershell
# logs
Get-Content logs\monitor.log -Tail 50
Get-Content logs\monitor_error.log -Tail 20
# DB
psql -U postgres -h localhost -d system_monitor -c "SELECT * FROM monitor.v_last_heartbeat WHERE success=false;"
psql -U postgres -h localhost -d system_monitor -c "SELECT device,model,temperature_c,power_on_hours FROM monitor.disk_smart ORDER BY ts DESC LIMIT 4;"
# dashboard health
Invoke-WebRequest http://127.0.0.1:8501/api/health -UseBasicParsing
# readiness PostgreSQL/schema e estado do buffer
Invoke-WebRequest http://127.0.0.1:8501/api/ready -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:8501/api/status -UseBasicParsing
# parar
.\scripts\stop_monitor.ps1  # ou: Stop-ScheduledTask -TaskName SystemMonitor, SystemMonitor-Dashboard
# iniciar
.\scripts\start_monitor.ps1  # ou: Start-ScheduledTask -TaskName SystemMonitor
```

## Dashboard técnico

O dashboard agrega as séries no PostgreSQL para manter cada gráfico limitado a
aproximadamente 600 pontos. A aba **Disco** mostra capacidade histórica,
throughput, IOPS, latência, ocupação e bytes acumulados na janela. A aba **Rede**
mostra tráfego acumulado, bit/s, pacotes/s, erros, descartes e utilização do link.

A aba **Energia** mantém medição e estimativa separadas:

- `CPU Package` é a fonte canônica; sensores de cores/memória/platform não são
  somados para evitar dupla contagem;
- potência nativa da GPU é usada quando disponível; `POWER_GPU_IDLE_W` e
  `POWER_GPU_MAX_W` habilitam um modelo linear opcional quando o driver não
  expõe watts;
- estimativa na tomada = `(CPU + GPU + POWER_AUX_BASELINE_W) /
  POWER_PSU_EFFICIENCY`;
- Wh/kWh usam integração trapezoidal e não preenchem lacunas longas.

Os padrões (`30 W`, eficiência `0.90`) são apenas referência. Calibre-os com um
medidor de tomada para comparações absolutas. Cada visual inclui definição,
fonte, fórmula e limitações diretamente na interface.

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
| `Task LastTaskResult 1` | `SYSTEM` sem permissão pasta | Use `scripts\install_tasks.ps1` como admin |
| Dashboard volta à Overview | `meta refresh` Streamlit | Trocado por Flask `fetch` + `localStorage` (preserva aba) |

## Projeto

- **Estrutura** ver `tree` acima. `.venv` isolado (~50 MB Flask vs 211 MB Streamlit), `.gitignore` exclui `.venv/`, `logs/*.log`, `.env`.
- **Versionado** com Git (`git log --oneline`). Reprodutível via `setup.ps1` + `.env.example` + `pyproject.toml` + `sql/schema.sql`.
- **Licença** MPL-2.0 (LibreHardwareMonitor) + MIT para código próprio.
