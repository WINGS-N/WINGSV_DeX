//go:build !linux

package xray

// physicalOutboundInterface is linux-only; elsewhere the tun inbound keeps the fork's
// built-in "auto" interface detection.
func physicalOutboundInterface() string { return "" }
