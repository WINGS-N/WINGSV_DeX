package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/WINGS-N/wingsv-dex/internal/config"
	"github.com/WINGS-N/wingsv-dex/internal/geoassets"
)

// RoutingService отдаёт фронту правила маршрутизации Xray и базы geoip/geosite,
// без которых правила geoip: и geosite: ничего не матчат.
type RoutingService struct {
	store *config.Store
	geo   *geoassets.Store
}

// NewRoutingService wires the service to the store and the geo asset directory.
func NewRoutingService(store *config.Store, geo *geoassets.Store) *RoutingService {
	return &RoutingService{store: store, geo: geo}
}

// RoutingState - всё, что рисует экран маршрутизации за один запрос.
type RoutingState struct {
	Rules   []config.XrayRoutingRule `json:"rules"`
	Sources config.XrayGeoSources    `json:"sources"`
	Geo     geoassets.Status         `json:"geo"`
	// Defaults показываем в подсказке поля, когда свой адрес не задан
	DefaultGeoIPURL   string `json:"defaultGeoipUrl"`
	DefaultGeoSiteURL string `json:"defaultGeositeUrl"`
	AutoUpdate        bool   `json:"autoUpdate"`
}

// State собирает правила, источники и состояние баз.
func (s *RoutingService) State() RoutingState {
	out := RoutingState{
		Rules:             s.store.XrayRoutingRules(),
		Sources:           s.store.XrayGeoSources(),
		DefaultGeoIPURL:   geoassets.DefaultGeoIPURL,
		DefaultGeoSiteURL: geoassets.DefaultGeoSiteURL,
	}
	out.AutoUpdate = out.Sources.AutoUpdate
	if s.geo != nil {
		out.Geo = s.geo.Status()
	}
	return out
}

// SetRules заменяет весь набор правил: порядок значим, Xray берёт первое совпавшее.
func (s *RoutingService) SetRules(rules []config.XrayRoutingRule) (RoutingState, error) {
	if _, err := s.store.SetXrayRoutingRules(rules); err != nil {
		return RoutingState{}, err
	}
	return s.State(), nil
}

// SetSources сохраняет свои адреса баз.
func (s *RoutingService) SetSources(sources config.XrayGeoSources) (RoutingState, error) {
	if _, err := s.store.SetXrayGeoSources(sources); err != nil {
		return RoutingState{}, err
	}
	return s.State(), nil
}

// SetGeoAutoUpdate включает или выключает автоматическое обновление баз.
func (s *RoutingService) SetGeoAutoUpdate(on bool) (RoutingState, error) {
	sources := s.store.XrayGeoSources()
	sources.AutoUpdate = on
	if _, err := s.store.SetXrayGeoSources(sources); err != nil {
		return RoutingState{}, err
	}
	if on {
		go s.refreshStale()
	}
	return s.State(), nil
}

// StartAutoUpdate держит базы свежими: разово вскоре после запуска, дальше раз в
// сутки. Молчаливый: качать базы это фон, и ругаться человеку тут не на что.
func (s *RoutingService) StartAutoUpdate() {
	go func() {
		time.Sleep(25 * time.Second)
		s.refreshStale()
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			s.refreshStale()
		}
	}()
}

// geoRefreshEvery - как часто перекачивать базы. Сутки: они меняются раз в день,
// и дёргать зеркало чаще незачем.
const geoRefreshEvery = 24 * time.Hour

func (s *RoutingService) refreshStale() {
	if s.geo == nil {
		return
	}
	sources := s.store.XrayGeoSources()
	if !sources.AutoUpdate {
		return
	}
	now := time.Now()
	// Нет баз вовсе - качаем сразу, иначе ждём свой срок
	if s.geo.Ready() && now.Unix()-sources.LastCheckUnix < int64(geoRefreshEvery.Seconds()) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	for _, kind := range []string{"geoip", "geosite"} {
		name, url, err := s.resolveGeo(kind)
		if err != nil {
			continue
		}
		_ = s.geo.Download(ctx, name, url)
	}
	sources.LastCheckUnix = now.Unix()
	_, _ = s.store.SetXrayGeoSources(sources)
}

// DownloadGeo качает одну базу: "geoip" или "geosite". Берёт свой адрес, если он
// задан, иначе источник по умолчанию.
func (s *RoutingService) DownloadGeo(kind string) (RoutingState, error) {
	if s.geo == nil {
		return RoutingState{}, errors.New("каталог гео-баз недоступен")
	}
	name, url, err := s.resolveGeo(kind)
	if err != nil {
		return RoutingState{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := s.geo.Download(ctx, name, url); err != nil {
		return RoutingState{}, err
	}
	return s.State(), nil
}

// ImportGeo берёт базу с диска: сеть бывает недоступна, а файл можно принести руками.
func (s *RoutingService) ImportGeo(kind, path string) (RoutingState, error) {
	if s.geo == nil {
		return RoutingState{}, errors.New("каталог гео-баз недоступен")
	}
	name, _, err := s.resolveGeo(kind)
	if err != nil {
		return RoutingState{}, err
	}
	if strings.TrimSpace(path) == "" {
		return RoutingState{}, errors.New("не выбран файл")
	}
	if err := s.geo.Import(name, path); err != nil {
		return RoutingState{}, err
	}
	return s.State(), nil
}

func (s *RoutingService) resolveGeo(kind string) (name, url string, err error) {
	sources := s.store.XrayGeoSources()
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "geoip":
		return geoassets.GeoIPFile, sources.GeoIPURL, nil
	case "geosite":
		return geoassets.GeoSiteFile, sources.GeoSiteURL, nil
	default:
		return "", "", errors.New("неизвестная база: " + kind)
	}
}
