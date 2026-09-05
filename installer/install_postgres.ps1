# Instala PostgreSQL automaticamente se não estiver presente.
# Uso: powershell -ExecutionPolicy Bypass -File install_postgres.ps1 [-SuperPassword "xxx"]
param(
    [string]$SuperPassword = "",
    [int]$Port = 5432
)
$ErrorActionPreference = "Stop"

function Test-PostgresInstalled {
    $svc = Get-Service -Name "postgresql*" -ErrorAction SilentlyContinue | Where-Object { $_.Status -eq "Running" }
    if ($svc) { return $true }
    if (Get-Command psql -ErrorAction SilentlyContinue) { return $true }
    if (Test-Path "C:\Program Files\PostgreSQL\*\bin\psql.exe") { return $true }
    return $false
}

if (Test-PostgresInstalled) {
    Write-Host "PostgreSQL já instalado — pulando." -ForegroundColor Green
    exit 0
}

if ([string]::IsNullOrWhiteSpace($SuperPassword)) {
    # gera senha aleatória 16 chars
    $SuperPassword = -join ((48..57)+(65..90)+(97..122) | Get-Random -Count 16 | ForEach-Object { [char]$_ })
    Write-Host "Senha gerada para superusuário postgres: $SuperPassword" -ForegroundColor Yellow
    Write-Host "Guarde esta senha — ela será usada em config.toml DATABASE_URL"
}

Write-Host "PostgreSQL não encontrado — tentando instalar versão mais recente..." -ForegroundColor Cyan

# Tenta winget primeiro (mais recente)
$installed = $false
if (Get-Command winget -ErrorAction SilentlyContinue) {
    Write-Host "Tentando winget install PostgreSQL.PostgreSQL..." -ForegroundColor Cyan
    try {
        # winget instala PostgreSQL EDB silencioso passando a senha e porta
        $overrideArgs = "--mode unattended --superpassword ""$SuperPassword"" --serverport $Port"
        $wingetArgs = @("install", "--id", "PostgreSQL.PostgreSQL", "-e", "--silent", "--accept-package-agreements", "--accept-source-agreements", "--override", $overrideArgs)
        & winget @wingetArgs 2>&1 | Out-String | Write-Host
        if ($LASTEXITCODE -eq 0) { $installed = $true }
    } catch { Write-Warning "winget falhou: $_" }
}

if (-not $installed -and (Get-Command choco -ErrorAction SilentlyContinue)) {
    Write-Host "Tentando choco install postgresql..." -ForegroundColor Cyan
    try {
        # choco postgresql package aceita --params '/Password:xxx /Port:5432'
        $params = "/Password:$SuperPassword /Port:$Port"
        & choco install postgresql --params "$params" -y --no-progress 2>&1 | Out-String | Write-Host
        if ($LASTEXITCODE -eq 0) { $installed = $true }
    } catch { Write-Warning "choco falhou: $_" }
}

if (-not $installed) {
    Write-Host "Tentando download direto EDB (fallback)..." -ForegroundColor Cyan
    try {
        $edbUrl = "https://sbp.enterprisedb.com/getfile.jsp?fileid=1849529" # placeholder; atualize para latest
        # Tenta resolver latest via winget manifest se EDB URL quebrar, avisa usuário
        $tmp = Join-Path $env:TEMP "postgresql-latest.exe"
        Write-Host "Baixando EDB installer (~300MB) para $tmp ..."
        # Usa bits ou Invoke-WebRequest com progresso
        & powershell -Command "Invoke-WebRequest -Uri '$edbUrl' -OutFile '$tmp' -UseBasicParsing" 2>&1 | Out-String | Write-Host
        if (Test-Path $tmp) {
            Write-Host "Executando instalador silencioso..."
            $args = @("--mode", "unattended", "--unattendedmodeui", "minimal", "--superpassword", $SuperPassword, "--serverport", "$Port", "--servicename", "postgresql-x64-17")
            Start-Process -FilePath $tmp -ArgumentList $args -Wait -Verb RunAs
            if (Test-PostgresInstalled) { $installed = $true }
        }
    } catch { Write-Warning "EDB download falhou: $_" }
}

if (-not (Test-PostgresInstalled)) {
    Write-Warning "PostgreSQL não pôde ser instalado automaticamente. Instale manualmente: https://www.postgresql.org/download/windows/ e depois rode system-monitor --init"
    exit 1
}

Write-Host "PostgreSQL instalado com sucesso." -ForegroundColor Green
# Aguarda serviço subir
Start-Sleep -Seconds 5
# Verifica conexão com a senha gerada (se foi usada)
if (-not [string]::IsNullOrWhiteSpace($SuperPassword)) {
    $env:PGPASSWORD = $SuperPassword
    try { & psql -U postgres -h localhost -p $Port -d postgres -c "SELECT 1" 2>&1 | Out-Null } catch {}
}

exit 0
