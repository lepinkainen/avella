//go:build !systray

package ui

// New returns a headless UI when built without systray support.
func New() UI { return NewNoop() }
