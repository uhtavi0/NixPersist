package apachelog

import "testing"

func TestRenderConfig(t *testing.T) {
	cfg, err := RenderConfig(ConfigParams{Payload: "/usr/bin/apachesh"})
	if err != nil {
		t.Fatalf("RenderConfig returned error: %v", err)
	}
	want := "CustomLog \"|/usr/bin/apachesh\" error\n"
	if cfg != want {
		t.Fatalf("expected config to equal %q\n--- got ---\n%s", want, cfg)
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
