package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
)

var (
	flagConsoleLevel          string
	flagConsoleWait           int
	flagConsoleClear          bool
	flagNetworkWait           int
	flagNetworkMax            int
	flagNetworkFilter         string
	flagNetworkStatic         bool
	flagNetworkRequestBody    bool
	flagNetworkRequestHeaders bool
	flagNetworkClear          bool
)

var consoleCompatCmd = &cobra.Command{
	Use:   "console [level|url] [url]",
	Short: "Observe console messages",
	Long: `Observe console messages during an optional navigation or wait window.
With a persistent session or --connect target, captured messages are appended to
the session console buffer until --clear is used. Ephemeral launches only report
messages captured while the command is running.`,
	Args: cobra.MaximumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		level, targetURL, err := resolveConsoleArgs(args, flagConsoleLevel)
		if err != nil {
			exitErr("console", err)
		}
		b, page := openPage()
		defer b.Close()
		if flagConsoleClear {
			if err := b.ClearConsoleLog(); err != nil {
				exitErr("console --clear", err)
			}
			type consoleResult struct {
				Entries []engine.ObserverEvent `json:"entries"`
				Count   int                    `json:"count"`
				Cleared bool                   `json:"cleared"`
			}
			output(consoleResult{Entries: []engine.ObserverEvent{}, Count: 0, Cleared: true}, "console buffer cleared")
			return
		}

		observer := engine.NewObserver(page, engine.ObserverOpts{
			Filters: engine.ObserverFilters{
				Kinds: []engine.ObserverKind{engine.KindConsole, engine.KindError},
			},
		})
		if err := observer.Start(context.Background()); err != nil {
			exitErr("console", err)
		}
		if targetURL != "" {
			navigateIfRequested(page, targetURL, "load")
		}
		if flagConsoleWait > 0 {
			time.Sleep(time.Duration(flagConsoleWait) * time.Second)
		}
		if err := observer.Stop(); err != nil {
			exitErr("console", err)
		}

		entries := observer.Drain(0)
		if entries == nil {
			entries = []engine.ObserverEvent{}
		}
		if err := b.AppendConsoleLog(entries); err != nil {
			exitErr("console", err)
		}
		if cumulative := b.ConsoleLog(); cumulative != nil {
			entries = cumulative
		}
		entries = filterConsoleLog(entries, level)
		type consoleResult struct {
			Entries []engine.ObserverEvent `json:"entries"`
			Count   int                    `json:"count"`
		}
		output(consoleResult{Entries: entries, Count: len(entries)}, formatConsoleEntries(entries))
	},
}

var networkCompatCmd = &cobra.Command{
	Use:   "network [url]",
	Short: "Observe network requests",
	Long: `Observe network requests during an optional navigation or wait window.
With a persistent session or --connect target, captured requests are appended to
the session network buffer until --clear is used. Ephemeral launches only report
requests captured while the command is running.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		targetURL := ""
		if len(args) > 0 {
			targetURL = args[0]
		}
		b, page := openPage()
		defer b.Close()
		if flagNetworkClear {
			if err := b.ClearNetworkLog(); err != nil {
				exitErr("network --clear", err)
			}
			type networkResult struct {
				Entries []*engine.CapturedEntry `json:"entries"`
				Count   int                     `json:"count"`
				Cleared bool                    `json:"cleared"`
			}
			output(networkResult{Entries: []*engine.CapturedEntry{}, Count: 0, Cleared: true}, "network log cleared")
			return
		}

		session, err := engine.StartCapture(page, engine.CaptureSpec{
			Max: flagNetworkMax,
		})
		if err != nil {
			exitErr("network", err)
		}
		if targetURL != "" {
			if _, err := engine.Navigate(page, targetURL, "load"); err != nil {
				fmt.Fprintf(os.Stderr, "navigate: %v\n", err)
			}
		}
		if flagNetworkWait > 0 {
			select {
			case <-session.ReachedMax():
			case <-time.After(time.Duration(flagNetworkWait) * time.Second):
			}
		}
		entries, err := session.Stop()
		if err != nil {
			exitErr("network", err)
		}
		if err := b.AppendNetworkLog(entries); err != nil {
			exitErr("network", err)
		}
		if cumulative := b.NetworkLog(); cumulative != nil {
			entries = cumulative
		}
		entries, err = filterNetworkLog(entries, flagNetworkFilter, flagNetworkStatic)
		if err != nil {
			exitErr("network", err)
		}
		entries = prepareNetworkEntries(entries, flagNetworkRequestBody, flagNetworkRequestHeaders)

		type networkResult struct {
			Entries []*engine.CapturedEntry `json:"entries"`
			Count   int                     `json:"count"`
		}
		output(networkResult{Entries: entries, Count: len(entries)}, formatNetworkEntries(entries))
	},
}

func resolveConsoleArgs(args []string, flagLevel string) (level string, targetURL string, err error) {
	level = flagLevel
	if !isConsoleLevel(level) {
		return "", "", errInvalidArg("level", level, "all, error, warning, info, debug")
	}
	if len(args) == 0 {
		return level, "", nil
	}
	if isConsoleLevel(args[0]) {
		level = args[0]
		if len(args) > 1 {
			targetURL = args[1]
		}
		return level, targetURL, nil
	}
	if len(args) > 1 {
		return "", "", fmt.Errorf("unexpected console argument %q; first argument must be a level when two arguments are provided", args[0])
	}
	return level, args[0], nil
}

func isConsoleLevel(level string) bool {
	switch level {
	case "all", "error", "warning", "info", "debug":
		return true
	default:
		return false
	}
}

func consoleFilterLevels(level string) []string {
	switch level {
	case "error":
		return []string{"error"}
	case "warning":
		return []string{"warning", "error"}
	case "all", "info":
		return []string{"log", "info", "warning", "error"}
	case "debug":
		return nil
	default:
		return []string{"log", "info", "warning", "error"}
	}
}

func filterConsoleLog(entries []engine.ObserverEvent, level string) []engine.ObserverEvent {
	allowed := consoleFilterLevels(level)
	if allowed == nil {
		return entries
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, item := range allowed {
		allowedSet[item] = struct{}{}
	}
	out := make([]engine.ObserverEvent, 0, len(entries))
	for _, entry := range entries {
		if _, ok := allowedSet[entry.Level]; ok {
			out = append(out, entry)
		}
	}
	return out
}

func formatConsoleEntries(entries []engine.ObserverEvent) string {
	if len(entries) == 0 {
		return "No console messages"
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		source := ""
		if entry.Source != "" {
			source = " (" + entry.Source + ")"
		}
		lines = append(lines, fmt.Sprintf("[console:%s] %s%s", entry.Level, entry.Text, source))
	}
	return strings.Join(lines, "\n")
}

func filterNetworkLog(entries []*engine.CapturedEntry, filter string, includeStatic bool) ([]*engine.CapturedEntry, error) {
	var re *regexp.Regexp
	var err error
	if filter != "" {
		re, err = regexp.Compile(filter)
		if err != nil {
			return nil, err
		}
	}
	out := make([]*engine.CapturedEntry, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if re != nil && !re.MatchString(entry.URL) {
			continue
		}
		if !includeStatic && engine.IsStaticNetworkEntry(entry.ResourceType, entry.MimeType) {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

func formatNetworkEntries(entries []*engine.CapturedEntry) string {
	if len(entries) == 0 {
		return "No network entries"
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, fmt.Sprintf("[%d] %s %s", entry.Status, entry.Method, entry.URL))
	}
	return strings.Join(lines, "\n")
}

func prepareNetworkEntries(entries []*engine.CapturedEntry, includeRequestBody, includeRequestHeaders bool) []*engine.CapturedEntry {
	for _, entry := range entries {
		if !includeRequestBody {
			entry.PostData = ""
		}
		if !includeRequestHeaders {
			entry.ReqHeaders = nil
		}
	}
	return entries
}

func init() {
	consoleCompatCmd.Flags().StringVar(&flagConsoleLevel, "level", "all", "Filter level: all, error, warning, info, debug")
	consoleCompatCmd.Flags().IntVar(&flagConsoleWait, "wait", 0, "Seconds to keep observing after optional navigation")
	consoleCompatCmd.Flags().BoolVar(&flagConsoleClear, "clear", false, "Clear the current console buffer and return no entries")
	networkCompatCmd.Flags().IntVar(&flagNetworkWait, "wait", 1, "Seconds to keep observing after optional navigation")
	networkCompatCmd.Flags().IntVar(&flagNetworkMax, "max", 0, "Stop after N completed requests (0 = unlimited)")
	networkCompatCmd.Flags().StringVar(&flagNetworkFilter, "filter", "", "Filter requests by URL regex")
	networkCompatCmd.Flags().BoolVar(&flagNetworkStatic, "static", false, "Include static resources such as images, CSS, fonts, and media")
	networkCompatCmd.Flags().BoolVar(&flagNetworkRequestBody, "request-body", false, "Include request bodies")
	networkCompatCmd.Flags().BoolVar(&flagNetworkRequestHeaders, "request-headers", false, "Include request headers")
	networkCompatCmd.Flags().BoolVar(&flagNetworkClear, "clear", false, "Clear the current network log and return no entries")
	rootCmd.AddCommand(consoleCompatCmd, networkCompatCmd)
}
