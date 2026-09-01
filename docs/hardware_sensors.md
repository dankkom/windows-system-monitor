# LibreHardwareMonitor - instalado

**Local:** `C:\tools\LibreHardwareMonitor\LibreHardwareMonitor.exe` (v0.9.6, 6.3 MB, portable)
**DLL:** `C:\tools\LibreHardwareMonitor\LibreHardwareMonitorLib.dll` - usado diretamente via `pythonnet` em `collectors/sensors.py`

**Integração:** `sensors.py` agora carrega a DLL via `clr` e cria um `Computer()` persistente com `IsCpuEnabled`, `IsGpuEnabled`, `IsMemoryEnabled`, `IsMotherboardEnabled`, `IsNetworkEnabled`, etc. Coleta 127 sensores por intervalo (antes: 1). Sem necessidade de GUI rodando.

**Dados coletados (não-elevado):**
- CPU per-core Load (16 cores), Total, Core Max
- GPU: Temperature Core 48-49C, Hot Spot 57C, clocks 210/405 MHz, fan 0%, load, power 10.7W, VRAM
- Memória: Used/Available, Load
- Rede: Upload/Download, throughput Wi-Fi ~4KB/s

**Limitação sem admin:** `CPU Temperature Core (Tctl/Tdie)` retorna 0.0 e `Clock Effective` 0.0 porque acesso ao SMU requer driver kernel (PawnIO/WinRing0). Para liberar temps reais e fans da placa-mãe:

1. Execute `C:\tools\PawnIO_setup.exe` como Administrador (instala driver PawnIO 2.2.0)
2. Rode o monitor elevado: clique direito PowerShell -> Executar como administrador -> `powershell -ExecutionPolicy Bypass -File C:\scripts\system-monitor\install_tasks.ps1`

**Arquivos salvos:** `PawnIO_setup.exe` (3.2 MB) em `C:\tools\` para instalação futura. Não é obrigatório para operação atual.

**Verificação:**
```powershell
python -c "import sys; sys.path.insert(0, 'C:\scripts\system-monitor'); from collectors.sensors import collect; c,r=collect('test'); print(len(r))"
psql -U postgres -h localhost -d system_monitor -c "SELECT sensor_type, count(*) FROM monitor.sensors WHERE ts > now()-'1 min'::interval GROUP BY 1;"
```
