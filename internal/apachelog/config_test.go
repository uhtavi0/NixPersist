package apachelog

import "testing"

func TestRenderConfig(t *testing.T) {
	cfg, err := RenderConfig(ConfigParams{Payload: "/usr/bin/apachesh"})
	if err != nil {
		t.Fatalf("RenderConfig returned error: %v", err)
	}
	wantLines := []string{
		startMarker,
		`CustomLog "|/usr/bin/apachesh" error`,
		endMarker,
	}
	for _, line := range wantLines {
		if !containsLine(cfg, line) {
			t.Fatalf("expected config to contain %q\n--- got ---\n%s", line, cfg)
		}
	}
}

func TestRenderConfig_InvalidInputs(t *testing.T) {
	tests := []ConfigParams{
		{Payload: ""},
		{Payload: "relative/path"},
		{Payload: "/bin/sh\n/tmp/payload"},
		{Payload: "/path/with\"quote"},
		{Payload: "/path/with<angle>"},
	}
	for _, tc := range tests {
		if _, err := RenderConfig(tc); err == nil {
			t.Fatalf("expected error for payload %q", tc.Payload)
		}
	}
}

func containsLine(s, line string) bool {
	for _, l := range splitLines(s) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, r := range s {
		if r == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start <= len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
