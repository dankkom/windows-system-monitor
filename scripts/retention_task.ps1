# LEGADO: delega para install_retention_go.ps1 (Go). Mantido para compatibilidade.
# Uso recomendado: .\scripts\install_retention_go.ps1  (requer admin, usa SYSTEM + monitor-go --retention)
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$GoScript = Join-Path $Root "scripts\install_retention_go.ps1"
if (Test-Path $GoScript) {
    & $GoScript
} else {
    throw "install_retention_go.ps1 nao encontrado. Execute monitor-go --retention manualmente."
}
