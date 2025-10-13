package rsyslog

import (
	"strings"
	"testing"
)

func TestRenderShellConfig(t *testing.T) {
	cfg, err := RenderShellConfig(ShellConfigParams{
		Trigger: "hacker",
		Payload: "/path/to/payload",
	})
	if err != nil {
		t.Fatalf("RenderShellConfig returned error: %v", err)
	}
	if !containsAll(cfg, shellMarkerStart, shellMarkerEnd, ":msg, contains, \"hacker\" ^/path/to/payload") {
		t.Fatalf("unexpected config contents:\n%s", cfg)
	}
}

func TestRemoveShellBlock(t *testing.T) {
	snippet, err := RenderShellConfig(ShellConfigParams{Trigger: "foo", Payload: "/bin/true"})
	if err != nil {
		t.Fatalf("RenderShellConfig returned error: %v", err)
	}
	original := "line1\n" + snippet + "line2\n"
	data, ok := removeShellBlock([]byte(original))
	if !ok {
		t.Fatal("expected snippet to be found")
	}
	expected := "line1\nline2\n"
	if string(data) != expected {
		t.Fatalf("unexpected result: got %q, want %q", string(data), expected)
	}

	_, ok = removeShellBlock([]byte("nothing here"))
	if ok {
		t.Fatal("expected snippet to be absent")
	}
}

func containsAll(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
