package rsyslog

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultShellConfigPath is the canonical rsyslog configuration file.
	DefaultShellConfigPath = "/etc/rsyslog.conf"

	shellMarkerStart = "# BEGIN NixPersist shell execute"
	shellMarkerEnd   = "# END NixPersist shell execute"
)

// ShellConfigParams drives rendering for the shell-exec rsyslog filter.
type ShellConfigParams struct {
	Trigger string
	Payload string
}

// Validate ensures mandatory parameters are set.
func (p ShellConfigParams) Validate() error {
	if strings.TrimSpace(p.Trigger) == "" {
		return errors.New("Trigger is required")
	}
	if strings.TrimSpace(p.Payload) == "" {
		return errors.New("Payload is required")
	}
	if strings.Contains(p.Trigger, "\n") {
		return errors.New("Trigger must not contain newlines")
	}
	if strings.Contains(p.Payload, "\n") {
		return errors.New("Payload must not contain newlines")
	}
	return nil
}

// RenderShellConfig returns the snippet appended to rsyslog.conf for shell execution.
func RenderShellConfig(p ShellConfigParams) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}

	escapedTrigger := strings.ReplaceAll(p.Trigger, "\"", `\\"`)
	lines := []string{
		shellMarkerStart,
		fmt.Sprintf(`:msg, contains, "%s" ^%s`, escapedTrigger, strings.TrimSpace(p.Payload)),
		shellMarkerEnd,
		"",
	}
	return strings.Join(lines, "\n"), nil
}

// InstallShell appends the rendered shell configuration to the given file.
// Requires root privileges as it modifies system rsyslog configuration.
func InstallShell(cfg, dest string) error {
	if os.Geteuid() != 0 {
		return errors.New("installer: root privileges required (run with sudo)")
	}
	if cfg == "" {
		return errors.New("installer: empty configuration provided")
	}
	if dest == "" {
		dest = DefaultShellConfigPath
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(dest), err)
	}

	existing, err := os.ReadFile(dest)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", dest, err)
	}
	if bytes.Contains(existing, []byte(shellMarkerStart)) {
		return fmt.Errorf("rsyslog shell snippet already present in %s", dest)
	}

	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open %s: %w", dest, err)
	}
	defer f.Close()

	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
		if _, err := f.Write([]byte("\n")); err != nil {
			return fmt.Errorf("append newline to %s: %w", dest, err)
		}
	}

	if _, err := f.WriteString(cfg); err != nil {
		return fmt.Errorf("append shell config to %s: %w", dest, err)
	}

	if err := reloadRsyslog(); err != nil {
		return fmt.Errorf("reload rsyslog: %w", err)
	}

	return nil
}

// RemoveShell deletes the NixPersist shell snippet from the given file and reloads rsyslog.
func RemoveShell(dest string) error {
	if os.Geteuid() != 0 {
		return errors.New("remove: root privileges required (run with sudo)")
	}
	if dest == "" {
		dest = DefaultShellConfigPath
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		return fmt.Errorf("read %s: %w", dest, err)
	}

	updated, found := removeShellBlock(data)
	if !found {
		return fmt.Errorf("rsyslog shell snippet not found in %s", dest)
	}

	if err := os.WriteFile(dest, updated, 0644); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}

	if err := reloadRsyslog(); err != nil {
		return fmt.Errorf("reload rsyslog after removal: %w", err)
	}

	return nil
}

func removeShellBlock(content []byte) ([]byte, bool) {
	start := bytes.Index(content, []byte(shellMarkerStart))
	if start == -1 {
		return content, false
	}
	end := bytes.Index(content[start:], []byte(shellMarkerEnd))
	if end == -1 {
		return content, false
	}
	end += start + len(shellMarkerEnd)

	for end < len(content) && (content[end] == '\r' || content[end] == '\n') {
		end++
	}

	cutStart := start
	if cutStart > 0 && content[cutStart-1] == '\n' {
		prev := cutStart - 1
		if prev > 0 && content[prev-1] == '\r' {
			prev--
		}
		if prev == 0 || content[prev-1] == '\n' {
			cutStart = prev
		}
	}

	updated := append([]byte{}, content[:cutStart]...)
	updated = append(updated, content[end:]...)

	updated = bytes.TrimRight(updated, "\r\n")
	if len(updated) > 0 {
		updated = append(updated, '\n')
	}

	return updated, true
}
