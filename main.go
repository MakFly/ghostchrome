package main

import (
	"os"

	"github.com/MakFly/ghostchrome/cmd"
)

var version = "dev"

func init() {
	cmd.SetVersion(version)
}

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
