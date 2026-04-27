package cmd

import (
	"github.com/MakFly/ghostchrome/engine"
	"github.com/MakFly/ghostchrome/packages/leboncoin"
)

// Wire the leboncoin-specific subcommand tree under
// `ghostchrome leboncoin ...`. Same pattern as cmd/linkedin.go.
func init() {
	(&leboncoin.CommandFactory{
		BuildBrowserOpts: func() engine.BrowserOpts {
			return buildBrowserOpts()
		},
	}).Register(rootCmd)
}
