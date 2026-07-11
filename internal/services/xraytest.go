package services

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/WINGS-N/wingsv-dex/internal/config"
	"github.com/WINGS-N/wingsv-dex/internal/xray"
)

// XrayTestService measures per-node latency for the Profiles screen: TCPing (a raw TCP
// connect to the node) and Real Delay (an HTTP probe through the node's proxy via bin/xray).
type XrayTestService struct {
	store   *config.Store
	exePath string
}

// NewXrayTestService wires the test service to the store and app executable path.
func NewXrayTestService(store *config.Store, exePath string) *XrayTestService {
	return &XrayTestService{store: store, exePath: exePath}
}

// PingResult is one node's measured delay in milliseconds; -1 means the test failed.
type PingResult struct {
	ID      string `json:"id"`
	DelayMs int64  `json:"delayMs"`
}

const probeURL = "https://www.gstatic.com/generate_204"

// TCPingAll connects to every Xray node's address:port and reports the connect time.
func (s *XrayTestService) TCPingAll() []PingResult {
	return s.runAll(16, func(p config.XrayProfile) int64 { return tcping(p) })
}

// RealDelayAll measures the real HTTP round-trip through each node's proxy. It is heavier
// than TCPing (each probe spins up an xray instance), so concurrency is lower.
func (s *XrayTestService) RealDelayAll() []PingResult {
	bin := xrayBinaryPath(s.exePath)
	return s.runAll(8, func(p config.XrayProfile) int64 { return s.realDelay(bin, p) })
}

// runAll fans the measure function out over all Xray profiles with a concurrency cap.
func (s *XrayTestService) runAll(limit int, measure func(config.XrayProfile) int64) []PingResult {
	profiles := s.store.XrayList()
	results := make([]PingResult, len(profiles))
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i, p := range profiles {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, p config.XrayProfile) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = PingResult{ID: p.ID, DelayMs: measure(p)}
		}(i, p)
	}
	wg.Wait()
	return results
}

func tcping(p config.XrayProfile) int64 {
	if p.Address == "" || p.Port == 0 {
		return -1
	}
	addr := net.JoinHostPort(p.Address, strconv.Itoa(p.Port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return -1
	}
	_ = conn.Close()
	return time.Since(start).Milliseconds()
}

func (s *XrayTestService) realDelay(bin string, p config.XrayProfile) int64 {
	port, err := freePort()
	if err != nil {
		return -1
	}
	cfg, err := xray.ProbeConfig(bin, p.RawLink, port)
	if err != nil {
		return -1
	}
	dir, err := os.MkdirTemp("", "wingsv-probe-")
	if err != nil {
		return -1
	}
	defer os.RemoveAll(dir)
	cfgPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		return -1
	}
	cmd := exec.Command(bin, "ping",
		"-config", cfgPath,
		"-timeout", "5",
		"-url", probeURL,
		"-proxy", "socks5://127.0.0.1:"+strconv.Itoa(port),
	)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return -1
	}
	delay, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil || delay < 0 {
		return -1
	}
	return delay
}

// freePort asks the OS for an unused loopback TCP port for a probe's SOCKS listener.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
