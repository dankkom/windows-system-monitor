; Inno Setup script for system-monitor (Go + lhm-dump + schema)
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
Source: "..\.env.example"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\sql\schema.sql"; DestDir: "{app}\sql"; Flags: ignoreversion
Source: "..\scripts\install_tasks_go.ps1"; DestDir: "{app}\scripts"; Flags: ignoreversion
Source: "..\scripts\install_retention_go.ps1"; DestDir: "{app}\scripts"; Flags: ignoreversion
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion isreadme

[Icons]
Name: "{group}\Dashboard (http://localhost:8501)"; Filename: "http://localhost:8501"
Name: "{group}\Uninstall {#MyAppName}"; Filename: "{uninstallexe}"

[Run]
; Cria .env a partir de .env.example se nao existir (usuario edita depois)
Filename: "{cmd}"; Parameters: "/c if not exist ""{app}\.env"" copy /y ""{app}\.env.example"" ""{app}\.env"""; Flags: runhidden
; Aplica schema (cria DB se faltar) - falha silenciosa se PG offline, usuario roda monitor-go --init depois
Filename: "{app}\monitor-go.exe"; Parameters: "--init"; WorkingDir: "{app}"; Flags: runhidden; StatusMsg: "Inicializando banco de dados..."
; Registra tarefas SYSTEM (coletor + dashboard)
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\scripts\install_tasks_go.ps1"""; Flags: runhidden; StatusMsg: "Registrando tarefas agendadas..."
; Retencao opcional (so cria task, inativa se ENABLE_RETENTION=false)
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -File ""{app}\scripts\install_retention_go.ps1"""; Flags: runhidden

[UninstallRun]
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -Command ""Unregister-ScheduledTask -TaskName 'SystemMonitor-Go' -Confirm:$false -ErrorAction SilentlyContinue; Unregister-ScheduledTask -TaskName 'SystemMonitor-Go-Dashboard' -Confirm:$false -ErrorAction SilentlyContinue; Unregister-ScheduledTask -TaskName 'SystemMonitor-Go-Retention' -Confirm:$false -ErrorAction SilentlyContinue"""; Flags: runhidden

[Code]
function InitializeSetup(): Boolean;
begin
  Result := IsAdmin();
  if not Result then
    MsgBox('Este instalador requer privilégios de Administrador.', mbError, MB_OK);
end;
