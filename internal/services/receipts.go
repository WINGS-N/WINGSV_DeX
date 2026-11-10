package services

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/WINGS-N/wingsv-dex/internal/config"
	"github.com/WINGS-N/wingsv-dex/internal/federation"
)

// receiptSampleInterval is how often the engine is fed fresh counters. The engine itself only
// closes a window every few minutes, so a tighter interval just keeps the baseline current
// and costs one cheap stats read.
const receiptSampleInterval = 60 * time.Second

// xrayStatsEndpoints returns the value for the xray config's api.listen and the gRPC target
// to dial it back on, or two empty strings when receipts are off. Off Windows the commander
// binds a unix socket in a private 0700 directory and chmods it 0666 (the ",0666" suffix):
// xray runs as root under the privileged helper, so the socket is root-owned, and the 0700
// parent is what keeps other local users out while leaving this user able to connect. On
// Windows AF_UNIX gRPC is unreliable, so a loopback-only TCP port stands in.
func (s *ConnectionService) xrayStatsEndpoints() (listen, target string) {
	s.mu.Lock()
	engine := s.receiptEngine
	dir := s.configDir
	s.mu.Unlock()
	if engine == nil || dir == "" {
		return "", ""
	}
	if runtime.GOOS == "windows" {
		port := freeLoopbackPort()
		if port == 0 {
			return "", ""
		}
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		return addr, addr
	}
	statsDir := filepath.Join(dir, "xray")
	if err := os.MkdirAll(statsDir, 0o700); err != nil {
		return "", ""
	}
	sock := filepath.Join(statsDir, "stats.sock")
	// A socket left behind by a previous run makes bind fail with EADDRINUSE. A root-run xray
	// may have left it owned by root; unlink still works because the 0700 parent is ours.
	_ = os.Remove(sock)
	return sock + ",0666", "unix://" + sock
}

// freeLoopbackPort probes a free loopback TCP port, or 0 if none can be found.
func freeLoopbackPort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// startXrayReceipts dials the xray stats commander and samples the exact proxy-outbound
// counters into the receipt engine until ctx is cancelled. nodeID is the donor server the
// federation profile points at.
func (s *ConnectionService) startXrayReceipts(ctx context.Context, target, nodeID string) {
	engine := s.receiptEngine
	if engine == nil || target == "" || nodeID == "" {
		return
	}
	stats, err := federation.DialXrayStats(target)
	if err != nil {
		s.runtimeLogf("receipts: xray stats dial failed: %v", err)
		return
	}
	s.mu.Lock()
	s.xrayStats = stats
	s.mu.Unlock()
	go func() {
		ticker := time.NewTicker(receiptSampleInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				stats.Close()
				s.mu.Lock()
				if s.xrayStats == stats {
					s.xrayStats = nil
				}
				s.mu.Unlock()
				return
			case <-ticker.C:
				up, down := stats.Uplink(), stats.Downlink()
				if up < 0 || down < 0 {
					continue // counters not ready; keep the prior sample
				}
				engine.Collect(nodeID, "xray", up, down)
			}
		}
	}()
}

// startVKTurnReceipts samples the relay's payload counters into the receipt engine until ctx
// is cancelled. nodeID is the VK TURN endpoint of the donor node.
func (s *ConnectionService) startVKTurnReceipts(ctx context.Context, nodeID string) {
	engine := s.receiptEngine
	if engine == nil || nodeID == "" {
		return
	}
	go func() {
		ticker := time.NewTicker(receiptSampleInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c, cancel := context.WithTimeout(ctx, 5*time.Second)
				up, down, err := s.manager.PayloadBytes(c)
				cancel()
				if err != nil {
					continue
				}
				engine.Collect(nodeID, "vktp", up, down)
			}
		}
	}()
}

// isFederationXray reports whether an xray profile was handed out by the federation
// subscription, the only profiles a receipt is signed for.
func isFederationXray(p config.XrayProfile) bool {
	return p.SubscriptionID == config.FederationSubscriptionID
}
