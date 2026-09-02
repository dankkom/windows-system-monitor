# Instala componentes opcionais: smartmontools e PawnIO
param(
    [switch]$SmartTools,
    [switch]$PawnIO
)
$ErrorActionPreference = "Continue"

if ($SmartTools) {
    if (Get-Command smartctl -ErrorAction SilentlyContinue) {
        Write-Host "smartctl já instalado — pulando." -ForegroundColor Green
    } else {
        Write-Host "Instalando smartmontools..." -ForegroundColor Cyan
        $ok = $false
        if (Get-Command winget -ErrorAction SilentlyContinue) {
            try { & winget install --id smartmontools.smartmontools -e --silent --accept-package-agreements --accept-source-agreements 2>&1 | Out-String | Write-Host; if ($LASTEXITCODE -eq 0) { $ok = $true } } catch { Write-Warning "winget smartmontools falhou: $_" }
        }
        if (-not $ok -and (Get-Command choco -ErrorAction SilentlyContinue)) {
            try { & choco install smartmontools -y --no-progress 2>&1 | Out-String | Write-Host; if ($LASTEXITCODE -eq 0) { $ok = $true } } catch { Write-Warning "choco smartmontools falhou: $_" }
        }
        if (-not $ok) { Write-Warning "smartmontools não pôde ser instalado automaticamente. Baixe em https://www.smartmontools.org/" }
    }
}

if ($PawnIO) {
    $svc = Get-Service -Name "PawnIO" -ErrorAction SilentlyContinue
    if ($svc -and $svc.Status -eq "Running") {
        Write-Host "PawnIO já instalado — pulando." -ForegroundColor Green
    } else {
        Write-Host "PawnIO — instalação requer driver assinado." -ForegroundColor Yellow
        Write-Host "Baixe o instalador PawnIO em https://github.com/... (LibreHardwareMonitor docs) e execute como admin." -ForegroundColor Yellow
        # Tentativa via choco se houver pacote (não oficial, pode falhar)
        if (Get-Command choco -ErrorAction SilentlyContinue) {
            try { & choco install pawnio -y --no-progress 2>&1 | Out-String | Write-Host } catch {}
        }
        # Verifica se após tentativa o serviço apareceu
        Start-Sleep -Seconds 2
        $svc2 = Get-Service -Name "PawnIO" -ErrorAction SilentlyContinue
        if (-not $svc2) { Write-Warning "PawnIO não instalado automaticamente — siga instruções manuais para 309 sensores." }
    }
}

Write-Host "Opcionais concluídos." -ForegroundColor Green
