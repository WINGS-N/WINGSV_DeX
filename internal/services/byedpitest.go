package services

import (
	"crypto/tls"
	"io"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/net/proxy"

	"github.com/WINGS-N/wingsv-dex/internal/byedpi"
	"github.com/WINGS-N/wingsv-dex/internal/config"
)

// Strategy-finder events: one per strategy as it is measured, then a done marker.
const (
	ByeDpiStrategyResultEvent = "byedpi:strategy:result"
	ByeDpiStrategyDoneEvent   = "byedpi:strategy:done"
)

// ByeDpiStrategyService runs the ByeDPI strategy finder: each candidate strategy is started
// as a local ciadpi, and the probe targets are reached through its SOCKS5 port to see how
// many succeed. The best strategy can then be applied as the ByeDPI command.
type ByeDpiStrategyService struct {
	store   *config.Store
	exePath string
	app     *application.App
}

// NewByeDpiStrategyService wires the finder to the store and app executable path.
func NewByeDpiStrategyService(store *config.Store, exePath string) *ByeDpiStrategyService {
	return &ByeDpiStrategyService{store: store, exePath: exePath}
}

// SetApp attaches the app so results can be streamed to the frontend.
func (s *ByeDpiStrategyService) SetApp(app *application.App) { s.app = app }

// StrategyResult is one strategy's probe outcome.
type StrategyResult struct {
	Command string `json:"command"`
	Success int    `json:"success"`
	Total   int    `json:"total"`
	DelayMs int64  `json:"delayMs"`
}

// Start begins the finder in the background and returns the number of strategies to try, so
// the UI can show progress; results stream via ByeDpiStrategyResultEvent.
func (s *ByeDpiStrategyService) Start() int {
	settings := s.store.ByeDPISettings()
	strategies := byedpi.Strategies(settings)
	targets := byedpi.Targets(settings)
	bin := helperBinaryPath(s.exePath, "byedpi")
	go s.run(bin, settings, strategies, targets)
	return len(strategies)
}

// Apply sets the given strategy as the active ByeDPI command (command mode).
func (s *ByeDpiStrategyService) Apply(command string) error {
	b := s.store.ByeDPISettings()
	b.UseCommandSettings = true
	b.Command = strings.TrimSpace(command)
	return s.store.SetByeDPISettings(b)
}

func (s *ByeDpiStrategyService) run(bin string, settings config.ByeDPISettings, strategies, targets []string) {
	limit := settings.ProxyTestConcurrencyLimit
	if limit <= 0 {
		limit = 20
	}
	timeout := time.Duration(settings.ProxyTestTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for _, strat := range strategies {
		wg.Add(1)
		sem <- struct{}{}
		go func(strat string) {
			defer wg.Done()
			defer func() { <-sem }()
			succ, total, delay := s.testStrategy(bin, strat, targets, timeout)
			if s.app != nil {
				s.app.Event.Emit(ByeDpiStrategyResultEvent, StrategyResult{Command: strat, Success: succ, Total: total, DelayMs: delay})
			}
		}(strat)
	}
	wg.Wait()
	if s.app != nil {
		s.app.Event.Emit(ByeDpiStrategyDoneEvent, nil)
	}
}

// testStrategy spawns ciadpi with the strategy, probes each target through its SOCKS5 port,
// and returns the success count and total probe time.
func (s *ByeDpiStrategyService) testStrategy(bin, strat string, targets []string, timeout time.Duration) (int, int, int64) {
	total := len(targets)
	port, err := freePort()
	if err != nil {
		return 0, total, -1
	}
	args := append([]string{"-i127.0.0.1", "-p" + strconv.Itoa(port), "-I0.0.0.0"}, byedpi.Tokenize(strat)...)
	cmd := exec.Command(bin, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return 0, total, -1
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()

	if !waitPort(port, 2*time.Second) {
		return 0, total, -1
	}
	dialer, err := proxy.SOCKS5("tcp", "127.0.0.1:"+strconv.Itoa(port), nil, &net.Dialer{Timeout: timeout})
	if err != nil {
		return 0, total, -1
	}
	start := time.Now()
	succ := 0
	for _, host := range targets {
		if probeTarget(dialer, host, timeout) {
			succ++
		}
	}
	return succ, total, time.Since(start).Milliseconds()
}

// probeTarget dials the target's HTTPS port through the SOCKS proxy and completes a TLS
// handshake; success means the strategy defeated the DPI block for that host.
func probeTarget(dialer proxy.Dialer, host string, timeout time.Duration) bool {
	conn, err := dialer.Dial("tcp", host+":443")
	if err != nil {
		return false
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	tc := tls.Client(conn, &tls.Config{ServerName: host, InsecureSkipVerify: true})
	return tc.Handshake() == nil
}

// waitPort waits until a local TCP port accepts connections or the deadline passes.
func waitPort(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	addr := "127.0.0.1:" + strconv.Itoa(port)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}
