package rsyslog

import (
	"strings"
	"testing"
)

// Original PoC behavior: no ruleset when not specified.
func TestRenderConfig_PoCStyle_NoRuleset(t *testing.T) {
	cfg, err := RenderConfig(ConfigParams{
		InputFile:       "/path/to/access.log",
		Tag:             "access",
		Severity:        "info",
		Facility:        "local6",
		AddMetadata:     true,
		PollingInterval: 10,
		FilterByTag:     true,
		FilterContains:  "Chrome/133.7.0.0",
		ProgramPath:     "/bin/echo",
		ProgramArgs:     "hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, cfg, "module(load=\"imfile\" PollingInterval=\"10\")")
	mustContain(t, cfg, "module(load=\"omprog\")")
	mustContain(t, cfg, "input(\n\ttype=\"imfile\"\n\tFile=\"/path/to/access.log\"\n\tTag=\"access\"\n\tSeverity=\"info\"\n\tFacility=\"local6\"\n\taddMetadata=\"on\"\n\treopenOnTruncate=\"on\"\n)")
	mustContain(t, cfg, "if ($syslogtag contains 'access') and ($msg contains 'Chrome/133.7.0.0') then {")
	mustContain(t, cfg, "action(type=\"omprog\" binary=\"/bin/echo hello\")")
}

// Additional test: explicit ruleset usage.
func TestRenderConfig_WithRuleset(t *testing.T) {
	cfg, err := RenderConfig(ConfigParams{
		InputFile:       "/path/to/access.log",
		Tag:             "access",
		Severity:        "info",
		Facility:        "local6",
		AddMetadata:     true,
		PollingInterval: 10,
		FilterByTag:     true,
		FilterContains:  "Chrome/133.7.0.0",
		ProgramPath:     "/bin/echo",
		ProgramArgs:     "hello",
		UseRuleset:      true,
		RulesetName:     "gopersist",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mustContain(t, cfg, "module(load=\"imfile\" PollingInterval=\"10\")")
	mustContain(t, cfg, "module(load=\"omprog\")")
	mustContain(t, cfg, "input(\n\ttype=\"imfile\"\n\tFile=\"/path/to/access.log\"\n\tTag=\"access\"\n\tSeverity=\"info\"\n\tFacility=\"local6\"\n\taddMetadata=\"on\"\n\treopenOnTruncate=\"on\"\n\truleset=\"gopersist\"\n)")
	mustContain(t, cfg, "ruleset(name=\"gopersist\") {")
	mustContain(t, cfg, "if ($syslogtag contains 'access') and ($msg contains 'Chrome/133.7.0.0') then {")
	mustContain(t, cfg, "action(type=\"omprog\" binary=\"/bin/echo hello\")")
}

func TestRenderConfig_Errors(t *testing.T) {
	_, err := RenderConfig(ConfigParams{})
	if err == nil {
		t.Fatalf("expected error for missing fields")
	}
}

func mustContain(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("expected to contain %q\n--- got ---\n%s", substr, s)
	}
}
