package engine

import (
	"reflect"
	"testing"
)

func TestSetVideoStateRestoresInMemoryStateWhenPersistenceFails(t *testing.T) {
	previous := VideoState{Active: true, Filename: "previous.webm"}
	browser := &Browser{
		connected: true,
		statePath: t.TempDir(), // Rename onto a directory fails after writing the temp state.
		state:     &sessionState{Video: previous},
	}
	if err := browser.SetVideoState(VideoState{Active: true, Filename: "next.webm"}); err == nil {
		t.Fatal("expected session-state persistence error")
	}
	if got := browser.VideoState(); !reflect.DeepEqual(got, previous) {
		t.Fatalf("VideoState() = %#v, want %#v", got, previous)
	}
}
