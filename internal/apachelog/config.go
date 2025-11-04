package apachelog

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// DefaultConfPath is the typical Apache configuration file on Debian/Ubuntu.
	DefaultConfPath = "/etc/apache2/apache2.conf"

	logFormat = "error"
)

// ConfigParams captures the inputs required to render the Apache CustomLog
// directive that invokes an external payload.
type ConfigParams struct {
	// Payload is the absolute path to the executable that Apache should invoke.
	Payload string
}

// Validate enforces the constraints required to safely render the configuration.
func (p ConfigParams) Validate() error {
	payload := strings.TrimSpace(p.Payload)
	if payload == "" {
		return errors.New("payload is required")
	}
	if strings.Contains(payload, "\n") {
		return errors.New("payload must not contain newlines")
	}
	if strings.ContainsAny(payload, "\"<>") {
		return errors.New("payload must not contain quotes or angle brackets")
	}
	if !strings.HasPrefix(payload, "/") {
		return errors.New("payload must be an absolute path")
	}
	return nil
}

// RenderConfig produces the Apache configuration snippet that pipes logs to the
// specified payload.
func RenderConfig(p ConfigParams) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}

	return fmt.Sprintf("CustomLog \"|%s\" %s\n", strings.TrimSpace(p.Payload), logFormat), nil
}
