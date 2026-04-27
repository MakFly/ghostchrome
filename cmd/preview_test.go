package cmd

import "testing"

func TestPreviewWaitFlagDefaultsToLoad(t *testing.T) {
	flag := previewCmd.Flags().Lookup("wait")
	if flag == nil {
		t.Fatal("expected preview command to define --wait")
	}
	if flag.DefValue != "load" {
		t.Fatalf("expected preview --wait default to be load, got %q", flag.DefValue)
	}
}
