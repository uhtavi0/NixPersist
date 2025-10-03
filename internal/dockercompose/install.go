package dockercompose

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// DefaultComposeName is the filename written to the target directory.
const DefaultComposeName = "docker-compose.yml"

var commandRunner = exec.Command

// Install writes the rendered docker-compose configuration to outputDir and
// invokes "docker compose up -d" (or "docker-compose") to start the container.
func Install(cfg string, outputDir string) (string, error) {
	if cfg == "" {
		return "", errors.New("install: configuration content is empty")
	}
	if strings.TrimSpace(outputDir) == "" {
		return "", errors.New("install: output directory is required")
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("install: create output directory %s: %w", outputDir, err)
	}

	dest := filepath.Join(outputDir, DefaultComposeName)
	if err := os.WriteFile(dest, []byte(cfg), 0644); err != nil {
		return "", fmt.Errorf("install: write compose file: %w", err)
	}

	if err := runCompose(dest, "up", "-d"); err != nil {
		return "", fmt.Errorf("install: docker compose up failed: %w", err)
	}

	return dest, nil
}

// Remove stops the deployment via "docker compose down" and deletes the compose file.
func Remove(outputDir string) error {
	if strings.TrimSpace(outputDir) == "" {
		return errors.New("remove: output directory is required")
	}

	dest := filepath.Join(outputDir, DefaultComposeName)
	if _, err := os.Stat(dest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove: compose file %s not found", dest)
		}
		return fmt.Errorf("remove: stat compose file: %w", err)
	}

	if err := runCompose(dest, "down"); err != nil {
		return fmt.Errorf("remove: docker compose down failed: %w", err)
	}

	if err := os.Remove(dest); err != nil {
		return fmt.Errorf("remove: delete compose file: %w", err)
	}

	if err := os.Remove(outputDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		// Keep directory cleanup best-effort; ignore common scenarios where the
		// directory is not empty or we lack permission.
		if !errors.Is(err, os.ErrPermission) && !errors.Is(err, syscall.ENOTEMPTY) {
			return fmt.Errorf("remove: cleanup directory: %w", err)
		}
	}

	return nil
}

func runCompose(composePath string, action string, extraArgs ...string) error {
	absPath, err := filepath.Abs(composePath)
	if err != nil {
		return fmt.Errorf("resolve compose path: %w", err)
	}
	dir := filepath.Dir(absPath)
	fileName := filepath.Base(absPath)

	args := make([]string, 0, 3+len(extraArgs))
	args = append(args, "-f", fileName, action)
	args = append(args, extraArgs...)

	run := func(binary string, args []string) error {
		cmd := commandRunner(binary, args...)
		cmd.Dir = dir
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w: %s", binary, err, strings.TrimSpace(string(output)))
		}
		return nil
	}

	var errs []string

	if _, err := exec.LookPath("docker"); err == nil {
		cmdArgs := append([]string{"compose"}, args...)
		if err := run("docker", cmdArgs); err == nil {
			return nil
		} else {
			errs = append(errs, err.Error())
		}
	}

	if _, err := exec.LookPath("docker-compose"); err == nil {
		if err := run("docker-compose", args); err == nil {
			return nil
		} else {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) == 0 {
		return errors.New("docker compose command not found (tried 'docker compose' and 'docker-compose')")
	}

	return errors.New(strings.Join(errs, "; "))
}
