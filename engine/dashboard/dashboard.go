// Package dashboard provides a live browser viewport stream over WebSocket.
// It captures CDP screencast frames and broadcasts them to connected clients,
// plus an activity feed of navigation and interaction events.
package dashboard

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-rod/rod"
)

// Dashboard wraps the HTTP server and screencast streamer.
type Dashboard struct {
	srv    *server
	stream *streamer
	mu     sync.Mutex
	cancel context.CancelFunc
}

// Start begins screencasting and serving the dashboard UI on the given port.
// Returns the URL where the dashboard is accessible.
func Start(page *rod.Page, port int) (*Dashboard, string, error) {
	ctx, cancel := context.WithCancel(context.Background())

	str, err := newStreamer(ctx, page)
	if err != nil {
		cancel()
		return nil, "", fmt.Errorf("streamer: %w", err)
	}

	srv, addr, err := newServer(port, str)
	if err != nil {
		cancel()
		return nil, "", fmt.Errorf("server: %w", err)
	}

	go srv.serve(ctx)

	return &Dashboard{srv: srv, stream: str, cancel: cancel}, addr, nil
}

// PushEvent sends a text event to all connected dashboard clients.
func (d *Dashboard) PushEvent(text string) {
	if d == nil || d.stream == nil {
		return
	}
	d.stream.pushEvent(text)
}

// Stop shuts down the dashboard server and stops screencasting.
func (d *Dashboard) Stop() {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cancel != nil {
		d.cancel()
		d.cancel = nil
	}
	if d.stream != nil {
		d.stream.stop()
	}
}
