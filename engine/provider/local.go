package provider

import (
	"context"
	"fmt"

	"github.com/go-rod/rod/lib/launcher"
)

// Local provisions Chrome by launching a local process via Rod's launcher.
type Local struct{}

func (Local) Name() string { return "local" }

func (Local) Connect(_ context.Context, opts ConnectOpts) (string, func(), error) {
	l := launcher.New()
	if opts.Headless {
		l = l.Headless(true)
	} else {
		l = l.Headless(false)
	}
	if opts.Proxy != "" {
		l = l.Proxy(opts.Proxy)
	}
	if opts.UserDataDir != "" {
		l = l.UserDataDir(opts.UserDataDir)
	}

	wsURL, err := l.Launch()
	if err != nil {
		return "", nil, fmt.Errorf("local launcher: %w", err)
	}

	cleanup := func() {
		l.Kill()
	}
	return wsURL, cleanup, nil
}
