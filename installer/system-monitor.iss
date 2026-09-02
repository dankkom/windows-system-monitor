; Inno Setup script for system-monitor (Go + lhm-dump + schema + config.toml)
; Requer Inno Setup 6.x. Build via CI ou local: iscc installer/system-monitor.iss
#define MyAppName "System Monitor"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "dankkom"
#define MyAppURL "https://github.com/dankkom/windows-system-monitor"

[Setup]
AppId={{3B9E9A2B-7C0D-4E2A-9F2B-8D2A1C9E0F01}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
DefaultDirName={pf}\system-monitor
DisableDirPage=no
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
OutputDir=..\dist
OutputBaseFilename=system-monitor-{#MyAppVersion}-setup
Compression=lzma
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
PrivilegesRequiredOverridesAllowed=commandline
ArchitecturesInstallIn64BitMode=x64
UninstallDisplayIcon={app}\monitor-go.exe

[Languages]
Name: "brazilianportuguese"; MessagesFile: "compiler:Languages\BrazilianPortuguese.isl"
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
; Binario Go (build antes: go build -o go/monitor-go.exe ./go/cmd/monitor)
Source: "..\go\monitor-go.exe"; DestDir: "{app}"; Flags: ignoreversion
; Helper LHM (build antes: dotnet publish go/lhm-dump -c Release -o build\lhm-dump)
Source: "..\build\lhm-dump\lhm-dump.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\build\lhm-dump\*.dll"; DestDir: "{app}"; Flags: ignoreversion
; Fallback: se build\lhm-dump nao existir, tenta go\lhm-dump\bin
Source: "..\go\lhm-dump\bin\Release\net472\lhm-dump.exe"; DestDir: "{app}"; Flags: ignoreversion skipifsourcedoesntexist
Source: "..\go\lhm-dump\bin\Release\net472\*.dll"; DestDir: "{app}"; Flags: ignoreversion skipifsourcedoesntexist
; Config e schema
Source: "..\config.toml.example"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\.env.example"; DestDir: "{app}"; Flags: ignoreversion skipifsourcedoesntexist
Source: "..\sql\schema.sql"; DestDir: "{app}\sql"; Flags: ignoreversion
Source: "..\scripts\install_tasks_go.ps1"; DestDir: "{app}\scripts"; Flags: ignoreversion
Source: "..\scripts\install_retention_go.ps1"; DestDir: "{app}\scripts"; Flags: ignoreversion
Source: "install_postgres.ps1"; DestDir: "{app}\scripts"; Flags: ignoreversion
Source: "install_optional.ps1"; DestDir: "{app}\scripts"; Flags: ignoreversion
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion isreadme

[Icons]
Name: "{group}\Dashboard (http://localhost:8501)"; Filename: "http://localhost:8501"
Name: "{group}\Uninstall {#MyAppName}"; Filename: "{uninstallexe}"

[Run]
; Opcionais e Postgres sao tratados em [Code] CurStepChanged; aqui so fallback se Code nao rodou
Filename: "{app}\monitor-go.exe"; Parameters: "--init"; WorkingDir: "{app}"; Flags: runhidden; StatusMsg: "Inicializando banco de dados..."
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\scripts\install_tasks_go.ps1"""; Flags: runhidden; StatusMsg: "Registrando tarefas agendadas..."
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\scripts\install_retention_go.ps1"""; Flags: runhidden

[UninstallRun]
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -Command ""Unregister-ScheduledTask -TaskName 'SystemMonitor-Go' -Confirm:$false -ErrorAction SilentlyContinue; Unregister-ScheduledTask -TaskName 'SystemMonitor-Go-Dashboard' -Confirm:$false -ErrorAction SilentlyContinue; Unregister-ScheduledTask -TaskName 'SystemMonitor-Go-Retention' -Confirm:$false -ErrorAction SilentlyContinue"""; Flags: runhidden

[Code]
var
  DBPage: TInputQueryWizardPage;
  InstallPGCheck: TNewCheckBox;
  SmartToolsCheck: TNewCheckBox;
  PawnIOCheck: TNewCheckBox;
  GeneratedPassword: String;

function IsAdmin(): Boolean;
begin
  Result := IsAdminLoggedOn;
end;

function InitializeSetup(): Boolean;
begin
  Result := IsAdmin();
  if not Result then
    MsgBox('Este instalador requer privilégios de Administrador.', mbError, MB_OK);
end;

procedure InitializeWizard();
begin
  DBPage := CreateInputQueryPage(wpWelcome,
    'Banco de dados PostgreSQL',
    'Configure a conexão com o PostgreSQL',
    'Informe os dados de conexão. O instalador vai gerar config.toml automaticamente (senha em texto plano em config.toml). Se marcar "Instalar PostgreSQL", a versão mais recente será baixada via winget/choco (ou EDB) com senha gerada se deixar em branco.');

  DBPage.Add('Host (ex: localhost ou 192.168.1.10):', False);
  DBPage.Add('Porta:', False);
  DBPage.Add('Usuário:', False);
  DBPage.Add('Senha:', True);
  DBPage.Add('Banco:', False);

  DBPage.Values[0] := 'localhost';
  DBPage.Values[1] := '5432';
  DBPage.Values[2] := 'postgres';
  DBPage.Values[3] := '';
  DBPage.Values[4] := 'system_monitor';

  { Checkboxes abaixo dos edits }
  InstallPGCheck := TNewCheckBox.Create(WizardForm);
  InstallPGCheck.Parent := DBPage.Surface;
  InstallPGCheck.Top := DBPage.Edits[4].Top + DBPage.Edits[4].Height + 16;
  InstallPGCheck.Left := 0;
  InstallPGCheck.Width := DBPage.Surface.Width;
  InstallPGCheck.Caption := 'Instalar PostgreSQL automaticamente se não estiver instalado (via winget/choco, versão mais recente)';
  InstallPGCheck.Checked := True;

  SmartToolsCheck := TNewCheckBox.Create(WizardForm);
  SmartToolsCheck.Parent := DBPage.Surface;
  SmartToolsCheck.Top := InstallPGCheck.Top + InstallPGCheck.Height + 4;
  SmartToolsCheck.Left := 0;
  SmartToolsCheck.Width := DBPage.Surface.Width;
  SmartToolsCheck.Caption := 'Instalar smartmontools (smartctl) para SMART de discos (via winget/choco)';
  SmartToolsCheck.Checked := True;

  PawnIOCheck := TNewCheckBox.Create(WizardForm);
  PawnIOCheck.Parent := DBPage.Surface;
  PawnIOCheck.Top := SmartToolsCheck.Top + SmartToolsCheck.Height + 4;
  PawnIOCheck.Left := 0;
  PawnIOCheck.Width := DBPage.Surface.Width;
  PawnIOCheck.Caption := 'Tentar instalar PawnIO driver para 309 sensores (pode exigir confirmação manual)';
  PawnIOCheck.Checked := False;
end;

function GetRandomPassword(): String;
var
  I: Integer;
  Chars: String;
begin
  Chars := 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
  Result := '';
  for I := 1 to 16 do
    Result := Result + Chars[Random(Length(Chars)) + 1];
end;

function URLEncode(S: String): String;
var
  I: Integer;
  C: Char;
  Hex: String;
begin
  Result := '';
  for I := 1 to Length(S) do
  begin
    C := S[I];
    if (C >= 'a') and (C <= 'z') or (C >= 'A') and (C <= 'Z') or (C >= '0') and (C <= '9') or (C = '-') or (C = '_') or (C = '.') or (C = '~') then
      Result := Result + C
    else
    begin
      Hex := IntToHex(Ord(C), 2);
      Result := Result + '%' + Hex;
    end;
  end;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
var
  Host, Port, User, Pass, DB: String;
begin
  Result := True;
  if CurPageID = DBPage.ID then
  begin
    Host := Trim(DBPage.Values[0]);
    Port := Trim(DBPage.Values[1]);
    User := Trim(DBPage.Values[2]);
    Pass := DBPage.Values[3];
    DB := Trim(DBPage.Values[4]);
    if (Host = '') or (Port = '') or (User = '') or (DB = '') then
    begin
      MsgBox('Preencha Host, Porta, Usuário e Banco. Senha pode ficar em branco se for instalar PostgreSQL (será gerada).', mbError, MB_OK);
      Result := False;
    end;
  end;
end;

function UpdateReadyMemo(Space, NewLine, MemoUserInfoInfo, MemoDirInfo, MemoTypeInfo, MemoComponentsInfo, MemoGroupInfo, MemoTasksInfo: String): String;
var
  S: String;
begin
  S := '';
  S := S + 'Banco de dados:' + NewLine;
  S := S + Space + 'Host: ' + DBPage.Values[0] + ':' + DBPage.Values[1] + NewLine;
  S := S + Space + 'Usuário: ' + DBPage.Values[2] + '  Banco: ' + DBPage.Values[4] + NewLine;
  if InstallPGCheck.Checked then
    S := S + Space + 'PostgreSQL: instalar automaticamente se ausente' + NewLine
  else
    S := S + Space + 'PostgreSQL: usar existente (não instalar)' + NewLine;
  if SmartToolsCheck.Checked then
    S := S + Space + 'Opcional: smartmontools será instalado' + NewLine;
  if PawnIOCheck.Checked then
    S := S + Space + 'Opcional: PawnIO driver será tentado' + NewLine;
  S := S + NewLine + MemoDirInfo + NewLine + MemoGroupInfo + NewLine;
  Result := S;
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  Host, Port, User, Pass, DB, URL, ConfigPath, ConfigContent: String;
  ResultCode: Integer;
  InstallPG, SmartOK, PawnOK: String;
begin
  if CurStep = ssPostInstall then
  begin
    Host := Trim(DBPage.Values[0]);
    Port := Trim(DBPage.Values[1]);
    User := Trim(DBPage.Values[2]);
    Pass := DBPage.Values[3];
    DB := Trim(DBPage.Values[4]);

    if (Pass = '') and InstallPGCheck.Checked then
    begin
      GeneratedPassword := GetRandomPassword();
      Pass := GeneratedPassword;
    end;

    { Gera config.toml com senha em texto plano (decisão do usuário) }
    URL := 'postgresql://' + URLEncode(User) + ':' + URLEncode(Pass) + '@' + Host + ':' + Port + '/' + DB;

    ConfigPath := ExpandConstant('{app}\config.toml');
    ConfigContent :=
      '# Gerado pelo instalador System Monitor em ' + GetDateTimeString('yyyy-mm-dd hh:nn:ss', '-', ':') + #13#10 +
      '[db]' + #13#10 +
      'url = "' + URL + '"' + #13#10 +
      'connect_timeout = 10' + #13#10 +
      'retry_seconds = 30' + #13#10 +
      'buffer_max_bytes = 2147483648' + #13#10 + #13#10 +
      '[dashboard]' + #13#10 +
      'host = "127.0.0.1"' + #13#10 +
      'port = 8501' + #13#10 +
      'timezone = "America/Sao_Paulo"' + #13#10 + #13#10 +
      '[intervals]' + #13#10 +
      'cpu = 10' + #13#10 + 'memory = 10' + #13#10 + 'disk_io = 10' + #13#10 + 'disk_usage = 60' + #13#10 + 'disk_physical = 300' + #13#10 + 'disk_smart = 300' + #13#10 + 'network = 10' + #13#10 + 'gpu = 10' + #13#10 + 'sensors = 15' + #13#10 + 'processes = 30' + #13#10 + 'connections = 30' + #13#10 + 'services = 60' + #13#10 + 'system = 60' + #13#10 + 'eventlog = 60' + #13#10 + #13#10 +
      '[collector]' + #13#10 + 'top_processes = 50' + #13#10 + 'hostname = ""' + #13#10 + #13#10 +
      '[power]' + #13#10 + 'aux_baseline_w = 24' + #13#10 + 'psu_efficiency = 0.90' + #13#10 + 'gpu_idle_w = 10' + #13#10 + 'gpu_max_w = 160' + #13#10 + #13#10 +
      '[retention]' + #13#10 + 'enabled = false' + #13#10 + 'batch_limit = 50000' + #13#10 + 'batch_sleep = 0.1' + #13#10 + 'processes = "30 days"' + #13#10 + 'connections = "7 days"' + #13#10 + 'sensors = "90 days"' + #13#10 + 'cpu = "90 days"' + #13#10 + 'memory = "90 days"' + #13#10 + 'gpu = "90 days"' + #13#10 + 'heartbeat = "30 days"' + #13#10 + 'eventlog = "30 days"' + #13#10 + 'disk_io = "90 days"' + #13#10 + 'net_io = "90 days"' + #13#10 + #13#10 +
      '[log]' + #13#10 + 'level = "INFO"' + #13#10;

    SaveStringToFile(ConfigPath, ConfigContent, False);

    { Instala PostgreSQL se marcado }
    if InstallPGCheck.Checked then
    begin
      Exec('powershell.exe',
        '-NoProfile -ExecutionPolicy Bypass -File "' + ExpandConstant('{app}\scripts\install_postgres.ps1') + '" -SuperPassword "' + Pass + '" -Port ' + Port,
        '', SW_SHOW, ewWaitUntilTerminated, ResultCode);
    end;

    { Opcionais }
    InstallPG := 'false'; SmartOK := 'false'; PawnOK := 'false';
    if SmartToolsCheck.Checked then SmartOK := 'true';
    if PawnIOCheck.Checked then PawnOK := 'true';
    if SmartToolsCheck.Checked or PawnIOCheck.Checked then
    begin
      Exec('powershell.exe',
        '-NoProfile -ExecutionPolicy Bypass -File "' + ExpandConstant('{app}\scripts\install_optional.ps1') + '" ' +
        '-SmartTools:$' + SmartOK + ' -PawnIO:$' + PawnOK,
        '', SW_SHOW, ewWaitUntilTerminated, ResultCode);
    end;

    { Roda --init com o config.toml recém-gerado (pode falhar se PG ainda subindo, Run fallback tenta de novo) }
    Exec(ExpandConstant('{app}\monitor-go.exe'), '--init', ExpandConstant('{app}'), SW_HIDE, ewWaitUntilTerminated, ResultCode);
  end;
end;
