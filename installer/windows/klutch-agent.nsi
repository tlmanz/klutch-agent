; NSIS wizard installer for the Klutch print agent (Windows).
;
; Builds a standard Next > Next > Finish installer that drops the agent into
; Program Files, adds Start-menu + Desktop shortcuts, optionally starts it at
; login (minimised to the tray), and registers an uninstaller. The agent binary
; must sit next to this script as "klutch-agent.exe" at build time (CI copies it).
;
; Build:  makensis /DVERSION=v1.2.3 klutch-agent.nsi   ->  klutch-agent-setup.exe

!include "MUI2.nsh"

!ifndef VERSION
  !define VERSION "dev"
!endif

Name "Klutch Agent"
OutFile "klutch-agent-setup.exe"
Unicode True
InstallDir "$PROGRAMFILES64\Klutch Agent"
InstallDirRegKey HKLM "Software\Klutch Agent" "InstallDir"
RequestExecutionLevel admin

!define APP_EXE "klutch-agent.exe"
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\KlutchAgent"

; Official Klutch mark: branding for the wizard, shortcuts, and Add/Remove entry.
!define MUI_ICON "klutch-agent.ico"
!define MUI_UNICON "klutch-agent.ico"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
; Finish page: offer to launch now.
!define MUI_FINISHPAGE_RUN "$INSTDIR\${APP_EXE}"
!define MUI_FINISHPAGE_RUN_TEXT "Start Klutch Agent now"
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "English"

Section "Klutch Agent" SecMain
  SectionIn RO
  SetOutPath "$INSTDIR"
  File "${APP_EXE}"

  ; Start-menu + desktop shortcuts (start minimised to tray).
  CreateDirectory "$SMPROGRAMS\Klutch Agent"
  CreateShortcut "$SMPROGRAMS\Klutch Agent\Klutch Agent.lnk" "$INSTDIR\${APP_EXE}"
  CreateShortcut "$DESKTOP\Klutch Agent.lnk" "$INSTDIR\${APP_EXE}"

  ; Autostart at login (the app's Settings tab can toggle this off later).
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "Klutch Agent" '"$INSTDIR\${APP_EXE}" -tray'

  ; Record install + uninstall metadata.
  WriteRegStr HKLM "Software\Klutch Agent" "InstallDir" "$INSTDIR"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayName" "Klutch Agent"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "${UNINST_KEY}" "Publisher" "Klutch"
  WriteRegStr HKLM "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\${APP_EXE}"
  WriteRegStr HKLM "${UNINST_KEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteUninstaller "$INSTDIR\uninstall.exe"
SectionEnd

Section "Uninstall"
  ; Stop a running instance so the exe is not locked.
  ExecWait 'taskkill /IM ${APP_EXE} /F'

  Delete "$INSTDIR\${APP_EXE}"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"

  Delete "$SMPROGRAMS\Klutch Agent\Klutch Agent.lnk"
  RMDir "$SMPROGRAMS\Klutch Agent"
  Delete "$DESKTOP\Klutch Agent.lnk"

  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "Klutch Agent"
  DeleteRegKey HKLM "${UNINST_KEY}"
  DeleteRegKey HKLM "Software\Klutch Agent"
  ; Note: leaves %APPDATA%\klutch-agent (device token + job history) intact.
SectionEnd
