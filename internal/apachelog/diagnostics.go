package apachelog

import (
	"fmt"
	"os"
	"strings"
)

// Result captures diagnostic data about the Apache environment.
type Result struct {
	ConfigPath         string
	ConfigExists       bool
	ConfigWritable     bool
	RunningAsRoot      bool
	SystemctlAvailable bool
	ApacheCtlAvailable bool
	ServiceActive      bool
	Notes              []string
}

// HasAccess reports whether Apache is likely manageable with the current privileges.
func (r Result) HasAccess() bool {
	return r.ConfigExists && (r.ConfigWritable || r.RunningAsRoot) && r.SystemctlAvailable
}

// Render formats the diagnostic information in a human-readable form.
func (r Result) Render() string {
	var b strings.Builder
	writeLine := func(label string, ok bool) {
		status := "NO"
		if ok {
			status = "YES"
		}
		fmt.Fprintf(&b, "- %s: %s\n", label, status)
	}
	writeLine(fmt.Sprintf("config present (%s)", r.ConfigPath), r.ConfigExists)
	writeLine("config writable", r.ConfigWritable)
	writeLine("running as root", r.RunningAsRoot)
	writeLine("systemctl available", r.SystemctlAvailable)
	writeLine("apachectl/apache2ctl available", r.ApacheCtlAvailable)
	writeLine("apache2 service active", r.ServiceActive)

	if len(r.Notes) > 0 {
		b.WriteString("\nNotes:\n")
		for _, note := range r.Notes {
			fmt.Fprintf(&b, "- %s\n", note)
		}
	}

	return b.String()
}

// Check inspects the local system to determine whether Apache log piping can be installed.
func Check(confPath string) Result {
	if strings.TrimSpace(confPath) == "" {
		confPath = DefaultConfPath
	}

	var r Result
	r.ConfigPath = confPath
	r.RunningAsRoot = os.Geteuid() == 0
	if !r.RunningAsRoot {
		r.Notes = append(r.Notes, "not running as root; writes to apache2.conf may fail")
	}

	if _, err := os.Stat(confPath); err == nil {
		r.ConfigExists = true
		if fileWritable(confPath) {
			r.ConfigWritable = true
		} else {
			r.Notes = append(r.Notes, fmt.Sprintf("cannot open %s for write; root privileges required", confPath))
		}
	} else if os.IsNotExist(err) {
		r.Notes = append(r.Notes, fmt.Sprintf("configuration %s does not exist", confPath))
	} else {
		r.Notes = append(r.Notes, fmt.Sprintf("failed to stat %s: %v", confPath, err))
	}

	if _, err := lookPath("systemctl"); err == nil {
		r.SystemctlAvailable = true
		cmd := execCommand("systemctl", "is-active", serviceName)
		output, err := cmd.CombinedOutput()
		if err == nil {
			r.ServiceActive = strings.TrimSpace(string(output)) == "active"
		} else {
			r.Notes = append(r.Notes, fmt.Sprintf("systemctl is-active %s failed: %v", serviceName, err))
		}
	} else {
		r.Notes = append(r.Notes, "systemctl binary not found; manual service restart required")
	}

	if _, err := lookPath("apache2ctl"); err == nil {
		r.ApacheCtlAvailable = true
	} else if _, err := lookPath("apachectl"); err == nil {
		r.ApacheCtlAvailable = true
	} else {
		r.Notes = append(r.Notes, "apachectl/apache2ctl not found on PATH")
	}

	return r
}

func fileWritable(path string) bool {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f.Close()
	return true
}
