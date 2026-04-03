package cmd

import (
	"encoding/json"
	"fmt"
	"os"
)

// printJSON marshals v to JSON and prints to stdout.
func printJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: json marshal: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

// printText prints s to stdout.
func printText(s string) {
	fmt.Println(s)
}

// output picks the right format based on flagFormat.
func output(jsonVal any, textVal string) {
	switch flagFormat {
	case "json":
		printJSON(jsonVal)
	default:
		printText(textVal)
	}
}
