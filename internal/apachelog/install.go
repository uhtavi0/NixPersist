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

	if bytes.Contains(original, []byte(startMarker)) {
		return errors.New("apache-log snippet already present in configuration")
	}

	var buf bytes.Buffer
	buf.Write(original)
	if buf.Len() > 0 && !bytes.HasSuffix(buf.Bytes(), []byte("\n")) {
		buf.WriteByte('\n')
	}
	if buf.Len() > 0 {
		buf.WriteByte('\n')
	}
	buf.WriteString(cfg)

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

	content := string(original)
	start := strings.Index(content, startMarker)
	if start == -1 {
		return errors.New("apache-log snippet not found in configuration")
	}
	end := strings.Index(content[start:], endMarker)
	if end == -1 {
		return errors.New("apache-log snippet end marker missing")
	}
	end += start + len(endMarker)

	trimStart := start
	if trimStart > 0 {
		i := trimStart - 1
		for i >= 0 && (content[i] == ' ' || content[i] == '\t') {
			i--
		}
		if i >= 0 && content[i] == '\n' {
			trimStart = i
			if i > 0 && content[i-1] == '\n' {
				trimStart = i - 1
			}
		}
	}

	trimEnd := end
	for trimEnd < len(content) && (content[trimEnd] == '\n' || content[trimEnd] == '\r') {
		trimEnd++
	}

	var buf bytes.Buffer
	buf.WriteString(content[:trimStart])
	if trimEnd < len(content) {
		buf.WriteByte('\n')
		buf.WriteString(strings.TrimLeft(content[trimEnd:], "\n"))
	}
	result := buf.Bytes()

	// Ensure file ends with newline if original did.
	if len(result) > 0 && result[len(result)-1] != '\n' {
		result = append(result, '\n')
	}

	if err := os.WriteFile(confPath, result, mode); err != nil {
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
