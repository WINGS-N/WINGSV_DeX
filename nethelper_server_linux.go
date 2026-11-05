//go:build linux

package main

import (
	"fmt"
	"strconv"

	"github.com/WINGS-N/wingsv-dex/internal/nethelper"
)

func runNetHelperServer(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: --net-helper-server <socket-path> <allowed-uid>")
	}
	uid, err := strconv.ParseUint(args[1], 10, 32)
	if err != nil {
		return fmt.Errorf("invalid allowed uid: %w", err)
	}
	return nethelper.RunServer(args[0], uint32(uid))
}
