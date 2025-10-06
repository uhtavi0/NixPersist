package dockercompose

import (
	"strings"
	"testing"
)

func TestRenderConfig_Basic(t *testing.T) {
	cfg, err := RenderConfig(ConfigParams{
		ServiceName:    "e2etest",
		Image:          "alpine:latest",
		PayloadCommand: "/usr/bin/touch /tmp/persisted",
	})
	if err != nil {
		t.Fatalf("RenderConfig returned error: %v", err)
	}
	mustContain(t, cfg, "version: \"3.9\"")
	mustContain(t, cfg, "services:\n  e2etest:")
	mustContain(t, cfg, "container_name: e2etest")
	mustContain(t, cfg, "image: alpine:latest")
	mustContain(t, cfg, "privileged: true")
	mustContain(t, cfg, "pid: \"host\"")
	mustContain(t, cfg, "volumes:\n      - \"/:/mnt\"")
	mustContain(t, cfg, "command:\n      - /bin/sh\n      - -c\n      - chroot /mnt /usr/bin/touch /tmp/persisted")
	mustContain(t, cfg, "restart: \"always\"")
}

func TestRenderConfig_InvalidInputs(t *testing.T) {
	tests := []ConfigParams{
		{},
		{ServiceName: "bad name", Image: "alpine", PayloadCommand: "/bin/true"},
		{ServiceName: "ok", Image: "", PayloadCommand: "/bin/true"},
		{ServiceName: "ok", Image: "alpine", PayloadCommand: ""},
	}
	for _, tc := range tests {
		if _, err := RenderConfig(tc); err == nil {
			t.Fatalf("expected error for params %#v", tc)
		}
	}
}

func mustContain(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("expected to contain %q\n--- got ---\n%s", substr, s)
	}
}
