package cmd

import (
	"testing"

	"github.com/MakFly/ghostchrome/engine"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestPlaywrightCompatCommandsRegistered(t *testing.T) {
	// Source: https://playwright.dev/agent-cli/introduction#all-commands
	// plus https://playwright.dev/agent-cli/capabilities for network-state-set
	// and the testing commands. Keep this list honest: registered does not mean
	// fully compatible, it only proves ghostchrome exposes the documented name.
	commands := []string{
		"open",
		"goto",
		"close",
		"click",
		"dblclick",
		"fill",
		"type",
		"select",
		"check",
		"uncheck",
		"hover",
		"drag",
		"upload",
		"snapshot",
		"screenshot",
		"pdf",
		"eval",
		"resize",
		"dialog-accept",
		"dialog-dismiss",
		"go-back",
		"go-forward",
		"reload",
		"press",
		"keydown",
		"keyup",
		"mousemove",
		"mousedown",
		"mouseup",
		"mousewheel",
		"tab-list",
		"tab-new",
		"tab-select",
		"tab-close",
		"state-save",
		"state-load",
		"cookie-list",
		"cookie-get",
		"cookie-set",
		"cookie-delete",
		"cookie-clear",
		"localstorage-list",
		"localstorage-get",
		"localstorage-set",
		"localstorage-delete",
		"localstorage-clear",
		"sessionstorage-list",
		"sessionstorage-get",
		"sessionstorage-set",
		"sessionstorage-delete",
		"sessionstorage-clear",
		"network",
		"route",
		"route-list",
		"unroute",
		"network-state-set",
		"console",
		"run-code",
		"tracing-start",
		"tracing-stop",
		"video-start",
		"video-chapter",
		"video-stop",
		"show",
		"pause-at",
		"resume",
		"step-over",
		"list",
		"close-all",
		"kill-all",
		"delete-data",
		"attach",
		"config-print",
		"install",
		"verify-element-visible",
		"verify-text-visible",
		"verify-list-visible",
		"verify-value",
		"generate-locator",
	}

	for _, name := range commands {
		if _, _, err := rootCmd.Find([]string{name, "--help"}); err != nil {
			t.Fatalf("expected %q to be registered: %v", name, err)
		}
	}
}

func TestPlaywrightCompatFlagsRegistered(t *testing.T) {
	checks := []struct {
		command string
		flag    string
	}{
		{command: "screenshot", flag: "filename"},
		{command: "screenshot", flag: "full-page"},
		{command: "snapshot", flag: "filename"},
		{command: "snapshot", flag: "depth"},
		{command: "snapshot", flag: "raw"},
		{command: "pdf", flag: "filename"},
		{command: "attach", flag: "cdp"},
		{command: "attach", flag: "endpoint"},
		{command: "attach", flag: "extension"},
		{command: "cookie-list", flag: "domain"},
		{command: "cookie-list", flag: "path"},
		{command: "cookie-set", flag: "same-site"},
		{command: "close-all", flag: "purge"},
		{command: "kill-all", flag: "purge"},
		{command: "console", flag: "level"},
		{command: "console", flag: "wait"},
		{command: "console", flag: "clear"},
		{command: "network", flag: "wait"},
		{command: "network", flag: "max"},
		{command: "network", flag: "filter"},
		{command: "network", flag: "static"},
		{command: "network", flag: "request-body"},
		{command: "network", flag: "request-headers"},
		{command: "network", flag: "clear"},
		{command: "tracing-stop", flag: "output"},
		{command: "video-start", flag: "size"},
		{command: "video-chapter", flag: "description"},
		{command: "video-chapter", flag: "duration"},
		{command: "run-code", flag: "filename"},
		{command: "open", flag: "browser"},
		{command: "open", flag: "persistent"},
		{command: "open", flag: "profile"},
		{command: "open", flag: "headed"},
		{command: "open", flag: "config"},
		{command: "install", flag: "skills"},
		{command: "verify-element-visible", flag: "by-role"},
		{command: "verify-element-visible", flag: "by-name"},
		{command: "verify-element-visible", flag: "by-label"},
		{command: "verify-element-visible", flag: "by-text"},
		{command: "verify-element-visible", flag: "url"},
		{command: "verify-text-visible", flag: "url"},
		{command: "verify-list-visible", flag: "url"},
		{command: "verify-value", flag: "by-role"},
		{command: "verify-value", flag: "by-name"},
		{command: "verify-value", flag: "by-label"},
		{command: "verify-value", flag: "by-text"},
		{command: "verify-value", flag: "url"},
		{command: "generate-locator", flag: "by-role"},
		{command: "generate-locator", flag: "by-name"},
		{command: "generate-locator", flag: "by-label"},
		{command: "generate-locator", flag: "by-text"},
		{command: "generate-locator", flag: "url"},
		{command: "route", flag: "status"},
		{command: "route", flag: "body"},
		{command: "route", flag: "content-type"},
		{command: "route", flag: "header"},
		{command: "route", flag: "remove-header"},
	}

	for _, check := range checks {
		cmd, _, err := rootCmd.Find([]string{check.command, "--help"})
		if err != nil {
			t.Fatalf("find %q: %v", check.command, err)
		}
		if lookupAnyFlag(cmd, check.flag) == nil {
			t.Fatalf("expected %q to define --%s", check.command, check.flag)
		}
	}
}

func TestCloseCompatReusableContextDetection(t *testing.T) {
	oldConnect := flagConnect
	oldSession := flagSession
	defer func() {
		flagConnect = oldConnect
		flagSession = oldSession
	}()

	t.Setenv("PLAYWRIGHT_CLI_SESSION", "")
	t.Setenv("GHOSTCHROME_SESSION", "")
	flagConnect = ""
	flagSession = ""
	if hasReusableBrowserContext() {
		t.Fatal("expected no reusable context by default")
	}

	flagConnect = "ws://127.0.0.1:9222/devtools/browser/test"
	if !hasReusableBrowserContext() {
		t.Fatal("expected --connect to count as reusable context")
	}

	flagConnect = ""
	flagSession = "work"
	if !hasReusableBrowserContext() {
		t.Fatal("expected --session to count as reusable context")
	}
}

func lookupAnyFlag(cmd *cobra.Command, name string) *pflag.Flag {
	if flag := cmd.Flags().Lookup(name); flag != nil {
		return flag
	}
	if flag := cmd.InheritedFlags().Lookup(name); flag != nil {
		return flag
	}
	if root := cmd.Root(); root != nil {
		return root.PersistentFlags().Lookup(name)
	}
	return nil
}

func TestSnapshotArgumentHelpers(t *testing.T) {
	refChecks := map[string]bool{
		"e34": true,
		"@2":  true,
		"env": false,
		"e":   false,
	}
	for value, want := range refChecks {
		if got := isSnapshotRef(value); got != want {
			t.Fatalf("isSnapshotRef(%q) = %v, want %v", value, got, want)
		}
	}

	selectorChecks := map[string]bool{
		"#main":            true,
		".item":            true,
		"[data-testid=go]": true,
		"main > button":    true,
		"https://example":  false,
		"example.com/path": false,
		"about:blank":      false,
	}
	for value, want := range selectorChecks {
		if got := looksLikeSelector(value); got != want {
			t.Fatalf("looksLikeSelector(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestResolveConsoleArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		flagLevel string
		wantLevel string
		wantURL   string
		wantErr   bool
	}{
		{name: "default", args: nil, flagLevel: "all", wantLevel: "all"},
		{name: "positional error", args: []string{"error"}, flagLevel: "all", wantLevel: "error"},
		{name: "positional info", args: []string{"info"}, flagLevel: "all", wantLevel: "info"},
		{name: "positional warning with url", args: []string{"warning", "https://example.com"}, flagLevel: "all", wantLevel: "warning", wantURL: "https://example.com"},
		{name: "url only", args: []string{"https://example.com"}, flagLevel: "error", wantLevel: "error", wantURL: "https://example.com"},
		{name: "invalid flag", args: nil, flagLevel: "nope", wantErr: true},
		{name: "two args require level", args: []string{"https://example.com", "extra"}, flagLevel: "all", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLevel, gotURL, err := resolveConsoleArgs(tt.args, tt.flagLevel)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotLevel != tt.wantLevel || gotURL != tt.wantURL {
				t.Fatalf("resolveConsoleArgs() = (%q, %q), want (%q, %q)", gotLevel, gotURL, tt.wantLevel, tt.wantURL)
			}
		})
	}
}

func TestConsoleFilterLevels(t *testing.T) {
	tests := []struct {
		level string
		want  []string
	}{
		{level: "error", want: []string{"error"}},
		{level: "warning", want: []string{"warning", "error"}},
		{level: "info", want: []string{"log", "info", "warning", "error"}},
		{level: "all", want: []string{"log", "info", "warning", "error"}},
		{level: "debug", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			got := consoleFilterLevels(tt.level)
			if len(got) != len(tt.want) {
				t.Fatalf("consoleFilterLevels(%q) len = %d, want %d (%v)", tt.level, len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("consoleFilterLevels(%q)[%d] = %q, want %q", tt.level, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFilterConsoleLog(t *testing.T) {
	entries := []engine.ObserverEvent{
		{Level: "log", Text: "hello"},
		{Level: "warning", Text: "warn"},
		{Level: "error", Text: "boom"},
	}
	got := filterConsoleLog(entries, "warning")
	if len(got) != 2 {
		t.Fatalf("warning filter returned %d entries, want 2", len(got))
	}
	if got[0].Level != "warning" || got[1].Level != "error" {
		t.Fatalf("unexpected warning filter result: %#v", got)
	}
	if got := filterConsoleLog(entries, "debug"); len(got) != 3 {
		t.Fatalf("debug filter returned %d entries, want 3", len(got))
	}
}

func TestFilterNetworkLog(t *testing.T) {
	entries := []*engine.CapturedEntry{
		{URL: "https://example.com/app.js", ResourceType: "Script", MimeType: "application/javascript"},
		{URL: "https://example.com/logo.png", ResourceType: "Image", MimeType: "image/png"},
		{URL: "https://example.com/api/users", ResourceType: "Fetch", MimeType: "application/json"},
	}
	got, err := filterNetworkLog(entries, "api", false)
	if err != nil {
		t.Fatalf("filterNetworkLog: %v", err)
	}
	if len(got) != 1 || got[0].URL != "https://example.com/api/users" {
		t.Fatalf("unexpected filtered network log: %#v", got)
	}
	got, err = filterNetworkLog(entries, "", false)
	if err != nil {
		t.Fatalf("filterNetworkLog: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("static resources should be excluded by default, got %d entries", len(got))
	}
	got, err = filterNetworkLog(entries, "", true)
	if err != nil {
		t.Fatalf("filterNetworkLog: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("--static should include all 3 entries, got %d", len(got))
	}
}

func TestPrepareNetworkEntries(t *testing.T) {
	entries := []*engine.CapturedEntry{{
		Method:     "POST",
		URL:        "https://example.com/api",
		PostData:   `{"ok":true}`,
		ReqHeaders: map[string]string{"Authorization": "Bearer token"},
	}}

	prepareNetworkEntries(entries, false, false)
	if entries[0].PostData != "" {
		t.Fatalf("PostData was not stripped: %q", entries[0].PostData)
	}
	if entries[0].ReqHeaders != nil {
		t.Fatalf("ReqHeaders were not stripped: %#v", entries[0].ReqHeaders)
	}

	entries = []*engine.CapturedEntry{{
		PostData:   `{"ok":true}`,
		ReqHeaders: map[string]string{"Authorization": "Bearer token"},
	}}
	prepareNetworkEntries(entries, true, true)
	if entries[0].PostData == "" {
		t.Fatal("PostData should be kept")
	}
	if entries[0].ReqHeaders == nil {
		t.Fatal("ReqHeaders should be kept")
	}
}
