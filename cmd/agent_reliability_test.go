package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dev-toolings/ghostchrome/engine"
)

func TestAgentWriteRedactsPayloadAndPreservesProtocol(t *testing.T) {
	old := flagOutputSecrets
	flagOutputSecrets = []string{"secret", "42", "a\"b\n"}
	t.Cleanup(func() { flagOutputSecrets = old })
	var buf bytes.Buffer
	s := agentSession{enc: json.NewEncoder(&buf)}
	s.write(agentResponse{ID: "secret", OK: true, Result: map[string]any{"value": "a\"b\n secret 42", "number": 42}, Error: "secret", Events: []engine.ObserverEvent{{Text: "secret"}}})
	var got agentResponse
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	result := got.Result.(map[string]any)
	if got.ID != "secret" || !got.OK || got.Protocol != engine.ProtocolVersion || result["number"] != float64(42) {
		t.Fatalf("protocol or types changed: %+v", got)
	}
	if strings.Contains(result["value"].(string), "secret") || got.Error != "<redacted>" || got.Events[0].Text != "<redacted>" {
		t.Fatalf("payload not redacted: %+v", got)
	}
}

func TestSnapshotArtifactRedactsSecrets(t *testing.T) {
	oldDir, oldSecrets := flagConfigOutputDir, flagOutputSecrets
	flagConfigOutputDir, flagOutputSecrets = t.TempDir(), []string{"secret-value"}
	t.Cleanup(func() { flagConfigOutputDir, flagOutputSecrets = oldDir, oldSecrets })
	path, err := writePlaywrightSnapshotArtifact(&engine.ExtractionResult{Nodes: []engine.ExtractedNode{{Role: "button", Name: "secret-value"}}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-value") || !strings.Contains(string(data), "<redacted>") {
		t.Fatalf("unsafe artifact: %s", data)
	}
}

func TestAgentRejectsMalformedArgsBeforeOpeningBrowser(t *testing.T) {
	for op, raw := range map[string]string{"scroll_to": `{"y":"bad"}`, "screenshot": `{"full_page":"bad"}`, "wait": `{"ms":"bad"}`} {
		t.Run(op, func(t *testing.T) {
			s := newAgentSession()
			if _, err := s.dispatch(agentRequest{Op: op, Args: json.RawMessage(raw)}); err == nil {
				t.Fatal("expected argument error")
			}
			if s.browser != nil {
				s.shutdown()
				t.Fatal("invalid arguments opened a browser")
			}
		})
	}
}

func TestAgentRetainsDialogPolicyAndErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Chrome")
	}
	t.Setenv("HOME", t.TempDir())
	restore := snapshotConfigGlobals()
	t.Cleanup(restore)
	oldSkip, oldStealth := skipImplicitDaemon, flagStealth
	t.Cleanup(func() { skipImplicitDaemon, flagStealth = oldSkip, oldStealth })
	skipImplicitDaemon, flagStealth = true, false
	flagConnect, flagSession, flagUserProfile = "", "", ""
	flagUserDataDir = filepath.Join(t.TempDir(), "chrome")
	flagHeadless, flagTimeout = true, 15
	s := newAgentSession()
	t.Cleanup(s.shutdown)
	if _, err := s.opDialog(json.RawMessage(`{"action":"dismiss"}`)); err != nil {
		t.Fatal(err)
	}
	_, page, err := s.ensurePage()
	if err != nil {
		t.Fatal(err)
	}
	if accept, _ := s.dialogPolicy.Snapshot(); accept {
		t.Fatal("initialization replaced dismiss policy")
	}
	if _, err := engine.Navigate(page, "data:text/html,"+url.PathEscape(`<script>document.title=String(confirm('test'))</script>`), "load"); err != nil {
		t.Fatal(err)
	}
	info, err := page.Info()
	if err != nil || info.Title != "false" {
		t.Fatalf("dialog was not dismissed: %+v, %v", info, err)
	}
	if _, err := page.Eval(`() => console.error("retained-error")`); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		result, err := s.opErrors()
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, entry := range result.([]engine.ErrorEntry) {
			if entry.Message == "retained-error" {
				found = true
			}
		}
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("errors lost previous console event")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// An observation timeout after a completed action must never advise retry.
	ctx, cancel := context.WithCancel(page.GetContext())
	cancel()
	for _, mode := range []engine.SnapshotMode{engine.SnapshotModeFull, engine.SnapshotModeDiff} {
		result, err := s.mutationResult(s.browser, page.Context(ctx), nil, mode)
		if err == nil || result != nil {
			t.Fatalf("observation failure became success: %v, %v", result, err)
		}
		if _, retry := engine.ClassifyError(err); retry {
			t.Fatal("completed action is retryable")
		}
	}
}
