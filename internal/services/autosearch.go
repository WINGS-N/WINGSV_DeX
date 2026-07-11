package services

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/net/proxy"

	"github.com/WINGS-N/wingsv-dex/internal/byedpi"
	"github.com/WINGS-N/wingsv-dex/internal/config"
	"github.com/WINGS-N/wingsv-dex/internal/xray"
)

// Auto-search events. State drives the header/progress; profile rows stream in as each
// candidate is measured (status checking|ok|err); done marks the end.
const (
	AutoSearchStateEvent   = "autosearch:state"
	AutoSearchProfileEvent = "autosearch:profile"
	AutoSearchDoneEvent    = "autosearch:done"
)

const (
	autoSearchTCPingWorkers   = 5
	autoSearchDownloadWorkers = 6
	autoSearchDownloadURL     = "https://speed.cloudflare.com/__down?bytes="
)

// AutoSearchService probes the Xray subscription pool to find working profiles: it TCPings
// all candidates, then downloads a test file through the fastest ones until it finds the
// target number of stable profiles, then offers to apply the best.
type AutoSearchService struct {
	store   *config.Store
	exePath string
	subs    *SubscriptionService
	app     *application.App

	mu                sync.Mutex
	cancelFn          context.CancelFunc
	running           bool
	mode              string
	pendingResponsive []rankedCandidate
	foundIDs          []string
	bestID            string
	bestTitle         string
}

// NewAutoSearchService wires the finder to the store, executable path and subscription
// service (used to refresh the candidate pool before a run).
func NewAutoSearchService(store *config.Store, exePath string, subs *SubscriptionService) *AutoSearchService {
	return &AutoSearchService{store: store, exePath: exePath, subs: subs}
}

// SetApp attaches the app so progress can be streamed to the frontend.
func (s *AutoSearchService) SetApp(app *application.App) { s.app = app }

type rankedCandidate struct {
	p       config.XrayProfile
	latency int64
}

// AutoSearchState is the run header / progress snapshot.
type AutoSearchState struct {
	Phase     string `json:"phase"` // prepare|tcping|download|whitelist_wait|awaiting_apply|done|failed
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	Found     int    `json:"found"`
	Target    int    `json:"target"`
	Message   string `json:"message"`
}

// AutoSearchProfileRow is one candidate's streamed row.
type AutoSearchProfileRow struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Address   string `json:"address"`
	LatencyMs int64  `json:"latencyMs"`
	Status    string `json:"status"` // checking|ok|err
	Metric    string `json:"metric"`
}

// Settings returns the auto-search settings.
func (s *AutoSearchService) Settings() config.AutoSearchSettings { return s.store.AutoSearchSettings() }

// SetSettings persists the auto-search settings.
func (s *AutoSearchService) SetSettings(a config.AutoSearchSettings) (config.AutoSearchSettings, error) {
	if err := s.store.SetAutoSearchSettings(a); err != nil {
		return config.AutoSearchSettings{}, err
	}
	return s.store.AutoSearchSettings(), nil
}

// Start begins a run. mode is "standard" or "whitelist". When gateWhitelist is true and mode
// is whitelist, the run pauses after TCPing (phase whitelist_wait) so the user can switch to
// the whitelisted network, then Continue resumes it (used by onboarding).
func (s *AutoSearchService) Start(mode string, gateWhitelist bool) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.running = true
	s.cancelFn = cancel
	s.mode = mode
	s.foundIDs = nil
	s.bestID, s.bestTitle = "", ""
	s.mu.Unlock()
	go s.run(ctx, mode, gateWhitelist)
}

// Continue resumes a whitelist run that paused for the network switch.
func (s *AutoSearchService) Continue() {
	s.mu.Lock()
	resp := s.pendingResponsive
	mode := s.mode
	s.pendingResponsive = nil
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFn = cancel
	s.running = true
	s.mu.Unlock()
	if resp == nil {
		return
	}
	go func() {
		s.downloadAndFinish(ctx, mode, resp)
	}()
}

// Cancel stops a running search.
func (s *AutoSearchService) Cancel() {
	s.mu.Lock()
	if s.cancelFn != nil {
		s.cancelFn()
	}
	s.running = false
	s.mu.Unlock()
}

func (s *AutoSearchService) emitState(st AutoSearchState) {
	if s.app != nil {
		s.app.Event.Emit(AutoSearchStateEvent, st)
	}
}

func (s *AutoSearchService) emitRow(r AutoSearchProfileRow) {
	if s.app != nil {
		s.app.Event.Emit(AutoSearchProfileEvent, r)
	}
}

func (s *AutoSearchService) run(ctx context.Context, mode string, gateWhitelist bool) {
	set := s.store.AutoSearchSettings()
	s.emitState(AutoSearchState{Phase: "prepare", Target: set.TargetCount})

	// Refresh the candidate pool from all subscriptions.
	if s.subs != nil {
		s.subs.RefreshAll()
	}
	candidates := s.store.XrayList()
	if !set.UseBuiltInSubscription {
		filtered := candidates[:0]
		for _, c := range candidates {
			if c.SubscriptionID != config.AutoSearchSubscriptionID {
				filtered = append(filtered, c)
			}
		}
		candidates = filtered
	}
	if len(candidates) == 0 {
		s.finishFailed("нет профилей для проверки")
		return
	}

	responsive := s.tcpingPhase(ctx, set, candidates)
	if ctx.Err() != nil {
		s.finishFailed("отменено")
		return
	}
	if len(responsive) == 0 {
		s.finishFailed("ни один профиль не ответил")
		return
	}

	if gateWhitelist && mode == "whitelist" {
		s.mu.Lock()
		s.pendingResponsive = responsive
		s.running = false
		s.mu.Unlock()
		s.emitState(AutoSearchState{Phase: "whitelist_wait", Total: len(responsive), Target: set.TargetCount})
		return
	}
	s.downloadAndFinish(ctx, mode, responsive)
}

// tcpingPhase connects to every candidate with a worker pool, streams each row, and returns
// the responsive candidates sorted fastest-first.
func (s *AutoSearchService) tcpingPhase(ctx context.Context, set config.AutoSearchSettings, candidates []config.XrayProfile) []rankedCandidate {
	timeout := time.Duration(set.TCPingTimeoutMs) * time.Millisecond
	total := len(candidates)
	sem := make(chan struct{}, autoSearchTCPingWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var responsive []rankedCandidate
	completed := 0

	for _, c := range candidates {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(c config.XrayProfile) {
			defer wg.Done()
			defer func() { <-sem }()
			latency := autoTCPing(c, timeout)
			mu.Lock()
			completed++
			if latency >= 0 {
				responsive = append(responsive, rankedCandidate{p: c, latency: latency})
			}
			found := 0
			done := completed
			mu.Unlock()
			if latency >= 0 {
				s.emitRow(AutoSearchProfileRow{ID: c.ID, Title: c.Title, Address: c.Address, LatencyMs: latency, Status: "ok"})
			} else {
				s.emitRow(AutoSearchProfileRow{ID: c.ID, Title: c.Title, Address: c.Address, LatencyMs: -1, Status: "err", Metric: "Нет ответа"})
			}
			s.emitState(AutoSearchState{Phase: "tcping", Completed: done, Total: total, Found: found, Target: set.TargetCount})
		}(c)
	}
	wg.Wait()
	// Fastest first, but favorites are probed before everyone else (the final selection is
	// still by throughput in the download phase).
	sort.SliceStable(responsive, func(i, j int) bool {
		if responsive[i].p.Favorite != responsive[j].p.Favorite {
			return responsive[i].p.Favorite
		}
		return responsive[i].latency < responsive[j].latency
	})
	return responsive
}

func autoTCPing(p config.XrayProfile, timeout time.Duration) int64 {
	if p.Address == "" || p.Port == 0 {
		return -1
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(p.Address, strconv.Itoa(p.Port)), timeout)
	if err != nil {
		return -1
	}
	_ = conn.Close()
	ms := time.Since(start).Milliseconds()
	if ms < 1 {
		ms = 1
	}
	return ms
}

// downloadAndFinish runs the download phase (optionally through a temporary ByeDPI front for
// whitelist mode), tags the found profiles into the Автопоиск subscription and awaits apply.
func (s *AutoSearchService) downloadAndFinish(ctx context.Context, mode string, responsive []rankedCandidate) {
	set := s.store.AutoSearchSettings()

	byedpiFront := ""
	var stopByedpi func()
	if mode == "whitelist" {
		front, stop := s.startTempByeDPI()
		byedpiFront = front
		stopByedpi = stop
	}
	defer func() {
		if stopByedpi != nil {
			stopByedpi()
		}
	}()

	found := s.downloadPhase(ctx, set, responsive, byedpiFront)
	if ctx.Err() != nil {
		s.finishFailed("отменено")
		return
	}
	if len(found) == 0 {
		s.finishFailed("не найдено стабильных профилей")
		return
	}

	ids := make([]string, len(found))
	for i, f := range found {
		ids[i] = f.p.ID
	}
	s.store.TagAutoSearchProfiles(ids)

	s.mu.Lock()
	s.foundIDs = ids
	s.bestID = found[0].p.ID
	s.bestTitle = found[0].p.Title
	s.running = false
	s.mu.Unlock()

	s.emitState(AutoSearchState{
		Phase:   "awaiting_apply",
		Found:   len(found),
		Target:  set.TargetCount,
		Message: found[0].p.Title,
	})
}

// downloadPhase probes each responsive candidate with a worker pool, stopping once the
// target number of stable profiles is found; returns them ranked best-first by throughput.
func (s *AutoSearchService) downloadPhase(ctx context.Context, set config.AutoSearchSettings, responsive []rankedCandidate, byedpiFront string) []rankedCandidate {
	bin := xrayBinaryPath(s.exePath)
	total := len(responsive)
	sem := make(chan struct{}, autoSearchDownloadWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var found []rankedCandidate
	completed := 0
	stop := false

	for _, r := range responsive {
		mu.Lock()
		stopped := stop
		mu.Unlock()
		if stopped || ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(r rankedCandidate) {
			defer wg.Done()
			defer func() { <-sem }()
			s.emitRow(AutoSearchProfileRow{ID: r.p.ID, Title: r.p.Title, Address: r.p.Address, LatencyMs: r.latency, Status: "checking", Metric: "подключаемся..."})
			stable, speed := s.probeDownload(ctx, bin, r.p, byedpiFront, set)
			mu.Lock()
			completed++
			if stable {
				r.latency = speed // reuse latency slot for throughput ranking
				found = append(found, r)
				if len(found) >= set.TargetCount {
					// Stop dispatching new probes; in-flight ones drain naturally.
					stop = true
				}
			}
			foundN := len(found)
			done := completed
			mu.Unlock()
			if stable {
				s.emitRow(AutoSearchProfileRow{ID: r.p.ID, Title: r.p.Title, Address: r.p.Address, LatencyMs: r.latency, Status: "ok", Metric: "Проверка трафика OK"})
			} else {
				s.emitRow(AutoSearchProfileRow{ID: r.p.ID, Title: r.p.Title, Address: r.p.Address, LatencyMs: -1, Status: "err", Metric: "Проверка трафика не прошла"})
			}
			s.emitState(AutoSearchState{Phase: "download", Completed: done, Total: total, Found: foundN, Target: set.TargetCount})
		}(r)
	}
	wg.Wait()
	// Rank by throughput (bytes/s stored in latency slot) descending.
	sort.SliceStable(found, func(i, j int) bool { return found[i].latency > found[j].latency })
	if len(found) > set.TargetCount {
		found = found[:set.TargetCount]
	}
	return found
}

// probeDownload spins up a proxy-only xray for the profile and downloads the test file
// through it downloadAttempts times; stable = the target size is met on every attempt.
// Returns stability and the best observed bytes/sec.
func (s *AutoSearchService) probeDownload(ctx context.Context, bin string, p config.XrayProfile, byedpiFront string, set config.AutoSearchSettings) (bool, int64) {
	port, err := freePort()
	if err != nil {
		return false, 0
	}
	cfg, err := xray.ProbeConfig(bin, p.RawLink, port, byedpiFront)
	if err != nil {
		return false, 0
	}
	dir, err := os.MkdirTemp("", "wingsv-autosearch-")
	if err != nil {
		return false, 0
	}
	defer os.RemoveAll(dir)
	cfgPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		return false, 0
	}
	runPath := filepath.Join(dir, "run.json")
	if err := os.WriteFile(runPath, []byte(`{"configPath":"`+cfgPath+`"}`), 0o600); err != nil {
		return false, 0
	}
	cmd := exec.Command(bin, "run", "-config", runPath)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return false, 0
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	if !waitPort(port, 4*time.Second) {
		return false, 0
	}

	sizeBytes := int64(set.DownloadSizeMb) * 1024 * 1024
	tolerance := clampInt64(sizeBytes/50, 64*1024, 256*1024)
	perAttempt := time.Duration(set.DownloadTimeoutSeconds) * time.Second
	dialer, err := proxy.SOCKS5("tcp", "127.0.0.1:"+strconv.Itoa(port), nil, &net.Dialer{Timeout: perAttempt})
	if err != nil {
		return false, 0
	}

	stableRuns := 0
	var bestSpeed int64
	for attempt := 0; attempt < set.DownloadAttempts; attempt++ {
		if ctx.Err() != nil {
			return false, bestSpeed
		}
		n, dur, ok := downloadThroughProxy(dialer, sizeBytes, perAttempt)
		if ok && n >= sizeBytes-tolerance {
			stableRuns++
			if dur > 0 {
				if sp := n * int64(time.Second) / int64(dur); sp > bestSpeed {
					bestSpeed = sp
				}
			}
		}
		if attempt < set.DownloadAttempts-1 {
			select {
			case <-ctx.Done():
				return false, bestSpeed
			case <-time.After(3 * time.Second):
			}
		}
	}
	return stableRuns >= set.DownloadAttempts, bestSpeed
}

func downloadThroughProxy(dialer proxy.Dialer, sizeBytes int64, timeout time.Duration) (int64, time.Duration, bool) {
	tr := &http.Transport{DisableKeepAlives: true}
	if cd, ok := dialer.(proxy.ContextDialer); ok {
		tr.DialContext = cd.DialContext
	} else {
		tr.Dial = dialer.Dial
	}
	tr.TLSClientConfig = &tls.Config{}
	client := &http.Client{Transport: tr, Timeout: timeout}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, autoSearchDownloadURL+strconv.FormatInt(sizeBytes, 10), nil)
	if err != nil {
		return 0, 0, false
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return 0, 0, false
	}
	n, _ := io.Copy(io.Discard, io.LimitReader(resp.Body, sizeBytes))
	return n, time.Since(start), n > 0
}

// startTempByeDPI runs a short-lived no-auth ByeDPI on a free port and returns its SOCKS
// address plus a stopper, for whitelist-mode probing.
func (s *AutoSearchService) startTempByeDPI() (string, func()) {
	bin := helperBinaryPath(s.exePath, "byedpi")
	set := s.store.ByeDPISettings()
	port, err := freePort()
	if err != nil {
		return "", nil
	}
	set.ProxyIP = "127.0.0.1"
	set.ProxyPort = port
	set.AuthEnabled = false
	proc, err := byedpi.Start(bin, set)
	if err != nil {
		return "", nil
	}
	if !waitPort(port, 4*time.Second) {
		proc.Stop()
		return "", nil
	}
	return "127.0.0.1:" + strconv.Itoa(port), proc.Stop
}

// Apply applies the found configuration (best profile active, backend Xray, ByeDPI auto-start
// for whitelist) or leaves things as they were. The found profiles stay tagged either way.
func (s *AutoSearchService) Apply(apply bool) error {
	s.mu.Lock()
	bestID := s.bestID
	mode := s.mode
	s.mu.Unlock()
	if apply && bestID != "" {
		if err := s.store.SetNetworkBackend(config.BackendXray); err != nil {
			return err
		}
		if err := s.store.XrayActivate(bestID); err != nil {
			return err
		}
		if mode == "whitelist" {
			b := s.store.ByeDPISettings()
			b.Enabled = true
			_ = s.store.SetByeDPISettings(b)
		}
	}
	if s.app != nil {
		s.app.Event.Emit(AutoSearchDoneEvent, map[string]any{"applied": apply})
	}
	return nil
}

func (s *AutoSearchService) finishFailed(msg string) {
	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	s.emitState(AutoSearchState{Phase: "failed", Message: msg})
}

func clampInt64(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
