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
}

// State собирает правила, источники и состояние баз.
func (s *RoutingService) State() RoutingState {
	out := RoutingState{
		Rules:             s.store.XrayRoutingRules(),
		Sources:           s.store.XrayGeoSources(),
		DefaultGeoIPURL:   geoassets.DefaultGeoIPURL,
		DefaultGeoSiteURL: geoassets.DefaultGeoSiteURL,
	}
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
