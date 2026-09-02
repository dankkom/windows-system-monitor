# System Monitor — Windows → PostgreSQL + Dashboard (Go)

Coleta **contínua e máxima** de telemetria do PC Windows e persiste no PostgreSQL (`system_monitor`, local ou central) com histórico permanente. Binário único Go `monitor-go.exe` (~15 MB, `CGO_ENABLED=0`, sem runtime) + dashboard `http://localhost:8501` (Chart.js estático). Distribuível para qualquer máquina Windows via instalador Inno Setup ou `installer/install.ps1` — requer apenas PostgreSQL externo (`DATABASE_URL`) e, opcionalmente, `smartctl`/`PawnIO` para métricas completas.

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
| **Sensores** | `monitor.sensors` | 15s | `lhm-dump.exe` (LHM .NET Framework 4.7.2) | **309 sensores** quando elevado: CPU `Tctl`/`CCD1`, `SuperIO Nuvoton`, GPU, Storage, fans/voltages |
| **Processos** | `monitor.processes` | 30s | `gopsutil/process` | top 50 por CPU+mem → pid/ppid, name, exe, cmdline, user, cpu/mem %, rss/vms, threads, io |
| **Conexões** | `monitor.connections` | 30s | `gopsutil/net` | TCP/UDP → laddr/raddr ip:port, status, pid |
| **Serviços** | `monitor.services` | 60s | `WMI Win32_Service` (`StackExchange/wmi`) | name, display, status, start_type, pid |
| **Sistema** | `monitor.system_info` | 60s | `gopsutil/host` + `cpu` | boot_time, uptime, OS build, arch, ram 96GB, cpu_name |
| **EventLog** | `monitor.eventlog` | 60s | `wevtutil` | System/Application → Error/Warning/Info, event_id, provider, count |
| **Heartbeat** | `monitor.heartbeat` | por coleta | interno | collector, duration_ms, rows, success, error + view `v_last_heartbeat` |

**Dependências externas (opcionais, degradam graciosamente):** `PawnIO 2.2.0` + `LibreHardwareMonitor` (309 sensores via `lhm-dump.exe`, senão 143); `smartmontools 7.5` (`smartctl.exe` para SMART); `nvidia-smi` (GPU); `.NET Framework 4.7.2` runtime para `lhm-dump.exe`. **Obrigatório:** `PostgreSQL 14+` externo (local ou central) acessível via `DATABASE_URL`. Build requer `Go 1.27+` e `.NET SDK 8` apenas para compilar `lhm-dump`.

## Arquitetura

```
C:\scripts\system-monitor/
├── go/                          # módulo Go (CGO_ENABLED=0)
│   ├── cmd/monitor/main.go      # binário único: coletor + dashboard + --once/--dry-run/--init/--retention
│   ├── internal/
│   │   ├── collectors/          # 15 coletores (cpu, memory, disk, net, gpu, sensors, processes...)
│   │   ├── config/              # .env + INTERVAL_* + POWER_* + RETENTION_*
│   │   ├── db/                  # pgx CopyFrom + spool SQLite + retention + schema embed
│   │   ├── spool/               # SQLite WAL buffer (2 GB)
│   │   └── dashboard/           # net/http + queries + embed static/templates
│   ├── lhm-dump/                # helper LHM (lhm-dump.csproj net472, CopyLocalLockFileAssemblies)
│   └── monitor-go.exe           # binário (go build, ~15 MB)
├── sql/schema.sql               # 16 tabelas + índices + view (embed em go/internal/db/schema.sql)
├── installer/
│   ├── system-monitor.iss       # Inno Setup (gera setup.exe)
│   └── install.ps1              # one-click: build + copia para Program Files + --init + tasks
├── scripts/
│   ├── install_tasks_go.ps1     # SystemMonitor-Go / Go-Dashboard (SYSTEM)
│   └── install_retention_go.ps1 # SystemMonitor-Go-Retention 02:00 (SYSTEM, só se ENABLE_RETENTION=true)
├── .env/.env.example
└── logs/                        # monitor-go.log + pending_batches.sqlite3
```

**Fluxo:** `monitor-go` loop (1s tick, `services`/`processes` via goroutines) → `collectors.*` → `db.Store.InsertBatch` (`pgx CopyFrom`) → `heartbeat` → `logs/monitor-go.log`. Dashboard `net/http` serve `go/internal/dashboard` (mesmas queries SQL, mesmo JSON). Histórico permanente; spool `pending_batches.sqlite3` bufferiza quando PG cai.

## Requisitos

- Windows 10/11, PostgreSQL externo (local ou central, 14+) acessível via `DATABASE_URL`
- Opcional para métricas completas: `PawnIO` (309 sensores), `smartmontools` (`smartctl`), `nvidia-smi` (GPU NVIDIA)
- Build (só para gerar instalador): `Go 1.27+`, `.NET SDK 8`
- Verificar: `go version; dotnet --version; psql --version; sc.exe query PawnIO; smartctl --version`

## Distribuição / Instalação em máquina nova

### Opção A — one-click (recomendado para teste)

```powershell
# Requer admin. Builda, copia para Program Files, cria .env, aplica schema e registra tasks.
powershell -ExecutionPolicy Bypass -File installer/install.ps1
# Edite C:\Program Files\system-monitor\.env -> DATABASE_URL=postgresql://user:senha@HOST:5432/system_monitor
# Depois: & "C:\Program Files\system-monitor\monitor-go.exe" --init
```

### Opção B — Inno Setup (distribuível)

```powershell
# 1. Build binários
go build -o go/monitor-go.exe ./go/cmd/monitor
dotnet publish go/lhm-dump/lhm-dump.csproj -c Release -o build/lhm-dump
# 2. Gerar instalador (requer Inno Setup 6)
iscc installer/system-monitor.iss
# Saída: dist/system-monitor-1.0.0-setup.exe  (executar como admin em qualquer host)
```

Instalador faz: copia para `{pf}\system-monitor`, cria `.env` de `.env.example` se ausente, roda `monitor-go --init` (cria DB + aplica schema), registra `SystemMonitor-Go` / `SystemMonitor-Go-Dashboard` / `SystemMonitor-Go-Retention` (SYSTEM, boot 02:00).

### Opção C — manual (dev)

```powershell
# 1. Build
go build -o go/monitor-go.exe ./go/cmd/monitor
dotnet publish go/lhm-dump/lhm-dump.csproj -c Release -o C:\tools\lhm-dump

# 2. .env
copy .env.example .env
# Edite DATABASE_URL (local ou central): postgresql://postgres:senha@HOST:5432/system_monitor

# 3. Banco + schema (automatico, substitui psql manual)
.\go\monitor-go.exe --init
# Alternativa manual: psql -U postgres -h HOST -d postgres -c "CREATE DATABASE system_monitor"
#                   psql -U postgres -h HOST -d system_monitor -f sql/schema.sql

# 4. Testar
.\go\monitor-go.exe --dry-run
.\go\monitor-go.exe --once; Get-Content logs\monitor-go.log -Tail 20
.\go\monitor-go.exe --retention-dry-run   # simula limpeza
# Se ENABLE_RETENTION=true: .\go\monitor-go.exe --retention

# 5. Tasks (admin)
powershell -ExecutionPolicy Bypass -File .\scripts\install_tasks_go.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\install_retention_go.ps1  # opcional
# Dashboard: http://127.0.0.1:8501  (/api/health, /api/ready, /api/status)
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

Agregações: bucket `to_timestamp(floor(epoch/bucket)*bucket)`, `disk_io`/`net` com `lag()` + `dt`/`max_gap`, potência `estimated = (CPU Package + GPU + POWER_AUX_BASELINE_W) / POWER_PSU_EFFICIENCY` com `POWER_GPU_IDLE_W`/`MAX_W` modelo linear, integração trapezoidal Wh/kWh, qualidade `partial`/`estimated_default`/`estimated_calibrated`.

## Retenção

Desabilitada por padrão (histórico permanente). Para ativar, em `.env`:

```
ENABLE_RETENTION=true
RETENTION_PROCESSES=30 days
RETENTION_CONNECTIONS=7 days
# ... veja .env.example
```

Registre a task diária: `powershell -ExecutionPolicy Bypass -File scripts/install_retention_go.ps1` (02:00 SYSTEM).
Teste: `monitor-go --retention-dry-run` (conta) / `monitor-go --retention` (delete paginado, `RETENTION_BATCH_LIMIT`/`RETENTION_BATCH_SLEEP`).

## Troubleshooting

| Sintoma | Causa | Solução |
|---|---|---|
| `sensors` 1 linha `no_sensor` | helper não encontrado/sem elevação | `lhm-dump.exe` deve estar ao lado de `monitor-go.exe` ou em `C:\tools\lhm-dump`; recompile `dotnet publish` e rode como SYSTEM com PawnIO |
| `disk_smart` vazio | `smartctl` ausente | `C:\Program Files\smartmontools\bin\smartctl.exe` ou `smartctl` no PATH |
| `init` falha | PG offline / URL errada | Verifique `DATABASE_URL` e `Get-Service postgresql*`; `monitor-go --init` cria DB + schema |
| `retention` não apaga | `ENABLE_RETENTION=false` | Ative no `.env` e rode `monitor-go --retention` |
| `Task LastTaskResult 1` | SYSTEM sem permissão | `install_tasks_go.ps1` como admin |
