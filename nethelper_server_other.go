//go:build !linux

package main

import "fmt"

func runNetHelperServer([]string) error { return fmt.Errorf("net-helper server is Linux-only") }
