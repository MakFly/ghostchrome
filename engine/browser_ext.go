package engine

import "github.com/go-rod/rod"

// RawBrowser returns the underlying rod.Browser instance.
// Needed for commands that operate on browser-level (tabs, etc).
func (b *Browser) RawBrowser() *rod.Browser { return b.browser }
