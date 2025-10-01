package rsyslog

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Result captures feasibility checks for using gopersist on a host.
type Result struct {
	RsyslogInstalled         bool
	RsyslogRunning           bool
	AppArmorInstalled        bool
	RsyslogAppArmorProtected bool

	Notes []string
}

// Check performs environment checks and returns a Result.
func Check() Result {
	var r Result

	r.RsyslogInstalled = checkRsyslogInstalled(&r)
	r.RsyslogRunning = checkRsyslogRunning(&r)
	r.AppArmorInstalled = checkAppArmorInstalled(&r)
	if r.AppArmorInstalled && r.RsyslogRunning {
		r.RsyslogAppArmorProtected = checkRsyslogAppArmorProtected(&r)
	}

	return r
}

func checkRsyslogInstalled(r *Result) bool {
	if _, err := exec.LookPath("rsyslogd"); err == nil {
		r.Notes = append(r.Notes, "found rsyslogd in PATH")
		return true
	}
	if exists("/etc/rsyslog.conf") {
		r.Notes = append(r.Notes, "found /etc/rsyslog.conf")
		return true
	}
	// Try systemd presence of unit
	if hasSystemctl() {
		if out, err := exec.Command("systemctl", "status", "rsyslog.service").CombinedOutput(); err == nil || len(out) > 0 {
			if bytes.Contains(out, []byte("Loaded: loaded")) || bytes.Contains(out, []byte("rsyslog.service")) {
				r.Notes = append(r.Notes, "systemd reports rsyslog.service present")
				return true
			}
		}
	}
	return false
}

func checkRsyslogRunning(r *Result) bool {
	if hasSystemctl() {
		out, err := exec.Command("systemctl", "is-active", "rsyslog.service").CombinedOutput()
		if err == nil && strings.TrimSpace(string(out)) == "active" {
			r.Notes = append(r.Notes, "rsyslog.service is active (systemd)")
			return true
		}
	}
	// Fallback: look for process
	if _, err := exec.LookPath("pgrep"); err == nil {
		if out, err := exec.Command("pgrep", "-x", "rsyslogd").CombinedOutput(); err == nil && len(bytes.TrimSpace(out)) > 0 {
			r.Notes = append(r.Notes, "rsyslogd process found via pgrep")
			return true
		}
	}
	return false
}

func checkAppArmorInstalled(r *Result) bool {
	// Common signals: apparmor_status binary, or sysfs entries
	if _, err := exec.LookPath("apparmor_status"); err == nil {
		r.Notes = append(r.Notes, "found apparmor_status in PATH")
		return true
	}
	if _, err := exec.LookPath("apparmor_parser"); err == nil {
		r.Notes = append(r.Notes, "found apparmor_parser in PATH")
		return true
	}
	if exists("/sys/kernel/security/apparmor/profiles") || exists("/sys/module/apparmor/parameters/enabled") {
		r.Notes = append(r.Notes, "AppArmor sysfs entries present")
		return true
	}
	return false
}

func checkRsyslogAppArmorProtected(r *Result) bool {
	// Try reading label from rsyslogd process if possible
	if pid := firstPID("rsyslogd"); pid != "" {
		path := filepath.Join("/proc", pid, "attr", "current")
		if data, err := os.ReadFile(path); err == nil {
			label := strings.TrimSpace(string(data))
			if label != "" && label != "unconfined" {
				r.Notes = append(r.Notes, fmt.Sprintf("rsyslogd confined by AppArmor label: %s", label))
				return true
			}
			if label == "unconfined" {
				r.Notes = append(r.Notes, "rsyslogd is unconfined (AppArmor)")
				return false
			}
		}
	}

	// Fallback: parse apparmor_status output
	if _, err := exec.LookPath("apparmor_status"); err == nil {
		if out, err := exec.Command("apparmor_status").CombinedOutput(); err == nil {
			text := string(out)
			if strings.Contains(text, "rsyslogd (enforce)") || strings.Contains(text, "rsyslogd") && strings.Contains(text, "profiles are in enforce mode") {
				r.Notes = append(r.Notes, "apparmor_status lists rsyslogd in enforce mode")
				return true
			}
			if strings.Contains(text, "rsyslogd (complain)") {
				r.Notes = append(r.Notes, "apparmor_status lists rsyslogd in complain mode")
				return true // still considered protected, though permissive
			}
		}
	}

	return false
}

func hasSystemctl() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// firstPID returns the first PID (as string) for a process name using pgrep.
func firstPID(name string) string {
	if _, err := exec.LookPath("pgrep"); err != nil {
		return ""
	}
	out, err := exec.Command("pgrep", "-x", name).CombinedOutput()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// Render returns a human-readable summary of the checks.
func (r Result) Render() string {
	b := &strings.Builder{}
	writeLine := func(label string, ok bool) {
		status := "NO"
		if ok {
			status = "YES"
		}
		fmt.Fprintf(b, "- %s: %s\n", label, status)
	}
	writeLine("rsyslog installed", r.RsyslogInstalled)
	writeLine("rsyslog running", r.RsyslogRunning)
	writeLine("AppArmor installed", r.AppArmorInstalled)
	writeLine("AppArmor enforced for rsyslog", r.RsyslogAppArmorProtected)

	if len(r.Notes) > 0 {
		b.WriteString("\nNotes:\n")
		for _, n := range r.Notes {
			fmt.Fprintf(b, "- %s\n", n)
		}
	}
	return b.String()
}
