# Skill packaging contract

`.claude/skills/ghostchrome/` is the canonical, version-controlled skill tree.
Keep one authored `SKILL.md` and one set of references, examples, and scripts.
Do not maintain separate Claude, Codex, Grok, CLI, or MCP copies in the source
tree.

Both release artifacts must carry this complete tree, or expose an equivalent
release payload to the setup installer:

```text
.claude/skills/ghostchrome/
├── SKILL.md
├── references/cli.md
├── references/mcp.md
├── references/troubleshooting.md
├── references/packaging.md
├── examples/cli-flow.sh
├── examples/mcp-config.toml
└── scripts/validate-skill.sh
```

The CLI and standalone MCP installers copy the same bytes to the selected global
client directories. A skill copy is valid only when its SHA-256 matches the
canonical `SKILL.md` and all referenced resources are present. Record that hash
in the installation manifest; use it to detect stale or partially installed
skills without reading user content.

Keep paths portable. References may point to repository documentation for local
development, but the installed skill must not require the checkout, a package
manager, or a hardcoded home directory. Client MCP configuration may contain the
stable installed executable path; the skill text itself must not embed that path.

Before publishing a release or changing the setup copier:

1. Run `scripts/validate-skill.sh` and the host skill validator.
2. Build both CLI and standalone MCP artifacts.
3. Verify that each artifact exposes the complete skill tree to setup.
4. Copy into temporary Claude, Codex, and Grok roots and compare the skill hash.
5. Confirm that CLI mode installs no MCP entry and MCP mode installs no CLI
   entrypoint.

Treat missing references, a hash mismatch, or an incomplete archive as a release
failure. Never silently regenerate the canonical skill from an installed copy.
