package cmd

import (
	"fmt"
	"time"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var flagDialogText string

var dialogCmd = &cobra.Command{
	Use:   "dialog",
	Short: "Handle JavaScript dialogs (alert, confirm, prompt)",
	Long: `Set up a one-shot handler for the next JavaScript dialog.
Use 'dialog accept' or 'dialog dismiss' subcommands.`,
}

var dialogAcceptCmd = &cobra.Command{
	Use:   "accept",
	Short: "Accept the next dialog",
	Long: `Set up a handler that will accept the next JavaScript dialog (alert/confirm/prompt).
For prompt() dialogs, use --text to provide a response.

Examples:
  ghostchrome dialog accept --connect ws://127.0.0.1:9222
  ghostchrome dialog accept --text "yes" --connect ws://127.0.0.1:9222`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runDialogHandler(true, flagDialogText)
	},
}

var dialogDismissCmd = &cobra.Command{
	Use:   "dismiss",
	Short: "Dismiss the next dialog",
	Long: `Set up a handler that will dismiss the next JavaScript dialog (alert/confirm/prompt).

Examples:
  ghostchrome dialog dismiss --connect ws://127.0.0.1:9222`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		runDialogHandler(false, flagDialogText)
	},
}

func runDialogHandler(accept bool, text string) {
	b, page := openPage()
	defer b.Close()

	result, err := engine.HandleNextDialog(page, accept, text, time.Duration(flagTimeout)*time.Second)
	if err != nil {
		exitErr("dialog", err)
	}

	text2 := fmt.Sprintf("Dialog %s timed out after %ds", result.Action, flagTimeout)
	if result.Handled {
		text2 = fmt.Sprintf("Dialog %s: [%s] %s", result.Action, result.Type, result.Message)
	}

	output(result, text2)
}

func init() {
	dialogAcceptCmd.Flags().StringVar(&flagDialogText, "text", "", "Response text for prompt() dialogs")
	dialogDismissCmd.Flags().StringVar(&flagDialogText, "text", "", "Response text for prompt() dialogs")
	dialogCmd.AddCommand(dialogAcceptCmd)
	dialogCmd.AddCommand(dialogDismissCmd)
	rootCmd.AddCommand(dialogCmd)
}
