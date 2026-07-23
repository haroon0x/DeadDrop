package main

import "sort"

// AgentPreset is a starting command for a coding CLI. DeadDrop only needs the
// CLI to edit files inside the worktree: changed files come from Git and
// verification comes from the manifest, never from the agent's own claims.
type AgentPreset struct {
	Name     string
	Template string
	Notes    string
}

// agentPresets are convenience defaults, not a compatibility guarantee. CLI
// flags move between releases, so every preset can be overridden with
// --command-template without changing --agent.
var agentPresets = map[string]AgentPreset{
	"claude": {
		Name:     "claude",
		Template: "claude --print --permission-mode acceptEdits {{prompt}}",
		Notes:    "Claude Code in non-interactive print mode.",
	},
	"codex": {
		Name:     "codex",
		Template: "codex exec --full-auto {{prompt}}",
		Notes:    "Codex CLI non-interactive exec subcommand.",
	},
	"aider": {
		Name:     "aider",
		Template: "aider --yes --no-auto-commits --no-check-update --message {{prompt}}",
		Notes:    "Auto-commits are disabled so DeadDrop stays the only thing that decides what lands.",
	},
	"cursor": {
		Name:     "cursor",
		Template: "cursor-agent --print {{prompt}}",
		Notes:    "Cursor CLI in print mode.",
	},
	"opencode": {
		Name:     "opencode",
		Template: "opencode run {{prompt}}",
		Notes:    "OpenCode single-shot run.",
	},
}

// presetTemplate returns the default command for an agent name.
func presetTemplate(agent string) (string, bool) {
	preset, ok := agentPresets[agent]
	if !ok {
		return "", false
	}
	return preset.Template, true
}

// knownAgents lists every accepted --agent value, for help text and errors.
func knownAgents() []string {
	names := []string{"mock", "gemini", "custom"}
	for name := range agentPresets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
