package main

import (
	_ "embed"
	"os"

	"github.com/dev-toolings/ghostchrome/cmd"
)

var version = "dev"

// ghostchromeSkill is the agent skill bundled into the binary so it can be
// installed globally (`ghostchrome skills install`) and removed on uninstall.
//
//go:embed all:.claude/skills/ghostchrome/SKILL.md
var ghostchromeSkill string

func init() {
	cmd.SetVersion(version)
	cmd.SetEmbeddedSkill("ghostchrome", ghostchromeSkill)
}

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
