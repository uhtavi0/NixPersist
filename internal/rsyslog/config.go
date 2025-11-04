package rsyslog

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

// ConfigParams captures the inputs for rendering an rsyslog configuration
// that tails a file via imfile and triggers a program via omprog.
type ConfigParams struct {
	// InputFile is the path to the log file to monitor.
	InputFile string
	// Tag is the tag assigned to messages from InputFile.
	Tag string
	// Severity optionally sets the syslog severity for the input.
	Severity string
	// Facility optionally sets the syslog facility for the input.
	Facility string
	// AddMetadata toggles addMetadata on the input (default true in PoC).
	AddMetadata bool
	// PollingInterval seconds for imfile module (0 to omit; PoC uses 10).
	PollingInterval int
	// RulesetName is optional; when set and UseRuleset=true, wrap logic in a ruleset.
	RulesetName string
	// UseRuleset controls whether to emit a ruleset wrapper and bind the input to it.
	UseRuleset bool
	// StateFile optionally sets a custom state file name for imfile.
	StateFile string

	// FilterContains when set, triggers if $msg contains this substring.
	FilterContains string
	// FilterRegex when set, triggers if $msg matches this regex.
	FilterRegex string

	// FilterByTag when true, include "$syslogtag contains Tag" in the condition.
	FilterByTag bool

	// ProgramPath is the payload to execute via omprog.
	ProgramPath string
	// ProgramArgs optional arguments for the payload.
	ProgramArgs string
}

// Validate checks required fields.
func (p ConfigParams) Validate() error {
	if p.InputFile == "" {
		return errors.New("InputFile is required")
	}
	if p.ProgramPath == "" {
		return errors.New("ProgramPath is required")
	}
	if p.Tag == "" {
		return errors.New("tag is required")
	}
	// RulesetName is only required if UseRuleset is true
	if p.UseRuleset && p.RulesetName == "" {
		return errors.New("RulesetName is required when UseRuleset is true")
	}
	if p.FilterContains == "" && p.FilterRegex == "" {
		return errors.New("at least one of FilterContains or FilterRegex must be set")
	}
	return nil
}

// RenderConfig produces a RainerScript snippet for rsyslog based on the params.
// Note: omprog arguments handling varies by rsyslog version. This renderer uses
// the common binary + arguments properties; adjust if your target differs.
func RenderConfig(p ConfigParams) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}

	var b bytes.Buffer
	// Ensure modules
	if p.PollingInterval > 0 {
		fmt.Fprintf(&b, "module(load=\"imfile\" PollingInterval=\"%d\")\n", p.PollingInterval)
	} else {
		b.WriteString("module(load=\"imfile\")\n")
	}
	b.WriteString("module(load=\"omprog\")\n\n")

	// imfile input
	b.WriteString("input(\n")
	fmt.Fprintf(&b, "\ttype=\"imfile\"\n")
	fmt.Fprintf(&b, "\tFile=\"%s\"\n", p.InputFile)
	fmt.Fprintf(&b, "\tTag=\"%s\"\n", p.Tag)
	if p.Severity != "" {
		fmt.Fprintf(&b, "\tSeverity=\"%s\"\n", p.Severity)
	}
	if p.Facility != "" {
		fmt.Fprintf(&b, "\tFacility=\"%s\"\n", p.Facility)
	}
	if p.AddMetadata {
		fmt.Fprintf(&b, "\taddMetadata=\"on\"\n")
	}
	// reopenOnTruncate keeps tailing rotated logs so triggers remain armed.
	fmt.Fprintf(&b, "\treopenOnTruncate=\"on\"\n")
	if p.StateFile != "" {
		fmt.Fprintf(&b, "\tStateFile=\"%s\"\n", p.StateFile)
	}
	if p.UseRuleset {
		fmt.Fprintf(&b, "\truleset=\"%s\"\n", p.RulesetName)
	}
	b.WriteString(")\n\n")

	// filter + action
	if p.UseRuleset {
		fmt.Fprintf(&b, "ruleset(name=\"%s\") {\n", p.RulesetName)
		b.WriteString("    if (")
	} else {
		b.WriteString("if (")
	}
	wrote := false
	if p.FilterByTag {
		fmt.Fprintf(&b, "$syslogtag contains '%s')", escapeQuotes(p.Tag))
		wrote = true
	}
	if p.FilterContains != "" {
		if wrote {
			b.WriteString(" and ")
		}
		fmt.Fprintf(&b, "($msg contains '%s')", escapeQuotes(p.FilterContains))
		wrote = true
	}
	if p.FilterRegex != "" {
		if wrote {
			b.WriteString(" or ")
		}
		fmt.Fprintf(&b, "re_match($msg, \"%s\")", escapeQuotes(p.FilterRegex))
		wrote = true
	}
	if !wrote {
		return "", errors.New("no filter expression constructed")
	}
	b.WriteString(" then {\n")
	if p.ProgramArgs != "" {
		fmt.Fprintf(&b, "        action(type=\"omprog\" binary=\"%s %s\")\n", escapeQuotes(p.ProgramPath), escapeQuotes(p.ProgramArgs))
	} else {
		fmt.Fprintf(&b, "        action(type=\"omprog\" binary=\"%s\")\n", escapeQuotes(p.ProgramPath))
	}
	if p.UseRuleset {
		b.WriteString("    }\n")
		b.WriteString("}\n")
	} else {
		b.WriteString("}\n")
	}

	return b.String(), nil
}

func escapeQuotes(s string) string {
	// Escape backslashes first, then quotes, for safe embedding in RainerScript strings.
	s = strings.ReplaceAll(s, `\\`, `\\\\`)
	s = strings.ReplaceAll(s, `"`, `\\"`)
	return s
}

// Prepare is a placeholder for future privileged operations.
func Prepare() error { return nil }
