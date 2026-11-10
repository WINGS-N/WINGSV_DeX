package services

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/WINGS-N/wingsv-dex/internal/config"
	"github.com/WINGS-N/wingsv-dex/internal/xraysubs"
)

// SubscriptionsUpdatedEvent carries the fresh subscription list to the frontend after a
// background auto-update, so an open Subscriptions screen reflects it without polling.
const SubscriptionsUpdatedEvent = "subscriptions:updated"

// defaultRefreshMinutes is used when a subscription sets no explicit interval.
const defaultRefreshMinutes = 720

// SubscriptionService manages xray subscriptions: fetch, parse, and scheduled auto-update.
type SubscriptionService struct {
	store *config.Store
	app   *application.App
}

// NewSubscriptionService wires a SubscriptionService to the store.
func NewSubscriptionService(store *config.Store) *SubscriptionService {
	return &SubscriptionService{store: store}
}

// SetApp attaches the app so background refreshes can push events to the frontend.
// HwidView is the device-fingerprint settings plus the live automatic values, so the UI can
// show what would be sent next to each editable field.
type HwidView struct {
	Enabled         bool   `json:"enabled"`
	Manual          bool   `json:"manual"`
	HWID            string `json:"hwid"`
	DeviceOS        string `json:"deviceOs"`
	VerOS           string `json:"verOs"`
	DeviceModel     string `json:"deviceModel"`
	AutoHWID        string `json:"autoHwid"`
	AutoDeviceOS    string `json:"autoDeviceOs"`
	AutoVerOS       string `json:"autoVerOs"`
	AutoDeviceModel string `json:"autoDeviceModel"`
}

// HwidSettings returns the current device-fingerprint settings and the automatic values.
func (s *SubscriptionService) HwidSettings() HwidView {
	v := s.store.SubscriptionHwidSettings()
	a := config.DeviceFingerprint()
	return HwidView{
		Enabled: v.Enabled, Manual: v.Manual,
		HWID: v.HWID, DeviceOS: v.DeviceOS, VerOS: v.VerOS, DeviceModel: v.DeviceModel,
		AutoHWID: a.HWID, AutoDeviceOS: a.OS, AutoVerOS: a.OSVersion, AutoDeviceModel: a.Model,
	}
}

// SetHwidSettings persists the device-fingerprint settings.
func (s *SubscriptionService) SetHwidSettings(enabled, manual bool, hwid, deviceOs, verOs, deviceModel string) error {
	return s.store.SetSubscriptionHwidSettings(config.SubscriptionHwidSettings{
		Enabled:     enabled,
		Manual:      manual,
		HWID:        strings.TrimSpace(hwid),
		DeviceOS:    strings.TrimSpace(deviceOs),
		VerOS:       strings.TrimSpace(verOs),
		DeviceModel: strings.TrimSpace(deviceModel),
	})
}

func (s *SubscriptionService) SetApp(app *application.App) { s.app = app }

// SubscriptionRefresh is returned after a refresh so the UI can show what changed.
type SubscriptionRefresh struct {
	Subscriptions []config.Subscription `json:"subscriptions"`
	Updated       int                   `json:"updated"`
	Error         string                `json:"error,omitempty"`
}

// List returns the stored subscriptions.
func (s *SubscriptionService) List() []config.Subscription {
	return s.store.SubscriptionList()
}

// Add adds a subscription and immediately fetches it. mirrors - запасные адреса,
// по которым обновление пойдёт, если основной перестал отвечать.
func (s *SubscriptionService) Add(title, url string, mirrors []string, proxyURL string) (SubscriptionRefresh, error) {
	validatedProxyURL, err := config.ValidateSubscriptionProxyURL(proxyURL)
	if err != nil {
		return SubscriptionRefresh{}, err
	}
	sub, err := s.store.AddSubscription(title, url)
	if err != nil {
		return SubscriptionRefresh{}, err
	}
	if len(mirrors) > 0 {
		updated, merr := s.store.SetSubscriptionMirrors(sub.ID, mirrors)
		if merr != nil {
			return SubscriptionRefresh{}, merr
		}
		sub = updated
	}
	if err := s.store.SetSubscriptionProxy(sub.ID, validatedProxyURL); err != nil {
		return SubscriptionRefresh{}, err
	}
	for _, configured := range s.store.SubscriptionList() {
		if configured.ID == sub.ID {
			sub = configured
			break
		}
	}
	n, ferr := s.refresh(sub)
	res := SubscriptionRefresh{Subscriptions: s.store.SubscriptionList(), Updated: n}
	if ferr != nil {
		res.Error = ferr.Error()
	}
	return res, nil
}

// SetProxy edits the explicit HTTP proxy for an existing subscription.
func (s *SubscriptionService) SetProxy(id, proxyURL string) ([]config.Subscription, error) {
	if err := s.store.SetSubscriptionProxy(id, proxyURL); err != nil {
		return nil, err
	}
	return s.store.SubscriptionList(), nil
}

// Remove deletes a subscription and its nodes.
func (s *SubscriptionService) Remove(id string) ([]config.Subscription, error) {
	// The account subscription is owned by WINGS Account: it is removed only when the user
	// signs out (AccountService.Logout -> RemoveFederationSubscription), never by hand here.
	if id == config.FederationSubscriptionID {
		return s.store.SubscriptionList(), nil
	}
	if err := s.store.RemoveSubscription(id); err != nil {
		return nil, err
	}
	return s.store.SubscriptionList(), nil
}

// SetAutoUpdate toggles a subscription's auto-update flag.
func (s *SubscriptionService) SetAutoUpdate(id string, on bool) ([]config.Subscription, error) {
	if err := s.store.SetSubscriptionAutoUpdate(id, on); err != nil {
		return nil, err
	}
	return s.store.SubscriptionList(), nil
}

// Refresh fetches one subscription by id.
func (s *SubscriptionService) Refresh(id string) (SubscriptionRefresh, error) {
	for _, sub := range s.store.SubscriptionList() {
		if sub.ID == id {
			n, err := s.refresh(sub)
			res := SubscriptionRefresh{Subscriptions: s.store.SubscriptionList(), Updated: n}
			if err != nil {
				res.Error = err.Error()
			}
			return res, nil
		}
	}
	return SubscriptionRefresh{Subscriptions: s.store.SubscriptionList()}, nil
}

// RefreshAll fetches every subscription.
// maxParallelRefreshes ограничивает одновременные обновления: подписок может быть
// много, а каждая это сетевой запрос с таймаутом.
const maxParallelRefreshes = 6

// RefreshAll обновляет подписки параллельно - раньше десять подписок ждали друг
// друга по очереди. Запись узлов идёт в стор под его мьютексом, так что гонок нет.
func (s *SubscriptionService) RefreshAll() SubscriptionRefresh {
	subs := s.store.SubscriptionList()
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		total   int
		lastErr string
	)
	slots := make(chan struct{}, maxParallelRefreshes)
	for _, sub := range subs {
		wg.Add(1)
		go func(target config.Subscription) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			n, err := s.refresh(target)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				lastErr = err.Error()
				return
			}
			total += n
		}(sub)
	}
	wg.Wait()
	return SubscriptionRefresh{Subscriptions: s.store.SubscriptionList(), Updated: total, Error: lastErr}
}

// refresh fetches a single subscription and stores its nodes + quota.
func (s *SubscriptionService) refresh(sub config.Subscription) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	res, err := xraysubs.Fetch(ctx, sub, s.store.HwidForSubscription(sub.ID))
	if err != nil {
		return 0, err
	}
	q := res.Quota
	if err := s.store.ApplySubscriptionNodes(sub.ID, res.Nodes, q.Upload, q.Download, q.Total, q.Expire); err != nil {
		return 0, err
	}
	return len(res.Nodes), nil
}

// StartAutoUpdate launches the background scheduler: an initial due-refresh shortly after
// startup, then a periodic sweep that refreshes auto-update subscriptions whose interval
// has elapsed. It emits SubscriptionsUpdatedEvent whenever it refreshes anything.
func (s *SubscriptionService) StartAutoUpdate() {
	go func() {
		time.Sleep(20 * time.Second)
		s.sweep()
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			s.sweep()
		}
	}()
}

func (s *SubscriptionService) sweep() {
	now := time.Now().Unix()
	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		refreshed bool
	)
	slots := make(chan struct{}, maxParallelRefreshes)
	for _, sub := range s.store.SubscriptionList() {
		if !sub.AutoUpdate || !due(sub, now) {
			continue
		}
		wg.Add(1)
		go func(target config.Subscription) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			if _, err := s.refresh(target); err == nil {
				mu.Lock()
				refreshed = true
				mu.Unlock()
			}
		}(sub)
	}
	wg.Wait()
	if refreshed && s.app != nil {
		s.app.Event.Emit(SubscriptionsUpdatedEvent, s.store.SubscriptionList())
	}
}

// due reports whether a subscription's refresh interval has elapsed.
func due(sub config.Subscription, now int64) bool {
	if sub.LastUpdatedAt == 0 {
		return true
	}
	minutes := sub.RefreshIntervalMinutes
	if minutes <= 0 {
		minutes = defaultRefreshMinutes
	}
	return now-sub.LastUpdatedAt >= int64(minutes)*60
}
