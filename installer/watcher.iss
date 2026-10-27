; Installer untuk MARSA Print Watcher (dipasang di laptop guru)
; Folder "dist" harus berisi: watcher.exe

#define MyAppName "MARSA Print Watcher"
#define MyAppVersion "1.0"

[Setup]
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher=SMK Ma'arif 9 Kebumen
DefaultDirName={autopf}\MarsaPrintWatcher
DefaultGroupName=MARSA Print Watcher
DisableProgramGroupPage=yes
OutputDir=output
OutputBaseFilename=MarsaPrintWatcherSetup
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64

[Files]
Source: "dist\watcher.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{userstartup}\MARSA Print Watcher"; Filename: "{app}\watcher.exe"

[Run]
; Daftarkan printer virtual "MARSA Print Server" pakai driver bawaan
; Windows "Microsoft Print To PDF" - tidak perlu driver tambahan apapun.
Filename: "powershell.exe"; Parameters: "-NoProfile -Command ""Add-Printer -Name 'MARSA Print Server' -DriverName 'Microsoft Print To PDF' -PortName 'PORTPROMPT:'"""; Flags: runhidden waituntilterminated; StatusMsg: "Mendaftarkan printer virtual..."
; Jalankan watcher sekarang juga (tanpa tunggu login ulang)
Filename: "{app}\watcher.exe"; Flags: nowait postinstall skipifsilent runasoriginaluser; Description: "Jalankan MARSA Print Watcher sekarang"

[UninstallRun]
Filename: "powershell.exe"; Parameters: "-NoProfile -Command ""Remove-Printer -Name 'MARSA Print Server'"""; Flags: runhidden; RunOnceId: "RemovePrinter"
Filename: "{cmd}"; Parameters: "/C taskkill /IM watcher.exe /F"; Flags: runhidden; RunOnceId: "KillWatcher"

[UninstallDelete]
Type: files; Name: "{userstartup}\MARSA Print Watcher.lnk"
