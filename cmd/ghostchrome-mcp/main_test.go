package main

import (
	"testing"
	"time"
)

func TestMCPIdleTimeoutResolution(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "unset defaults to 15m", value: "", want: defaultMCPIdleTimeout},
		{name: "blank defaults to 15m", value: "   ", want: defaultMCPIdleTimeout},
		{name: "duration", value: "30m", want: 30 * time.Minute},
		{name: "bare seconds", value: "900", want: 900 * time.Second},
		{name: "zero disables", value: "0", want: 0},
		{name: "off disables", value: "off", want: 0},
		{name: "invalid disables", value: "not-a-duration", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GHOSTCHROME_IDLE_TIMEOUT", tt.value)
			if got := mcpIdleTimeout(); got != tt.want {
				t.Fatalf("mcpIdleTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}
