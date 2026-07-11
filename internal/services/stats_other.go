//go:build !windows

package services

// interfaceCounters reads a network interface's rx/tx byte counters from sysfs.
func interfaceCounters(name string) (int64, int64, bool) { return wgInterfaceStats(name) }
