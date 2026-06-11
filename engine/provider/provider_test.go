package provider_test

import (
	"testing"

	"github.com/MakFly/ghostchrome/engine/provider"
)

func TestLocalName(t *testing.T) {
	p := provider.Local{}
	if p.Name() != "local" {
		t.Fatalf("expected 'local', got %q", p.Name())
	}
}

func TestLocalImplementsProvider(t *testing.T) {
	var _ provider.Provider = provider.Local{}
}
