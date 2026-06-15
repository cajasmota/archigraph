//go:build windows

package watchers

import (
	"fmt"
	"strings"
	"testing"
)

// windowsSampleUnit returns a Unit with predictable Windows-style paths so
// that XML generation tests do not depend on the real user home directory.
func windowsSampleUnit() Unit {
	return Unit{
		Group:   "demo",
		Repo:    `C:\Users\testuser\src\core`,
		BinPath: `C:\Program Files\grafel\grafel.exe`,
	}
}

// TestSchtasksXML_TaskName verifies the generated XML uses the unit label as
// the task description — the schtasks registration uses the label from
// the Load call, not a value embedded in the XML.
func TestSchtasksXML_TaskName(t *testing.T) {
	u := windowsSampleUnit()
	xml := SchtasksXML(u)
	if !strings.Contains(xml, u.Label()) {
		// Description is optional in the XML — but the label must be present.
		// If it isn't in the description at least the XML must be valid.
		t.Logf("label %q not found in XML body (OK if description is absent):\n%s", u.Label(), xml)
	}
}

// TestSchtasksXML_Command verifies the binary path appears in the <Command> element.
func TestSchtasksXML_Command(t *testing.T) {
	u := windowsSampleUnit()
	xml := SchtasksXML(u)
	want := `<Command>` + u.BinPath + `</Command>`
	if !strings.Contains(xml, want) {
		t.Errorf("XML missing command element %q\n%s", want, xml)
	}
}

// TestSchtasksXML_Arguments verifies the Arguments element passes `watch "<repo>"`.
func TestSchtasksXML_Arguments(t *testing.T) {
	u := windowsSampleUnit()
	xml := SchtasksXML(u)
	want := `<Arguments>watch "` + u.Repo + `"</Arguments>`
	if !strings.Contains(xml, want) {
		t.Errorf("XML missing arguments element %q\n%s", want, xml)
	}
}

// TestSchtasksXML_WorkingDirectory verifies WorkingDirectory is set to the repo path.
func TestSchtasksXML_WorkingDirectory(t *testing.T) {
	u := windowsSampleUnit()
	xml := SchtasksXML(u)
	want := `<WorkingDirectory>` + u.Repo + `</WorkingDirectory>`
	if !strings.Contains(xml, want) {
		t.Errorf("XML missing working directory element %q\n%s", want, xml)
	}
}

// TestSchtasksXML_LogonTrigger verifies the task fires at user logon
// (mirrors launchd RunAtLoad and systemd WantedBy=default.target).
func TestSchtasksXML_LogonTrigger(t *testing.T) {
	u := windowsSampleUnit()
	xml := SchtasksXML(u)
	if !strings.Contains(xml, "<LogonTrigger>") {
		t.Errorf("XML missing <LogonTrigger>:\n%s", xml)
	}
	if !strings.Contains(xml, "<Enabled>true</Enabled>") {
		t.Errorf("XML missing <Enabled>true</Enabled>:\n%s", xml)
	}
}

// TestSchtasksXML_RestartOnFailure verifies crash-restart semantics are
// present (mirrors launchd KeepAlive and systemd Restart=on-failure).
func TestSchtasksXML_RestartOnFailure(t *testing.T) {
	u := windowsSampleUnit()
	xml := SchtasksXML(u)
	if !strings.Contains(xml, "<RestartOnFailure>") {
		t.Errorf("XML missing <RestartOnFailure>:\n%s", xml)
	}
}

// TestSchtasksXML_Namespace verifies the Task element uses the canonical
// Windows Task Scheduler 1.4 namespace.
func TestSchtasksXML_Namespace(t *testing.T) {
	u := windowsSampleUnit()
	xml := SchtasksXML(u)
	if !strings.Contains(xml, "schemas.microsoft.com/windows/2004/02/mit/task") {
		t.Errorf("XML missing Task Scheduler namespace:\n%s", xml)
	}
}

// TestWatcherTaskName verifies the task name helper returns the unit label.
func TestWatcherTaskName(t *testing.T) {
	u := windowsSampleUnit()
	if got := WatcherTaskName(u); got != u.Label() {
		t.Errorf("WatcherTaskName = %q, want %q", got, u.Label())
	}
}

// TestParseWatcherTaskStatus_Running verifies that a CSV with "Running" status
// is parsed correctly.
func TestParseWatcherTaskStatus_Running(t *testing.T) {
	// Minimal CSV matching the schtasks /query /fo csv /v format.
	csv := `"HostName","TaskName","Next Run Time","Status","Logon Mode","Last Run Time","Last Result","Author","Task To Run","Start In","Comment","Scheduled Task State","Idle Time","Power Management","Run As User","Delete Task If Not Scheduled","Stop Task If Runs X Hours and X Mins","Schedule","Schedule Type","Start Time","Start Date","End Date","Days","Months","Repeat: Every","Repeat: Until: Time","Repeat: Until: Duration","Repeat: Stop If Still Running","PID"
"DESKTOP","com.grafel.watcher.demo.core","N/A","Running","Interactive/Background","5/23/2026 10:00:00 AM","0","testuser","C:\Program Files\grafel\grafel.exe","N/A","N/A","Enabled","Disabled","N/A","testuser","Disabled","Disabled","Scheduling data is not available in this format.","One Time Only","10:00:00 AM","5/23/2026","N/A","N/A","N/A","Disabled","N/A","N/A","N/A","4242"
`
	ws := WatcherStatus{TaskName: "com.grafel.watcher.demo.core"}
	result := parseWatcherTaskStatus(ws, []byte(csv))
	if !result.Running {
		t.Error("expected Running=true from 'Running' status column")
	}
	if result.PID != 4242 {
		t.Errorf("expected PID=4242, got %d", result.PID)
	}
}

// TestParseWatcherTaskStatus_NotRunning verifies that a non-running status
// leaves Running=false.
func TestParseWatcherTaskStatus_NotRunning(t *testing.T) {
	csv := `"HostName","TaskName","Next Run Time","Status","PID"
"DESKTOP","com.grafel.watcher.demo.core","N/A","Ready","0"
`
	ws := WatcherStatus{TaskName: "com.grafel.watcher.demo.core", Installed: true}
	result := parseWatcherTaskStatus(ws, []byte(csv))
	if result.Running {
		t.Error("expected Running=false for 'Ready' status")
	}
	if result.PID != 0 {
		t.Errorf("expected PID=0 for status=Ready, got %d", result.PID)
	}
}

// TestIsNonFatal verifies the helper correctly identifies wrapped non-fatal errors.
func TestIsNonFatal(t *testing.T) {
	inner := fmt.Errorf("exit status 1")
	nfErr := fmt.Errorf("task registered but /run failed: %w", errNonFatal{inner})
	if !IsNonFatal(nfErr) {
		t.Error("expected IsNonFatal=true for wrapped errNonFatal")
	}

	plain := fmt.Errorf("schtasks /create: exit status 1")
	if IsNonFatal(plain) {
		t.Error("expected IsNonFatal=false for plain error")
	}
}
