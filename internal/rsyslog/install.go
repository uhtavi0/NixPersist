package rsyslog

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	// DefaultConfigDir is where rsyslog loads drop-in configs.
	DefaultConfigDir  = "/etc/rsyslog.d"
	DefaultConfigName = "99-gopersist.conf"
)

// Install writes the provided rsyslog configuration to /etc/rsyslog.d and
// reloads (or restarts) rsyslog.
// Requires root privileges.
func Install(cfg string) error {
	if os.Geteuid() != 0 {
		return errors.New("installer: root privileges required (run with sudo)")
	}
	if cfg == "" {
		return errors.New("installer: empty configuration provided")
	}

	// Ensure target directory exists
	if err := os.MkdirAll(DefaultConfigDir, 0755); err != nil {
		return fmt.Errorf("create %s: %w", DefaultConfigDir, err)
	}

	dest := filepath.Join(DefaultConfigDir, DefaultConfigName)
	if err := os.WriteFile(dest, []byte(cfg), 0644); err != nil {
		return fmt.Errorf("write config to %s: %w", dest, err)
	}

	if err := reloadRsyslog(); err != nil {
		return fmt.Errorf("reload rsyslog: %w", err)
	}

	return nil
}

// Remove deletes the gopersist drop-in configuration and reloads rsyslog.
// Requires root privileges.
func Remove() error {
	if os.Geteuid() != 0 {
		return errors.New("remove: root privileges required (run with sudo)")
	}

	dest := filepath.Join(DefaultConfigDir, DefaultConfigName)
	if _, err := os.Stat(dest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove: %s not present", dest)
		}
		return fmt.Errorf("remove: stat %s: %w", dest, err)
	}

	if err := os.Remove(dest); err != nil {
		return fmt.Errorf("remove: delete %s: %w", dest, err)
	}

	if err := reloadRsyslog(); err != nil {
		return fmt.Errorf("reload rsyslog after removal: %w", err)
	}

	return nil
}

func reloadRsyslog() error {
	// Reload or restart rsyslog to apply changes
	if hasSystemctl() {
		if _, err := exec.Command("systemctl", "reload", "rsyslog").CombinedOutput(); err == nil {
			return nil
		}
		if out, err := exec.Command("systemctl", "restart", "rsyslog").CombinedOutput(); err != nil {
			return fmt.Errorf("failed to reload or restart rsyslog via systemctl: %w; output: %s", err, string(out))
		}
		return nil
	}

	// Fallback to SysV-style service command if present
	if _, err := exec.LookPath("service"); err == nil {
		if _, err := exec.Command("service", "rsyslog", "reload").CombinedOutput(); err == nil {
			return nil
		}
		if out, err := exec.Command("service", "rsyslog", "restart").CombinedOutput(); err != nil {
			return fmt.Errorf("failed to reload or restart rsyslog via service: %w; output: %s", err, string(out))
		}
		return nil
	}

	return errors.New("could not find a method to reload rsyslog (systemctl or service not available)")
}
