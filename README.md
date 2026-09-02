# System Monitor — Instalação em 5 minutos (Windows)

Coleta contínua de telemetria do seu PC Windows e envia para um **PostgreSQL** (local ou central) + dashboard em `http://localhost:8501`. Um único `monitor-go.exe` (~15 MB), sem Python, sem Docker. Ideal para deixar rodando no boot e esquecer.

> **Você só quer instalar?** Vá direto para [Instalação rápida](#instalação-rápida). O resto é detalhe.

---

## Instalação rápida

### 1. Baixe o instalador
Em [**Releases**](https://github.com/dankkom/windows-system-monitor/releases) baixe `system-monitor-*-setup.exe` (ou use o `*-windows-amd64.zip` portátil).

> Alternativa sem instalador: `powershell -ExecutionPolicy Bypass -File installer/install.ps1` direto do repositório clonado (faz build local e **pergunta** os mesmos dados de banco interativamente — também gera `config.toml` e pode instalar PG/opcionais).

### 2. Execute como Administrador
Duplo-clique no `setup.exe` **como Administrador** → assistente em português:

1. **Banco de dados** — informe `Host` (ex: `localhost` ou `192.168.1.10`), `Porta` (5432), `Usuário` (`postgres`), **Senha** e `Banco` (`system_monitor`). O instalador gera `config.toml` automaticamente (senha em texto plano em `config.toml`).
2. **PostgreSQL** — marque *“Instalar PostgreSQL automaticamente se não estiver instalado”* se o host for `localhost` e você não tem PG. O instalador baixa a **versão mais recente** via `winget`/`choco` (fallback EDB) e já cria o usuário com a senha que você informou (ou gera uma se deixar em branco).
3. **Opcionais** — marque *smartmontools* (para SMART) e *PawnIO* (para 309 sensores) se quiser — também instalados via `winget`/`choco`.

O instalador então:
- copia `monitor-go.exe` + `lhm-dump.exe` (sensores) para `C:\Program Files\system-monitor`
- grava `C:\Program Files\system-monitor\config.toml` com o `DATABASE_URL` que você informou
- instala PostgreSQL (se marcado e ausente) e opcionais
- roda `monitor-go --init` (cria o banco `system_monitor` + 16 tabelas) — **sem precisar editar `.env` manualmente**
- registra 3 tarefas no boot (SYSTEM): `SystemMonitor-Go` (coletor), `SystemMonitor-Go-Dashboard` (dashboard) e `SystemMonitor-Go-Retention` (limpeza, só se ativada)

### 4. Confira se está coletando
```powershell
Get-Content "C:\Program Files\system-monitor\logs\monitor-go.log" -Tail 20
# deve mostrar: [sensors] 309 rows -> monitor.sensors ... stored  (ou 143 sem PawnIO)
Invoke-WebRequest http://localhost:8501/api/health -UseBasicParsing
# StatusCode 200 = dashboard ok
```

Abra **http://localhost:8501** no navegador. Pronto — coleta já está no boot e sobrevive a reinícios. Logs em `logs/monitor-go.log`; se o banco cair, os dados ficam em `pending_batches.sqlite3` e são reenviados.

---

## O que você precisa antes

| Obrigatório | Detalhe |
|---|---|
| **Windows 10/11** | Testado em 11. Roda como `SYSTEM` via Tarefas Agendadas |
| **PostgreSQL 14+** | Local ou remoto. Só precisa de `DATABASE_URL`. Não instala PG sozinho |

| Opcional (melhora dados, não trava) | O que ganha |
|---|---|
| **PawnIO + LibreHardwareMonitor** | 309 sensores (CPU `Tctl`/`CCD1`, SuperIO Nuvoton, fans/voltages) vs 143 sem |
| **smartmontools** (`smartctl`) | SMART de discos: `temperature_c`, `power_on_hours`, `percentage_used` |
| **nvidia-smi** | GPU NVIDIA: util, memória, temp, power, clocks |
| **.NET Framework 4.7.2** | Já vem no Windows 11; só para `lhm-dump.exe` |

Sem esses opcionais o agente ainda sobe — as tabelas correspondentes ficam com `no_sensor` ou vazias.

---

## Passo a passo detalhado

### Opção A — Instalador (recomendado)
1. Baixe `system-monitor-*-setup.exe` em Releases.
2. Clique direito → **Executar como administrador** → wizard pede `Host/Porta/Usuário/Senha/Banco` e se deve instalar PostgreSQL/opcionais → Avançar.
3. O instalador já gera `config.toml` e roda `--init`. Confira `http://localhost:8501`.
4. Se precisar corrigir, edite `C:\Program Files\system-monitor\config.toml` (`[db] url`) e rode `& "C:\Program Files\system-monitor\monitor-go.exe" --init`.

**Desinstalar:** Painel de Controle → Programas → System Monitor → Desinstalar (remove as 3 tarefas).

### Opção B — `install.ps1` (sem gerar `setup.exe`)
Para quem clonou o repo e quer instalar direto sem Inno Setup (pergunta interativa e gera `config.toml`):

```powershell
git clone https://github.com/dankkom/windows-system-monitor.git
cd windows-system-monitor
powershell -ExecutionPolicy Bypass -File installer/install.ps1
# responde Host/Porta/Usuário/Senha/Banco + opcionais no prompt
# ou não-interativo: .\installer\install.ps1 -DbHost 192.168.1.10 -DbPassword "xxx" -InstallPostgres -WithSmartTools
```

### Opção C — Manual / portátil (sem admin, sem tarefas)
Útil para testar sem instalar:

```powershell
copy config.toml.example config.toml
# edite [db] url  (ou defina env var DATABASE_URL)
.\go\monitor-go.exe --init
.\go\monitor-go.exe --dry-run   # testa coletores sem gravar
.\go\monitor-go.exe --once      # uma coleta completa
.\go\monitor-go.exe --serve     # só dashboard em http://localhost:8501
```

Para rodar coleta + dashboard juntos manualmente: `.\go\monitor-go.exe --collect --serve`.

---

## Primeiro acesso ao dashboard

- URL: **http://localhost:8501** (ou `http://SEU_HOST:8501` se mudou `[dashboard] host/port` em `config.toml` ou env `DASHBOARD_HOST/PORT`)
- Health: `/api/health` (200), `/api/ready`, `/api/status` (pendências do spool)
- Se não abrir, veja `logs/monitor-go.log` e `Get-ScheduledTask SystemMonitor-Go* | Get-ScheduledTaskInfo`.

---

## Configuração (config.toml)

Tudo via **`config.toml`** ao lado do `monitor-go.exe` (ou `C:\Program Files\system-monitor\config.toml` quando instalado). O instalador gera este arquivo automaticamente — **não precisa criar `.env` manualmente** (`.env` ainda é lido por compatibilidade, mas `config.toml` tem precedência). Só `db.url` é obrigatório.

```toml
[db]
url = "postgresql://postgres:senha@192.168.1.10:5432/system_monitor"
# url também pode ser sobrescrita por env var DATABASE_URL

[dashboard]
host = "127.0.0.1"
port = 8501

[intervals]
cpu = 10
sensors = 15
# ... veja config.toml.example para todos

[retention]
enabled = false
# para ativar: enabled = true  (e ajuste processes = "30 days" etc.)

[power]
aux_baseline_w = 24
```

Env vars ainda têm precedência (`DATABASE_URL`, `INTERVAL_CPU`, `RETENTION_*`, `DASHBOARD_PORT` etc.), útil para CI/docker. Após mudar `config.toml`, reinicie as tarefas. Teste retenção sem apagar: `monitor-go --retention-dry-run`. Senha fica em texto plano em `config.toml` (decisão de UX).

---

## Gerenciamento no dia a dia

```powershell
# logs
Get-Content "C:\Program Files\system-monitor\logs\monitor-go.log" -Tail 50

# pausar/retomar
Stop-ScheduledTask -TaskName SystemMonitor-Go, SystemMonitor-Go-Dashboard
Start-ScheduledTask -TaskName SystemMonitor-Go; Start-ScheduledTask -TaskName SystemMonitor-Go-Dashboard

# checar último heartbeat com falha
psql -U postgres -h HOST -d system_monitor -c "SELECT * FROM monitor.v_last_heartbeat WHERE success=false;"

# testar retenção
& "C:\Program Files\system-monitor\monitor-go.exe" --retention-dry-run
& "C:\Program Files\system-monitor\monitor-go.exe" --retention  # só se ENABLE_RETENTION=true
```

---

## O que é coletado

16 tabelas em `monitor` (intervalos configuráveis, histórico permanente):

| Domínio | Tabela | Intervalo | Fonte | Exemplo |
|---|---|---|---|---|
| CPU | `monitor.cpu` | 10s | gopsutil | total %, per-core, freq MHz |
| Memória | `monitor.memory` | 10s | gopsutil | total/used %, swap, pagefile |
| Disco uso | `monitor.disk_usage` | 60s | gopsutil | por volume → total/used/free |
| Disco IO | `monitor.disk_io` | 10s | gopsutil | read/write bytes, busy_time |
| Disco físico | `monitor.physical_disk` | 300s | Get-PhysicalDisk | SSD/HDD/NVMe, HealthStatus |
| SMART | `monitor.disk_smart` | 300s | smartctl -j | temperature, wear, power_on_hours |
| Rede IO | `monitor.net_io` | 10s | gopsutil | bytes/packets, err/drop |
| Rede addrs | `monitor.net_addr` | 60s | gopsutil | iface, address, netmask |
| GPU | `monitor.gpu` | 10s | nvidia-smi | util, mem, temp, power, fan |
| Sensores | `monitor.sensors` | 15s | lhm-dump.exe | 309 sensores quando SYSTEM+PawnIO |
| Processos | `monitor.processes` | 30s | gopsutil | top 50 por CPU/mem |
| Conexões | `monitor.connections` | 30s | gopsutil | TCP/UDP laddr/raddr, pid |
| Serviços | `monitor.services` | 60s | WMI Win32_Service | name, status, start_type |
| Sistema | `monitor.system_info` | 60s | gopsutil/host | boot_time, OS build, RAM |
| EventLog | `monitor.eventlog` | 60s | wevtutil | System/Application Error/Warning |
| Heartbeat | `monitor.heartbeat` | por coleta | interno | duration_ms, rows, success + `v_last_heartbeat` |

Dashboard calcula agregações com `lag()`/`dt`, bucket `to_timestamp(floor(epoch/bucket)*bucket)` e estimativa de potência ` (CPU Package + GPU + POWER_AUX_BASELINE_W) / POWER_PSU_EFFICIENCY`.

---

## Solução de problemas (instalação)

| Sintoma | Causa provável | O que fazer |
|---|---|---|
| `monitor-go --init` falha | PG offline ou `DATABASE_URL` errada | `Test-NetConnection HOST -Port 5432`; `Get-Service postgresql*`; confira senha em `config.toml` (`[db] url`) |
| `sensors` só 1 linha `no_sensor` | `lhm-dump.exe` não encontrado ou sem SYSTEM/PawnIO | Ao instalar, `lhm-dump.exe` fica ao lado de `monitor-go.exe`. Para 309 sensores, instale PawnIO e rode como SYSTEM (instalador já faz) |
| `disk_smart` vazio | `smartctl` não instalado | Instale [smartmontools](https://www.smartmontools.org/) ou deixe vazio — degrade gracioso |
| Dashboard não abre | Tarefa `SystemMonitor-Go-Dashboard` não iniciou | `Get-ScheduledTask SystemMonitor-Go-Dashboard | Get-ScheduledTaskInfo` → `LastTaskResult`; `Get-Content logs/monitor-go.log -Tail 30` |
| `Task LastTaskResult 1` | Instalador não rodou como admin | Reinstale clicando direito → Executar como administrador |

---

<details>
<summary><strong>Para desenvolvedores (build, arquitetura, CI)</strong></summary>

### Build local

```powershell
go build -o go/monitor-go.exe ./go/cmd/monitor          # CGO_ENABLED=0, ~15 MB
dotnet publish go/lhm-dump/lhm-dump.csproj -c Release -o build/lhm-dump
.\go\monitor-go.exe --dry-run; .\go\monitor-go.exe --once; Get-Content logs/monitor-go.log -Tail 20
```

### Arquitetura

```
go/cmd/monitor/main.go      # coletor + dashboard + --once/--dry-run/--init/--retention
go/internal/collectors/     # 15 coletores
go/internal/config/         # config.toml + INTERVAL_* + RETENTION_* (env > toml > default)
go/internal/db/             # pgx CopyFrom + spool SQLite (2 GB WAL) + schema embed
go/internal/dashboard/      # net/http + embed static/templates
go/lhm-dump/                # helper LHM net472
installer/system-monitor.iss  # wizard DATABASE_URL + PG/opcionais, gera config.toml
installer/install.ps1 / install_postgres.ps1 / install_optional.ps1
config.toml.example         # exemplo (gerado pelo instalador)
sql/schema.sql              # 16 tabelas (copiado em go/internal/db/schema.sql para embed)
```

Fluxo: `monitor-go` loop 1s tick (`services`/`processes` em goroutines) → `collectors` → `db.Store.InsertBatch` (`CopyFrom`) → `heartbeat` → `logs/monitor-go.log`. Dashboard serve `go/internal/dashboard`. Spool `pending_batches.sqlite3` bufferiza quando PG cai.

### CI/Release

- `ci.yml` (push/PR): `windows-latest` `go vet`/`build`/`test` + `dotnet publish` + artifact.
- `release.yml` (tag `v*`): `CGO_ENABLED=0` build + `dotnet publish` + `Compress-Archive` zip + `ISCC installer/system-monitor.iss` → `dist/*.exe` → `softprops/action-gh-release`.

</details>
