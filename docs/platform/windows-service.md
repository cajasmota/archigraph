# archigraph daemon — Windows service smoke-test guide

This document is for the Windows tester validating `archigraph install` / `archigraph uninstall` / `archigraph status` on a real Windows machine after the `schtasks_windows.go` implementation lands.

## Prerequisites

- Windows 10 or Windows 11 (Task Scheduler 1.4 — `version="1.4"` in the XML).
- A built `archigraph.exe` on the target machine. See [build instructions](#building-archigraph-exe-for-windows).
- PowerShell or `cmd.exe` running as the **current user** (no administrator needed).

## Building archigraph.exe for Windows

From a Unix or Windows machine with Go installed:

```bash
GOOS=windows GOARCH=amd64 go build -o archigraph.exe ./cmd/archigraph
```

Copy `archigraph.exe` to the target Windows machine.

---

## Smoke test procedure

### 1. Verify initial state — no task registered

```powershell
schtasks /query /tn com.archigraph.daemon
# Expected: ERROR: The system cannot find the file specified.
```

### 2. Install the daemon service

```powershell
.\archigraph.exe install
```

Expected output (approximate):

```
archigraph daemon service installed.
Status: installed=true running=true pid=<N>
```

### 3. Verify the task exists in Task Scheduler

```powershell
schtasks /query /tn com.archigraph.daemon /fo list /v
```

Key fields to check:

| Field | Expected value |
|---|---|
| `Task Name` | `\com.archigraph.daemon` |
| `Status` | `Running` |
| `Logon Mode` | `Interactive/Background` |
| `Run As User` | current Windows user |
| `Scheduled Task State` | `Enabled` |

### 4. Verify the daemon named pipe is connectable

```powershell
# Replace <username> with your lowercased Windows username
Test-Path "\\.\pipe\archigraph-daemon-<username>"
# Expected: True
```

Or use the archigraph status command:

```powershell
.\archigraph.exe status
# Expected: installed=true running=true pid=<N>
```

### 5. Verify the task XML was staged to disk

```powershell
# %LOCALAPPDATA% = C:\Users\<user>\AppData\Local
ls "$env:LOCALAPPDATA\archigraph\tasks\com.archigraph.daemon.xml"
# Expected: file exists
```

### 6. Simulate a crash-restart (RestartOnFailure)

```powershell
# Find the PID from step 4, then kill the daemon process:
Stop-Process -Id <pid> -Force

# Wait ~90 seconds (RestartOnFailure Interval is PT1M = 1 minute)
Start-Sleep -Seconds 90

# Check if the task restarted:
schtasks /query /tn com.archigraph.daemon /fo list /v | Select-String "Status"
# Expected: Status: Running
```

### 7. Verify idempotency — running install a second time

```powershell
.\archigraph.exe install
# Expected: no error, returns current status without modifying anything
```

### 8. Uninstall the daemon service

```powershell
.\archigraph.exe uninstall
```

Expected output (approximate):

```
archigraph daemon service removed.
```

### 9. Verify the task is gone

```powershell
schtasks /query /tn com.archigraph.daemon
# Expected: ERROR: The system cannot find the file specified.
```

### 10. Verify the XML file was removed

```powershell
ls "$env:LOCALAPPDATA\archigraph\tasks\com.archigraph.daemon.xml"
# Expected: file not found error
```

### 11. Verify logon persistence (cold-boot test)

1. Install the task: `.\archigraph.exe install`
2. Log off and log back in (or restart the machine).
3. After login, run: `.\archigraph.exe status`
4. Expected: `running=true` — the task fired at logon.

---

## Expected file paths

| Artifact | Path |
|---|---|
| Task XML (staged) | `%LOCALAPPDATA%\archigraph\tasks\com.archigraph.daemon.xml` |
| Daemon logs | `%APPDATA%\archigraph\logs\` |
| Named pipe | `\\.\pipe\archigraph-daemon-<lowercased-username>` |

---

## Known limitations in this release

- **No stdout/stderr log files.** Unlike macOS (plist `StandardOutPath`) and Linux (systemd journal), Windows Task Scheduler does not redirect process output to a file natively. The daemon writes logs to `%APPDATA%\archigraph\logs\` via its own logger. A future issue (#933) may wire file-based log redirection via XML `<StandardOutput>` extensions.
- **No code-signing.** The binary is unsigned. Windows SmartScreen may warn on first run. Administrators can unblock the binary via `Unblock-File .\archigraph.exe`.
- **UAC elevation not required.** The task runs at `LeastPrivilege` (user-level). If elevated tasks are needed in future, the installer would need to be updated — but that is out of scope for this release.
- **Crash-restart limited to 3 attempts.** After 3 failures within 1 minute, Task Scheduler stops restarting. Manual intervention via `archigraph install` is required to reset the retry counter.

---

## Troubleshooting

### Task appears in schtasks but daemon not running

Check whether the binary path in the task XML is correct:

```powershell
schtasks /query /tn com.archigraph.daemon /xml
```

Look at the `<Command>` element. If the path is wrong, run `.\archigraph.exe uninstall && .\archigraph.exe install` from the directory where `archigraph.exe` lives.

### `schtasks /create` fails with access denied

This should not happen at `LeastPrivilege`. If it does, check whether Group Policy on the machine restricts Task Scheduler registration for non-admins (`Computer Configuration > Windows Settings > Security Settings > Local Policies > User Rights Assignment > Increase scheduling priority`).

### Named pipe not appearing after install

The task may have started but crashed immediately. Check the Windows Event Log:

```powershell
Get-EventLog -LogName Application -Source "Task Scheduler" -Newest 20 | Format-List
```

Also check for archigraph-specific errors:

```powershell
Get-Content "$env:APPDATA\archigraph\logs\daemon.log" -Tail 50
```

---

## Sibling issues

- **#856** — Platform parity parent epic
- **#933** — SchtasksXML watcher unit tests
- **#935** — CI matrix for Windows runner
- **#937** — tree-sitter CGO bindings on Windows
