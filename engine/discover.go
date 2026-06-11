package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// DefaultDiscoverPorts is the list of common Chrome remote debugging ports
// scanned by DiscoverCDP when no explicit port is supplied.
var DefaultDiscoverPorts = []int{9222, 9223, 9224, 9225, 9226, 9227, 9228, 9229}

// DiscoverCDP probes 127.0.0.1 on the given ports for a running Chrome
// exposing /json/version, and returns the first webSocketDebuggerUrl found.
// When ports is nil/empty, DefaultDiscoverPorts is used. Returns an error
// when no endpoint responds within timeout.
func DiscoverCDP(ports []int, timeout time.Duration) (string, error) {
	if len(ports) == 0 {
		ports = DefaultDiscoverPorts
	}
	if timeout <= 0 {
		timeout = 400 * time.Millisecond
	}

	type result struct {
		ws  string
		err error
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Indexed slots keep the result order aligned with the input ports
	// slice so the caller always sees the lowest-numbered responding port
	// first — regardless of which probe goroutine finishes first.
	results := make([]result, len(ports))
	var wg sync.WaitGroup
	wg.Add(len(ports))
	for i, p := range ports {
		i, p := i, p
		go func() {
			defer wg.Done()
			ws, err := probePort(ctx, p)
			results[i] = result{ws: ws, err: err}
		}()
	}
	wg.Wait()

	var lastErr error
	for _, r := range results {
		if r.ws != "" {
			return r.ws, nil
		}
		if r.err != nil {
			lastErr = r.err
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no Chrome found on default debug ports")
	}
	return "", fmt.Errorf("discover cdp: %w", lastErr)
}

func probePort(ctx context.Context, port int) (string, error) {
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	d := net.Dialer{Timeout: 200 * time.Millisecond}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return "", nil // closed port is not an error worth surfacing
	}
	_ = conn.Close()

	url := "http://" + addr + "/json/version"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 300 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", nil
	}
	var payload struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.WebSocketDebuggerURL == "" {
		return "", nil
	}
	return payload.WebSocketDebuggerURL, nil
}
