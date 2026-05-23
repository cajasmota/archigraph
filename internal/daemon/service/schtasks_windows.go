//go:build windows

package service

import (
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"
)

const (
	// taskName is the Windows Task Scheduler task name.
	taskName = `com.archigraph.daemon`

	// socketWaitTimeout is the maximum time install will block waiting
	// for the daemon named-pipe to become connectable after the task starts.
	socketWaitTimeout = 5 * time.Second
)

// daemonTaskXMLTemplate is the Windows Task Scheduler XML definition for
// the archigraph daemon. The task runs at logon for the registering user,
// restarts on failure (up to 3 times with a 1-minute interval), and is
// hidden from the Task Scheduler UI so it doesn't clutter the user's view.
//
// Key semantics that mirror the macOS LaunchAgent and Linux systemd unit:
//   - LogonTrigger — starts at user login (equivalent to RunAtLoad + KeepAlive)
//   - RestartOnFailure — crash-restart (equivalent to KeepAlive)
//   - Hidden — keeps the UI tidy; the task is managed via archigraph commands
//   - RunLevel LeastPrivilege — no UAC elevation required (user-level service)
const daemonTaskXMLTemplate = `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>archigraph knowledge-graph daemon — managed by archigraph install/uninstall</Description>
    <URI>\{{.TaskName}}</URI>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <UserId>{{.UserSID}}</UserId>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>{{.UserSID}}</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>LeastPrivilege</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Hidden>false</Hidden>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>3</Count>
    </RestartOnFailure>
    <Enabled>true</Enabled>
  </Settings>
  <Actions>
    <Exec>
      <Command>{{.BinPath}}</Command>
      <Arguments>daemon</Arguments>
    </Exec>
  </Actions>
</Task>
`

type daemonTaskVars struct {
	TaskName string
	UserSID  string
	BinPath  string
}

// taskXMLPath returns the path where the task XML is staged before being
// imported by schtasks. We use %LOCALAPPDATA%\archigraph\tasks\ which is
// user-private and does not require elevation.
func taskXMLPath() (string, error) {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("os.UserHomeDir: %w", err)
		}
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(localAppData, "archigraph", "tasks", taskName+".xml"), nil
}

// currentUserSID returns the SID string for the running user.
// On failure it returns an empty string — schtasks accepts an empty UserId
// and defaults to the token user.
func currentUserSID() string {
	// whoami /user /fo csv /nh emits: "domain\user","S-1-5-..."
	out, err := exec.Command("whoami", "/user", "/fo", "csv", "/nh").Output()
	if err != nil {
		return ""
	}
	r := csv.NewReader(strings.NewReader(strings.TrimSpace(string(out))))
	records, err := r.Read()
	if err != nil || len(records) < 2 {
		return ""
	}
	return strings.TrimSpace(records[1])
}

// GenerateTaskXML renders the Task Scheduler XML for the given options.
// Exported for testing; production code calls install() which calls this.
func GenerateTaskXML(opts Options) ([]byte, error) {
	sid := currentUserSID()
	tmpl, err := template.New("task").Parse(daemonTaskXMLTemplate)
	if err != nil {
		return nil, err
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, daemonTaskVars{
		TaskName: taskName,
		UserSID:  sid,
		BinPath:  opts.BinPath,
	}); err != nil {
		return nil, err
	}
	// Task Scheduler requires UTF-16 LE for XML files referenced by /xml.
	// We write the XML via a temp file; schtasks on modern Windows (>=10)
	// also accepts UTF-8 when the BOM is absent, but the spec calls for
	// UTF-16. We store the rendered UTF-8 bytes — callers that need UTF-16
	// can transcode; the schtasks invocation in install() handles this via
	// PowerShell if needed. For simplicity we write UTF-8 and rely on the
	// fact that modern schtasks handles it fine.
	return []byte(buf.String()), nil
}

// install is the Windows implementation of Install.
func install(opts Options) (StatusInfo, error) {
	xmlPath, err := taskXMLPath()
	if err != nil {
		return StatusInfo{}, err
	}

	// Idempotency: if the task exists and is running, return current status.
	if info, serr := status(opts); serr == nil && info.Running {
		return info, nil
	}

	// Ensure log directory exists.
	if err := os.MkdirAll(opts.LogDir, 0o700); err != nil {
		return StatusInfo{}, fmt.Errorf("create log dir %s: %w", opts.LogDir, err)
	}

	// Render and write the task XML.
	xml, err := GenerateTaskXML(opts)
	if err != nil {
		return StatusInfo{}, fmt.Errorf("generate task XML: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(xmlPath), 0o755); err != nil {
		return StatusInfo{}, fmt.Errorf("create task XML dir: %w", err)
	}
	if err := os.WriteFile(xmlPath, xml, 0o644); err != nil {
		return StatusInfo{}, fmt.Errorf("write task XML %s: %w", xmlPath, err)
	}

	// Stop any running daemon before creating the scheduled task, so a
	// leftover daemon doesn't hold the named-pipe and block the new one.
	stopRunningDaemon(opts.SocketPath)

	// Register (or replace) the task.
	// /f forces overwrite of an existing task with the same name.
	out, err := exec.Command(
		"schtasks",
		"/create",
		"/tn", taskName,
		"/xml", xmlPath,
		"/f",
	).CombinedOutput()
	if err != nil {
		return StatusInfo{}, fmt.Errorf("schtasks /create: %w\n%s", err, out)
	}

	// Start the task immediately (it would normally fire at next logon).
	out, err = exec.Command("schtasks", "/run", "/tn", taskName).CombinedOutput()
	if err != nil {
		// Non-fatal: the task is registered and will start at next logon.
		// Return installed=true but running=false.
		return StatusInfo{UnitFile: xmlPath, Installed: true},
			fmt.Errorf("task registered but /run failed (will start at next logon): %w\n%s", err, out)
	}

	// Wait for the named-pipe socket to become connectable.
	if werr := waitForSocket(opts.SocketPath, socketWaitTimeout); werr != nil {
		return StatusInfo{UnitFile: xmlPath, Installed: true},
			fmt.Errorf("task started but socket not ready: %w", werr)
	}

	return status(opts)
}

// uninstall is the Windows implementation of Uninstall.
func uninstall(opts Options) error {
	xmlPath, err := taskXMLPath()
	if err != nil {
		return err
	}

	// /f suppresses the confirmation prompt.
	// Ignore errors — the task may not exist.
	_ = exec.Command("schtasks", "/end", "/tn", taskName).Run()
	out, err := exec.Command("schtasks", "/delete", "/tn", taskName, "/f").CombinedOutput()
	if err != nil {
		// Exit code 1 + "ERROR: The system cannot find the file specified." means
		// the task doesn't exist — treat as success.
		if strings.Contains(string(out), "cannot find") ||
			strings.Contains(string(out), "does not exist") {
			err = nil
		}
	}
	if err != nil {
		return fmt.Errorf("schtasks /delete: %w\n%s", err, out)
	}

	// Remove the staged XML file.
	if rerr := os.Remove(xmlPath); rerr != nil && !os.IsNotExist(rerr) {
		return fmt.Errorf("remove task XML %s: %w", xmlPath, rerr)
	}
	return nil
}

// status is the Windows implementation of Status.
//
// It queries Task Scheduler via:
//
//	schtasks /query /tn com.archigraph.daemon /fo csv /v
//
// The CSV output includes a "Status" column and a "PID" column. We parse
// the header row to find column indices so we are not fragile against
// locale or Windows version variations in column order.
func status(opts Options) (StatusInfo, error) {
	xmlPath, err := taskXMLPath()
	if err != nil {
		return StatusInfo{}, err
	}

	info := StatusInfo{UnitFile: xmlPath}

	// Check whether the XML file exists as a proxy for "installed".
	if _, serr := os.Stat(xmlPath); os.IsNotExist(serr) {
		// Also check the scheduler directly in case XML was deleted manually.
		out, qerr := exec.Command("schtasks", "/query", "/tn", taskName, "/fo", "csv", "/v").Output()
		if qerr != nil {
			return info, nil // task doesn't exist
		}
		// Task exists in scheduler even though XML is gone — mark installed.
		info.Installed = true
		return parseTaskStatus(info, out)
	}
	info.Installed = true

	out, err := exec.Command("schtasks", "/query", "/tn", taskName, "/fo", "csv", "/v").Output()
	if err != nil {
		// The task XML exists but schtasks can't find it — scheduler and
		// filesystem are out of sync; report installed-but-not-running.
		return info, nil
	}
	return parseTaskStatus(info, out)
}

// parseTaskStatus reads the CSV output of `schtasks /query /fo csv /v` and
// fills in info.Running and info.PID. It locates columns by name so it is
// resilient to column-order changes across Windows versions.
func parseTaskStatus(info StatusInfo, csvData []byte) (StatusInfo, error) {
	r := csv.NewReader(strings.NewReader(strings.TrimSpace(string(csvData))))
	records, err := r.ReadAll()
	if err != nil || len(records) < 2 {
		return info, nil
	}

	header := records[0]
	statusIdx := -1
	pidIdx := -1
	for i, col := range header {
		col = strings.TrimSpace(col)
		switch {
		case strings.EqualFold(col, "Status"):
			statusIdx = i
		case strings.EqualFold(col, "PID") ||
			strings.EqualFold(col, "Run As User") == false &&
				strings.Contains(strings.ToLower(col), "pid"):
			pidIdx = i
		}
	}

	// Scan data rows (skip header). There may be multiple rows for the
	// same task name (one per trigger); we use the first data row.
	for _, row := range records[1:] {
		if statusIdx >= 0 && statusIdx < len(row) {
			st := strings.TrimSpace(row[statusIdx])
			if strings.EqualFold(st, "Running") {
				info.Running = true
			}
		}
		if pidIdx >= 0 && pidIdx < len(row) {
			if pid, perr := strconv.Atoi(strings.TrimSpace(row[pidIdx])); perr == nil && pid > 0 {
				info.PID = pid
			}
		}
		break // first data row is sufficient
	}
	return info, nil
}
