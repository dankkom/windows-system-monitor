# System Monitor — Windows → PostgreSQL + Dashboard (Go)

Coleta **contínua e máxima** de telemetria do PC Windows (AMD Ryzen 7 5700X + RTX 4060 Ti) e persiste no PostgreSQL local `system_monitor` com histórico permanente. Binário único Go `monitor-go.exe` + dashboard `http://localhost:8501` (frontend Chart.js estático, ~15 MB binário).

## Métricas (16 coletores, intervalos configuráveis)

| Domínio | Tabela | Intervalo padrão | Fonte (Go) | Dados |
|---|---|---|---|---|
| **CPU** | `monitor.cpu` | 10s | `gopsutil/cpu` | total %, per-core %, freq MHz, cores lógicos/físicos |
| **Memória** | `monitor.memory` | 10s | `gopsutil/mem` | total/available/used %, free, swap, pagefile |
| **Disco uso** | `monitor.disk_usage` | 60s | `gopsutil/disk` | por volume → total/used/free %, fstype, mount |
| **Disco IO** | `monitor.disk_io` | 10s | `gopsutil/disk` | read/write bytes/count/time, busy_time + throughput |
| **Disco físico** | `monitor.physical_disk` | 300s | `Get-PhysicalDisk` (PowerShell) | `FriendlyName`, `MediaType` SSD/HDD/NVMe, `BusType`, `HealthStatus`, `Size` |
| **SMART** | `monitor.disk_smart` | 300s | `smartctl -a -j` | `temperature_c`, `power_on_hours`, `percentage_used` (wear), `available_spare`, `media_errors` |
| **Rede IO** | `monitor.net_io` | 10s | `gopsutil/net` | por iface → bytes/packets sent/recv, err/drop, speed, throughput |
| **Rede addrs** | `monitor.net_addr` | 60s | `gopsutil/net` | iface, family, address, netmask, broadcast |
| **GPU** | `monitor.gpu` | 10s | `nvidia-smi` | util %, mem total/used/free, temp, power, fan, clocks, PCIe |
| **Sensores** | `monitor.sensors` | 15s | `lhm-dump` (LHM via Python helper) | **309 sensores** quando elevado: CPU `Tctl`/`CCD1`, `SuperIO Nuvoton`, GPU, Storage, fans/voltages |
| **Processos** | `monitor.processes` | 30s | `gopsutil/process` | top 50 por CPU+mem → pid/ppid, name, exe, cmdline, user, cpu/mem %, rss/vms, threads, io |
| **Conexões** | `monitor.connections` | 30s | `gopsutil/net` | TCP/UDP → laddr/raddr ip:port, status, pid |
| **Serviços** | `monitor.services` | 60s | `WMI Win32_Service` (`StackExchange/wmi`) | name, display, status, start_type, pid |
| **Sistema** | `monitor.system_info` | 60s | `gopsutil/host` + `cpu` | boot_time, uptime, OS build, arch, ram 96GB, cpu_name |
| **EventLog** | `monitor.eventlog` | 60s | `wevtutil` | System/Application → Error/Warning/Info, event_id, provider, count |
| **Heartbeat** | `monitor.heartbeat` | por coleta | interno | collector, duration_ms, rows, success, error + view `v_last_heartbeat` |

**Dependências externas:** `C:\tools\LibreHardwareMonitor` (v0.9.6) + `PawnIO 2.2.0` (SuperIO); `smartmontools 7.5` (`smartctl.exe`); `Go 1.27+` para build; `.NET SDK 8` apenas para compilar `lhm-dump` (opcional – fallback Python em `.venv`).

## Arquitetura

```
C:\scripts\system-monitor/
├── go/                          # módulo Go
│   ├── cmd/monitor/main.go      # binário único: coletor + dashboard + --once/--dry-run
│   ├── internal/
│   │   ├── collectors/          # 15 coletores (cpu, memory, disk, net, gpu, sensors, processes...)
│   │   ├── config/              # .env + INTERVAL_* + POWER_*
│   │   ├── db/                  # pgx + CopyFrom + spool SQLite
│   │   ├── spool/               # SQLite WAL buffer (2 GB)
│   │   └── dashboard/           # net/http + queries (porta de queries_light.py) + embed static/templates
│   ├── lhm-dump/                # helper LHM (lhm-dump.py fallback + lhm-dump.csproj net472)
│   └── monitor-go.exe           # binário (go build)
├── sql/
│   ├── schema.sql               # 16 tabelas + índices + view (TimescaleDB opcional)
│   └── queries.sql
├── scripts/
│   └── install_tasks_go.ps1     # registra SystemMonitor-Go / SystemMonitor-Go-Dashboard (SYSTEM)
├── .env/.env.example
└── logs/                        # monitor-go.log
```

**Fluxo:** `monitor-go` loop (1s tick, `services`/`processes` via goroutines) → `collectors.*` → `db.Store.InsertBatch` (`pgx CopyFrom`) → `heartbeat` → `logs/monitor-go.log`. Dashboard `net/http` serve `go/internal/dashboard` (mesmas queries SQL, mesmo JSON). Histórico permanente; spool `pending_batches.sqlite3` bufferiza quando PG cai.

## Requisitos

- Windows 11, Go 1.27+, PostgreSQL 18 (`F:\postgresql18`, `5432`), `C:\tools\LibreHardwareMonitor`, `PawnIO` RUNNING, `smartmontools`
- Verificar: `go version; psql --version; Get-Service postgresql-x64-18; sc.exe query PawnIO; smartctl --version`

## Instalação

```powershell
# 1. Build
cd C:\scripts\system-monitor\go
go build -o monitor-go.exe ./cmd/monitor
# helper LHM Python (fallback, usa .venv existente):
# .venv já contém pythonnet; helper em go/lhm-dump/lhm-dump.py
# opcional: compilar helper .NET Framework (requer .NET SDK 8 + targeting pack net472)
dotnet publish go/lhm-dump/lhm-dump.csproj -c Release -o C:\tools\lhm-dump

# 2. Configurar .env (copiar de .env.example se não existe)
# DATABASE_URL=postgresql://postgres:senha@localhost:5432/system_monitor
# INTERVAL_* / POWER_* / DASHBOARD_*

# 3. Aplicar schema
$env:PGPASSWORD="senha"; psql -U postgres -h localhost -d postgres -c "CREATE DATABASE system_monitor OWNER postgres;"
psql -U postgres -h localhost -d system_monitor -f sql\schema.sql

# 4. Testar sem inserir
.\go\monitor-go.exe --dry-run
.\go\monitor-go.exe --once; Get-Content logs\monitor-go.log -Tail 20

# 5. Registrar tarefas de boot (admin)
powershell -ExecutionPolicy Bypass -File .\scripts\install_tasks_go.ps1
# Dashboard: http://127.0.0.1:8501  (health /api/health, ready /api/ready, status /api/status)
```

## Uso diário

```powershell
Get-Content logs\monitor-go.log -Tail 50
psql -U postgres -h localhost -d system_monitor -c "SELECT * FROM monitor.v_last_heartbeat WHERE success=false;"
Invoke-WebRequest http://127.0.0.1:8501/api/health -UseBasicParsing
Invoke-WebRequest http://127.0.0.1:8501/api/status -UseBasicParsing
# parar/iniciar
Stop-ScheduledTask -TaskName SystemMonitor-Go, SystemMonitor-Go-Dashboard
Start-ScheduledTask -TaskName SystemMonitor-Go
```

## Dashboard / Energia

Mesmas agregações do Python: bucket `to_timestamp(floor(epoch/bucket)*bucket)`, `disk_io`/`net` com `lag()` + `dt`/`max_gap`, potência `estimated = (CPU Package + GPU + POWER_AUX_BASELINE_W) / POWER_PSU_EFFICIENCY` com `POWER_GPU_IDLE_W`/`MAX_W` modelo linear, integração trapezoidal Wh/kWh, qualidade `partial`/`estimated_default`/`estimated_calibrated`.

## Troubleshooting

| Sintoma | Causa | Solução |
|---|---|---|
| `sensors` 1 linha `no_sensor` | helper não encontrado/sem elevação | `go/lhm-dump/lhm-dump.py` requer `.venv` + `LibreHardwareMonitorLib.dll`; para 309 sensores rode como SYSTEM (`install_tasks_go.ps1`) |
| `disk_smart` vazio | `smartctl` ausente | `C:\Program Files\smartmontools\bin\smartctl.exe` |
| `eventlog` buffered | wevtutil cp1252 | corrigido via `charmap.Windows1252` + NUL strip |
| `Task LastTaskResult 1` | SYSTEM sem permissão | `install_tasks_go.ps1` como admin |
