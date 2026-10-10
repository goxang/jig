//go:build !linux

package host

// confine does nothing off Linux: permissions there are advisory and audited.
func confine(sandboxSpec) error { return nil }

const sandboxed = false
