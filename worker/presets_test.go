package main

import (
	"strings"
	"testing"
)

func TestPresetTemplateResolves(t *testing.T) {
	for _, name := range []string{"claude", "codex", "aider", "cursor", "opencode"} {
		tmpl, ok := presetTemplate(name)
		if !ok {
			t.Fatalf("expected preset for %q", name)
		}
		if !strings.Contains(tmpl, "{{prompt}}") {
			t.Errorf("preset %q must interpolate the prompt, got %q", name, tmpl)
		}
	}
}

func TestPresetTemplateRejectsUnknown(t *testing.T) {
	if _, ok := presetTemplate("definitely-not-an-agent"); ok {
		t.Fatal("expected unknown agent to be rejected")
	}
}

// DeadDrop must remain the only thing that decides what lands, so no preset may
// let its CLI create commits on its own.
func TestAiderPresetDisablesAutoCommits(t *testing.T) {
	tmpl, _ := presetTemplate("aider")
	if !strings.Contains(tmpl, "--no-auto-commits") {
		t.Fatalf("aider preset must disable auto commits, got %q", tmpl)
	}
}

func TestKnownAgentsIncludesBuiltinsAndPresets(t *testing.T) {
	got := strings.Join(knownAgents(), ",")
	for _, name := range []string{"mock", "gemini", "custom", "claude", "codex", "aider"} {
		if !strings.Contains(got, name) {
			t.Errorf("knownAgents missing %q, got %s", name, got)
		}
	}
}

// A preset renders through the same quoting path as a custom template, so a
// prompt containing shell metacharacters must not escape its argument.
func TestPresetPromptIsShellQuoted(t *testing.T) {
	tmpl, _ := presetTemplate("claude")
	rendered := replaceTemplate(tmpl, "prompt", "oops'; rm -rf /; echo '")
	if strings.Contains(rendered, "; rm -rf /") && !strings.Contains(rendered, `'"'"'`) {
		t.Fatalf("prompt was not shell quoted: %s", rendered)
	}
}
