package apachelog

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const serviceName = "apache2"

var (
	execCommand = exec.Command
	lookPath    = exec.LookPath
)

// Install appends the rendered Apache log pipe configuration to the provided
// configuration file. When restart is true, systemctl restart apache2 is invoked.
func Install(params ConfigParams, confPath string, restart bool) error {
	if strings.TrimSpace(confPath) == "" {
		confPath = DefaultConfPath
	}

	cfg, err := RenderConfig(params)
	if err != nil {
		return err
	}

	original, mode, err := readConfig(confPath)
	if err != nil {
		return err
	}

	if containsCustomLogDirective(string(original), strings.TrimSpace(cfg)) {
		return errors.New("apache-log snippet already present in configuration")
	}

	var buf bytes.Buffer
	buf.Write(original)
	if buf.Len() > 0 && !bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
		buf.WriteByte('\n')
	}
	buf.WriteString(cfg)
	if !strings.HasSuffix(cfg, "\n") {
		buf.WriteByte('\n')
	}

	if err := os.WriteFile(confPath, buf.Bytes(), mode); err != nil {
		return fmt.Errorf("write apache configuration: %w", err)
	}

	if restart {
		if err := restartApache(); err != nil {
			return err
		}
	}

	return nil
}

// Remove deletes the NixPersist Apache snippet from confPath. When restart is
// true, systemctl restart apache2 is invoked.
func Remove(confPath string, restart bool) error {
	if strings.TrimSpace(confPath) == "" {
		confPath = DefaultConfPath
	}

	original, mode, err := readConfig(confPath)
	if err != nil {
		return err
	}

	content, found := removeCustomLogDirective(string(original))
	if !found {
		return errors.New("apache-log snippet not found in configuration")
	}
	if err := os.WriteFile(confPath, []byte(content), mode); err != nil {
		return fmt.Errorf("write apache configuration: %w", err)
	}

	if restart {
		if err := restartApache(); err != nil {
			return err
		}
	}

	return nil
}

func readConfig(path string) ([]byte, os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, fmt.Errorf("apache configuration %s does not exist", path)
		}
		return nil, 0, fmt.Errorf("stat configuration %s: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("read configuration %s: %w", path, err)
	}
	return data, info.Mode(), nil
}

func restartApache() error {
	if _, err := lookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl not available: %w", err)
	}
	cmd := execCommand("systemctl", "restart", serviceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl restart %s: %w: %s", serviceName, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func containsCustomLogDirective(content, directive string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == directive {
			return true
		}
	}
	return false
}

func isCustomLogDirective(line string) bool {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, `CustomLog "|`) {
		return false
	}
	if !strings.HasSuffix(line, " "+logFormat) {
		return false
	}
	return true
}

func removeCustomLogDirective(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	var result []string
	removed := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !removed && isCustomLogDirective(line) {
			removed = true
			// Skip immediate blank line after the directive if present.
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

	// Trim trailing blank lines to keep file tidy.
	for len(result) > 0 && strings.TrimSpace(result[len(result)-1]) == "" {
		result = result[:len(result)-1]
	}

	joined := strings.Join(result, "\n")
	if joined != "" {
		joined += "\n"
	}
	return joined, true
}
