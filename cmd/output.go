package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/MakFly/ghostchrome/engine"
)

// actionResult is a common result struct for interaction commands (click, hover, type).
type actionResult struct {
	Action  string                   `json:"action"`
	Ref     string                   `json:"ref,omitempty"`
	Locator string                   `json:"locator,omitempty"`
	Result  *engine.ExtractionResult `json:"result"`
}

// renderProfile resolves the current render profile once per invocation.
func renderProfile() engine.RenderProfile {
	return engine.ResolveProfile(flagProfile, flagFormat)
}

// output picks the right format based on --format / --profile.
// In agent-JSON mode, compact marshaling drops whitespace.
func output(jsonVal any, textVal string) {
	p := renderProfile()
	switch flagFormat {
	case "json":
		var (
			data []byte
			err  error
		)
		if p.Agent {
			data, err = json.Marshal(jsonVal)
		} else {
			data, err = json.MarshalIndent(jsonVal, "", "  ")
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: json marshal: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(data))
	default:
		fmt.Println(textVal)
	}
}
