package apachelog

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCheck_WithDependencies(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "apache2.conf")
	if err := os.WriteFile(conf, []byte("# test config\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	origLookPath := lookPath
	origExec := execCommand
	defer func() {
		lookPath = origLookPath
		execCommand = origExec
	}()

	lookPath = func(name string) (string, error) {
		switch name {
		case "systemctl":
			return "/bin/systemctl", nil
		case "apache2ctl":
			return "/usr/sbin/apache2ctl", nil
		default:
			return "", os.ErrNotExist
		}
	}
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "systemctl" && len(args) >= 2 && args[0] == "is-active" {
			return exec.Command("printf", "active")
		}
		return exec.Command("false")
	}

	res := Check(conf)

	if !res.ConfigExists || !res.ConfigWritable {
		t.Fatalf("expected config to exist and be writable, got %+v", res)
	}
	if !res.SystemctlAvailable || !res.ApacheCtlAvailable {
		t.Fatalf("expected systemctl and apachectl availability, got %+v", res)
	}
	if !res.ServiceActive {
		t.Fatalf("expected service to be active, got %+v", res)
	}
}

func TestCheck_MissingConfig(t *testing.T) {
	origLookPath := lookPath
	defer func() {
		lookPath = origLookPath
	}()
	lookPath = func(name string) (string, error) {
		return "", os.ErrNotExist
	}

	res := Check("/nonexistent/apache2.conf")
	if res.ConfigExists {
		t.Fatalf("expected missing config, got %+v", res)
	}
	if res.SystemctlAvailable {
		t.Fatalf("expected systemctl unavailable, got %+v", res)
	}
	if len(res.Notes) == 0 {
		t.Fatalf("expected notes to mention missing config")
	}
}
