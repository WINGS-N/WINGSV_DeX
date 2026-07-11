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

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/WINGS-N/wingsv-dex/internal/config"
	"github.com/WINGS-N/wingsv-dex/internal/xray"
)

// Test result events: one XrayTestResultEvent per node as its measurement lands (so the
// Profiles screen can fill badges in and run a per-row loader on the rest), then
// XrayTestDoneEvent when the whole run finishes.
const (
	XrayTestResultEvent = "xraytest:result"
	XrayTestDoneEvent   = "xraytest:done"
)

// XrayTestService measures per-node latency for the Profiles screen: TCPing (a raw TCP
// connect to the node) and Real Delay (an HTTP probe through the node's proxy via bin/xray).
type XrayTestService struct {
	store   *config.Store
	exePath string
	app     *application.App
}

// NewXrayTestService wires the test service to the store and app executable path.
func NewXrayTestService(store *config.Store, exePath string) *XrayTestService {
	return &XrayTestService{store: store, exePath: exePath}
}

// SetApp attaches the app so per-node results can be streamed to the frontend.
func (s *XrayTestService) SetApp(app *application.App) { s.app = app }

// PingResult is one node's measured delay in milliseconds; -1 means the test failed.
type PingResult struct {
	ID      string `json:"id"`
	DelayMs int64  `json:"delayMs"`
}

const probeURL = "https://www.gstatic.com/generate_204"

// Start kicks off a test ("tcping" | "real") in the background over every Xray node and
// returns the ids being tested so the UI can show a loader on each until its result event
// arrives. Results stream via XrayTestResultEvent; XrayTestDoneEvent marks completion.
func (s *XrayTestService) Start(kind string) []string {
	profiles := s.store.XrayList()
	ids := make([]string, len(profiles))
	for i, p := range profiles {
		ids[i] = p.ID
	}
	limit := 16
	measure := func(p config.XrayProfile) int64 { return tcping(p) }
	if kind == "real" {
		bin := xrayBinaryPath(s.exePath)
		limit = 8
		measure = func(p config.XrayProfile) int64 { return s.realDelay(bin, p) }
	}
	go s.run(profiles, limit, measure)
	return ids
}

// run fans the measure function out over all nodes with a concurrency cap, emitting each
// result as it completes and persisting the whole batch when the run finishes.
func (s *XrayTestService) run(profiles []config.XrayProfile, limit int, measure func(config.XrayProfile) int64) {
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	var mu sync.Mutex
	records := make(map[string]config.PingRecord, len(profiles))
	for _, p := range profiles {
		wg.Add(1)
		sem <- struct{}{}
		go func(p config.XrayProfile) {
			defer wg.Done()
			defer func() { <-sem }()
			delay := measure(p)
			mu.Lock()
			records[p.ID] = config.PingRecord{Success: delay >= 0, LatencyMs: int(max64(delay, 0))}
			mu.Unlock()
			if s.app != nil {
				s.app.Event.Emit(XrayTestResultEvent, PingResult{ID: p.ID, DelayMs: delay})
			}
		}(p)
	}
	wg.Wait()
	_ = s.store.SetXrayPingResults(records)
	if s.app != nil {
		s.app.Event.Emit(XrayTestDoneEvent, nil)
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
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
