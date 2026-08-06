package cmd

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dev-toolings/ghostchrome/engine"
	"github.com/go-rod/rod/lib/proto"
	"github.com/spf13/cobra"
	"github.com/ysmood/gson"
)

var flagTracingOutput string

var tracingStartCmd = &cobra.Command{
	Use:   "tracing-start",
	Short: "Start browser tracing",
	Long: `Start Chrome DevTools Protocol browser tracing for the active session.
This is a Playwright CLI-compatible command name. tracing-stop writes a
ghostchrome CDP trace bundle by default, not a Playwright Trace Viewer-compatible
trace yet.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		b, _ := openPage()
		defer b.Close()
		if !b.Connected() {
			exitErr("tracing-start", fmt.Errorf("requires -s/--session, --connect, or an attached default session"))
		}

		if state := b.BrowserTraceState(); state.Active {
			exitErr("tracing-start", fmt.Errorf("browser tracing already active since %s", state.StartedAt))
		}
		if err := (proto.TracingStart{
			Categories:   "devtools.timeline,v8,blink,netlog,disabled-by-default-devtools.screenshot",
			TransferMode: proto.TracingStartTransferModeReportEvents,
		}).Call(b.RodBrowser()); err != nil {
			exitErr("tracing-start", err)
		}
		started := time.Now().UTC().Format(time.RFC3339)
		if err := b.SetBrowserTraceState(engine.BrowserTraceState{Active: true, StartedAt: started, Output: defaultTraceOutput(flagTracingOutput)}); err != nil {
			exitErr("tracing-start", err)
		}
		output(map[string]any{"active": true, "started_at": started}, "tracing started")
	},
}

var tracingStopCmd = &cobra.Command{
	Use:   "tracing-stop",
	Short: "Stop browser tracing and save a trace file",
	Long: `Stop Chrome DevTools Protocol browser tracing and save the collected
events. The default output is .playwright-cli/trace.zip, matching the
Playwright CLI path shape. The zip is a ghostchrome CDP trace bundle and is
marked as not Playwright Trace Viewer-compatible until ghostchrome emits the
real Playwright trace schema.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		b, _ := openPage()
		defer b.Close()
		if !b.Connected() {
			exitErr("tracing-stop", fmt.Errorf("requires -s/--session, --connect, or an attached default session"))
		}

		state := b.BrowserTraceState()
		if !state.Active {
			exitErr("tracing-stop", fmt.Errorf("browser tracing is not active"))
		}
		outputPath := defaultTraceOutput(flagTracingOutput)
		if outputPath == "" {
			outputPath = defaultTraceOutput(state.Output)
		}

		var events []map[string]any
		complete := make(chan proto.TracingTracingComplete, 1)
		wait := b.RodBrowser().EachEvent(
			func(e *proto.TracingDataCollected) {
				events = append(events, decodeTraceEvents(e.Value)...)
			},
			func(e *proto.TracingTracingComplete) bool {
				complete <- *e
				return true
			},
		)

		if err := (proto.TracingEnd{}).Call(b.RodBrowser()); err != nil {
			exitErr("tracing-stop", err)
		}
		done := make(chan struct{})
		go func() {
			wait()
			close(done)
		}()

		var traceComplete proto.TracingTracingComplete
		select {
		case traceComplete = <-complete:
		case <-time.After(15 * time.Second):
			exitErr("tracing-stop", fmt.Errorf("timed out waiting for Tracing.tracingComplete"))
		}
		<-done

		format, err := writeTraceOutput(outputPath, events, state.StartedAt, traceComplete.DataLossOccurred)
		if err != nil {
			exitErr("tracing-stop", err)
		}
		if err := b.SetBrowserTraceState(engine.BrowserTraceState{}); err != nil {
			exitErr("tracing-stop", err)
		}
		type tracingStopResult struct {
			Output                string `json:"output"`
			Format                string `json:"format"`
			Events                int    `json:"events"`
			DataLossOccurred      bool   `json:"data_loss_occurred"`
			PlaywrightCompatible  bool   `json:"playwright_compatible"`
			TraceViewerCompatible bool   `json:"trace_viewer_compatible"`
		}
		result := tracingStopResult{
			Output:                outputPath,
			Format:                format,
			Events:                len(events),
			DataLossOccurred:      traceComplete.DataLossOccurred,
			PlaywrightCompatible:  false,
			TraceViewerCompatible: false,
		}
		output(result, fmt.Sprintf("trace saved to %s (%d events)", outputPath, len(events)))
	},
}

func defaultTraceOutput(path string) string {
	if path != "" {
		return path
	}
	return playwrightArtifactPath("trace.zip")
}

func decodeTraceEvents(raw []map[string]gson.JSON) []map[string]any {
	out := make([]map[string]any, 0, len(raw))
	for _, event := range raw {
		decoded := make(map[string]any, len(event))
		for key, value := range event {
			decoded[key] = value.Val()
		}
		out = append(out, decoded)
	}
	return out
}

func writeTraceJSON(path string, events []map[string]any, startedAt string, dataLoss bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	payload := map[string]any{
		"traceEvents":           events,
		"metadata":              map[string]any{"tool": "ghostchrome", "started_at": startedAt, "format": "cdp-json"},
		"data_loss_occurred":    dataLoss,
		"playwright_compatible": false,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func writeTraceOutput(path string, events []map[string]any, startedAt string, dataLoss bool) (string, error) {
	if strings.EqualFold(filepath.Ext(path), ".zip") {
		return "ghostchrome-cdp-zip", writeTraceZip(path, events, startedAt, dataLoss)
	}
	return "cdp-json", writeTraceJSON(path, events, startedAt, dataLoss)
}

func writeTraceZip(path string, events []map[string]any, startedAt string, dataLoss bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	if err := zipJSON(zw, "cdp-trace.json", map[string]any{
		"traceEvents":        events,
		"data_loss_occurred": dataLoss,
	}); err != nil {
		_ = zw.Close()
		return err
	}
	if err := zipJSON(zw, "metadata.json", map[string]any{
		"tool":                    "ghostchrome",
		"started_at":              startedAt,
		"format":                  "ghostchrome-cdp-zip",
		"playwright_compatible":   false,
		"trace_viewer_compatible": false,
		"events":                  len(events),
	}); err != nil {
		_ = zw.Close()
		return err
	}
	if err := zipText(zw, "README.md", "This archive was written by ghostchrome.\n\nIt contains Chrome DevTools Protocol trace events in cdp-trace.json.\nIt is intentionally marked as not Playwright Trace Viewer-compatible until ghostchrome emits the real Playwright trace schema.\n"); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

func zipJSON(zw *zip.Writer, name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return zipText(zw, name, string(data))
}

func zipText(zw *zip.Writer, name string, text string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(text))
	return err
}

func init() {
	tracingStopCmd.Flags().StringVar(&flagTracingOutput, "output", "", "Output trace path (default .playwright-cli/trace.zip; use .json for raw CDP JSON)")
	rootCmd.AddCommand(tracingStartCmd, tracingStopCmd)
	commandGroups["tracing-start"] = "observe"
	commandGroups["tracing-stop"] = "observe"
}
