package ui

// New returns a socket-backed UI implementation.
func New() UI { return NewSocket() }
