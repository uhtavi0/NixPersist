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
	line := fmt.Sprintf(`:msg, contains, "%s" ^%s`, escapedTrigger, strings.TrimSpace(p.Payload))
	return line + "\n", nil
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
	if hasShellDirective(existing, strings.TrimSpace(cfg)) {
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

	updated, found := removeShellDirective(data)
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

func hasShellDirective(content []byte, directive string) bool {
	for _, line := range strings.Split(string(content), "\n") {
		if strings.TrimSpace(line) == directive {
			return true
		}
	}
	return false
}

func isShellDirective(line string) bool {
	line = strings.TrimSpace(line)
	const prefix = `:msg, contains, "`
	if !strings.HasPrefix(line, prefix) {
		return false
	}

	rest := line[len(prefix):]
	quoteIdx := strings.Index(rest, `"`)
	if quoteIdx == -1 {
		return false
	}

	action := strings.TrimSpace(rest[quoteIdx+1:])
	if !strings.HasPrefix(action, "^") {
		return false
	}

	cmd := strings.TrimSpace(action[1:])
	return cmd != ""
}

func removeShellDirective(content []byte) ([]byte, bool) {
	lines := strings.Split(string(content), "\n")
	var result []string
	removed := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !removed && isShellDirective(line) {
			removed = true
			if i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "" {
				i++
			}
			continue
		}
		result = append(result, line)
	}

	if !removed {
		return content, false
	}

	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}

	if len(result) == 0 {
		return []byte{}, true
	}

	return []byte(strings.Join(result, "\n") + "\n"), true
}
