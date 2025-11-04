package apachelog

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallAndRemove(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "apache2.conf")
	if err := os.WriteFile(conf, []byte("ServerName localhost\n"), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	if err := Install(ConfigParams{Payload: "/usr/bin/testsh"}, conf, false); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}

	data, err := os.ReadFile(conf)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	content := string(data)
	if strings.Count(content, startMarker) != 1 {
		t.Fatalf("expected one start marker, got %d\n%s", strings.Count(content, startMarker), content)
	}
	if !strings.Contains(content, `CustomLog "|/usr/bin/testsh" error`) {
		t.Fatalf("expected CustomLog directive, got\n%s", content)
	}

	if err := Remove(conf, false); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	final, err := os.ReadFile(conf)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(final) != "ServerName localhost\n" {
		t.Fatalf("unexpected final config:\n%q", string(final))
	}
}

func TestInstallDuplicateFails(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "apache2.conf")
	if err := os.WriteFile(conf, []byte("ServerRoot /etc/apache2\n"), 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	if err := Install(ConfigParams{Payload: "/usr/bin/testsh"}, conf, false); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}

	if err := Install(ConfigParams{Payload: "/usr/bin/testsh"}, conf, false); err == nil {
		t.Fatalf("expected duplicate install to fail")
	}
}

func TestInstallRestartInvokesSystemctl(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "apache2.conf")
	if err := os.WriteFile(conf, []byte{}, 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	var called []string
	origLookPath := lookPath
	origExec := execCommand
	lookPath = func(string) (string, error) {
		return "/bin/systemctl", nil
	}
	execCommand = func(name string, args ...string) *exec.Cmd {
		called = append(called, name+" "+strings.Join(args, " "))
		return exec.Command("true")
	}
	defer func() {
		lookPath = origLookPath
		execCommand = origExec
	}()

	if err := Install(ConfigParams{Payload: "/usr/bin/testsh"}, conf, true); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}

	if len(called) != 1 {
		t.Fatalf("expected systemctl to be called once, got %v", called)
	}
	if called[0] != "systemctl restart apache2" {
		t.Fatalf("unexpected command: %s", called[0])
	}
}

func TestRemoveRestartError(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "apache2.conf")
	if err := os.WriteFile(conf, []byte{}, 0644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	if err := Install(ConfigParams{Payload: "/usr/bin/testsh"}, conf, false); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}

	origLookPath := lookPath
	origExec := execCommand
	lookPath = func(string) (string, error) {
		return "/bin/systemctl", nil
	}
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}
	defer func() {
		lookPath = origLookPath
		execCommand = origExec
	}()

	if err := Remove(conf, true); err == nil {
		t.Fatalf("expected restart failure to bubble up")
	}
}
