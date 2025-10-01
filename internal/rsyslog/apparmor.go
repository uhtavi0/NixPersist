package rsyslog

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// DisableRsyslogProfile disables AppArmor's rsyslog profile (Ubuntu/Debian paths)
// to permit omprog execution. Requires root and is destructive until reboot or
// re-enabling; callers should present a clear confirmation gate and provide a
// corresponding re-enable path.
//
// Equivalent shell (from PoC.md):
//
//	apparmor_parser -R /etc/apparmor.d/usr.sbin.rsyslogd
//	ln -sf /etc/apparmor.d/usr.sbin.rsyslogd /etc/apparmor.d/disable/
func DisableRsyslogProfile() error {
	if _, err := exec.LookPath("apparmor_parser"); err != nil {
		return errors.New("apparmor_parser not found; is AppArmor installed?")
	}
	if err := exec.Command("apparmor_parser", "-R", "/etc/apparmor.d/usr.sbin.rsyslogd").Run(); err != nil {
		return fmt.Errorf("failed to remove rsyslog AppArmor profile: %w", err)
	}
	if err := exec.Command("ln", "-sf", "/etc/apparmor.d/usr.sbin.rsyslogd", "/etc/apparmor.d/disable/").Run(); err != nil {
		return fmt.Errorf("failed to place profile in disable/: %w", err)
	}
	return nil
}

// EnableRsyslogProfile restores the rsyslog AppArmor profile after a disable.
// Requires root privileges.
func EnableRsyslogProfile() error {
	if _, err := exec.LookPath("apparmor_parser"); err != nil {
		return errors.New("apparmor_parser not found; is AppArmor installed?")
	}
	if err := os.Remove("/etc/apparmor.d/disable/usr.sbin.rsyslogd"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove disable symlink: %w", err)
	}
	if err := exec.Command("apparmor_parser", "-r", "/etc/apparmor.d/usr.sbin.rsyslogd").Run(); err != nil {
		return fmt.Errorf("failed to re-load rsyslog AppArmor profile: %w", err)
	}
	return nil
}
