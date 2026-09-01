package main

import (
	"embed"
	"io/fs"
	"os"
	"strings"

	"github.com/dev-toolings/ghostchrome/cmd"
)

var version = "dev"

// ghostchromeSkill is the agent skill bundled into the binary so it can be
// installed globally (`ghostchrome skills install`) and removed on uninstall.
//
//go:embed all:.claude/skills/ghostchrome/SKILL.md
var ghostchromeSkill string

// ghostchromeSkillTree carries the complete client-neutral skill bundle in
// the CLI artifact. The standalone MCP artifact is runtime-only; the release
// installer downloads the same tree as a separate asset.
//
//go:embed all:.claude/skills/ghostchrome
var ghostchromeSkillTree embed.FS

func init() {
	cmd.SetVersion(version)
	cmd.SetEmbeddedSkill("ghostchrome", ghostchromeSkill)
	_ = fs.WalkDir(ghostchromeSkillTree, ".claude/skills/ghostchrome", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, readErr := ghostchromeSkillTree.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		relative := strings.TrimPrefix(path, ".claude/skills/ghostchrome/")
		cmd.SetEmbeddedSkillFile(relative, string(data))
		return nil
	})
}

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
