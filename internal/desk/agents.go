package desk

type spec struct {
	id, name, root                                     string
	configs, instructions, skills, plugins             []string
	projectConfigs, projectInstructions, projectSkills []string
}

func specs() []spec {
	return []spec{
		{"codex", "Codex", ".codex", []string{".codex/config.toml"}, []string{".codex/AGENTS.md"}, []string{".codex/skills", ".agents/skills"}, []string{".codex/plugins/cache"}, []string{".codex/config.toml"}, []string{"AGENTS.md", "AGENTS.override.md"}, []string{".agents/skills", ".codex/skills"}},
		{"claude", "Claude Code", ".claude", []string{".claude/settings.json", ".claude.json"}, []string{".claude/CLAUDE.md"}, []string{".claude/skills"}, []string{".claude/plugins/cache", ".claude/plugins/synced"}, []string{".claude/settings.json", ".claude/settings.local.json", ".mcp.json"}, []string{"CLAUDE.md", "CLAUDE.local.md", ".claude/CLAUDE.md", "AGENTS.md"}, []string{".claude/skills"}},
		{"opencode", "OpenCode", ".config/opencode", []string{".config/opencode/opencode.json", ".config/opencode/opencode.jsonc"}, []string{".config/opencode/AGENTS.md"}, []string{".config/opencode/skills", ".agents/skills", ".claude/skills"}, []string{".config/opencode/plugins"}, []string{"opencode.json", "opencode.jsonc", ".opencode/opencode.json", ".opencode/opencode.jsonc"}, []string{"AGENTS.md", "CLAUDE.md"}, []string{".opencode/skills", ".agents/skills", ".claude/skills"}},
		{"grok", "Grok", ".grok", []string{".grok/config.toml", ".grok/managed_config.toml", ".grok/requirements.toml"}, []string{".grok/AGENTS.md"}, []string{".grok/skills", ".agents/skills", ".claude/skills", ".grok/bundled/skills"}, []string{".grok/installed-plugins"}, []string{".grok/config.toml", ".mcp.json"}, []string{"AGENTS.md", ".grok/AGENTS.md"}, []string{".grok/skills", ".agents/skills", ".claude/skills"}},
		{"agy", "Agy", ".gemini/antigravity-cli", []string{".gemini/antigravity-cli/settings.json", ".gemini/config/mcp_config.json", ".gemini/config/skills.json", ".gemini/config/plugins.json"}, []string{".gemini/GEMINI.md", ".gemini/config/AGENTS.md"}, []string{".gemini/config/skills", ".gemini/antigravity-cli/builtin/skills"}, []string{".gemini/config/plugins", ".gemini/antigravity-cli/builtin/plugins"}, []string{".agents/skills.json", ".agents/plugins.json"}, []string{"AGENTS.md", "GEMINI.md"}, []string{".agents/skills", ".agent/skills", "_agents/skills", "_agent/skills"}},
	}
}
