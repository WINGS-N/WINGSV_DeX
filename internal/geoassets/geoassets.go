// Package geoassets держит файлы geoip.dat и geosite.dat, без которых правила
// маршрутизации geoip: и geosite: молча ничего не матчат: ядро просто не находит
// базу и пропускает правило.
package geoassets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Имена те же, что ищет ядро в каталоге ассетов.
const (
	GeoIPFile   = "geoip.dat"
	GeoSiteFile = "geosite.dat"
)

// Источники по умолчанию - релизы Loyalsoldier, их же берёт приложение.
const (
	DefaultGeoIPURL   = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat"
	DefaultGeoSiteURL = "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat"
)

// Store - каталог с базами.
type Store struct {
	dir string
}

// New заводит хранилище в подкаталоге geo каталога конфигурации.
func New(configDir string) (*Store, error) {
	dir := filepath.Join(configDir, "geo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// Dir - каталог, который уезжает ядру как расположение ассетов.
func (s *Store) Dir() string { return s.dir }

// Status - что из баз лежит на диске.
type Status struct {
	GeoIP   FileStatus `json:"geoip"`
	GeoSite FileStatus `json:"geosite"`
}

// FileStatus - состояние одной базы.
type FileStatus struct {
	Present     bool   `json:"present"`
	Bytes       int64  `json:"bytes"`
	UpdatedUnix int64  `json:"updatedUnix"`
	Name        string `json:"name"`
}

// Status читает состояние обеих баз.
func (s *Store) Status() Status {
	return Status{GeoIP: s.statusOf(GeoIPFile), GeoSite: s.statusOf(GeoSiteFile)}
}

func (s *Store) statusOf(name string) FileStatus {
	out := FileStatus{Name: name}
	info, err := os.Stat(filepath.Join(s.dir, name))
	if err != nil {
		return out
	}
	out.Present = info.Size() > 0
	out.Bytes = info.Size()
	out.UpdatedUnix = info.ModTime().Unix()
	return out
}

// Ready - обе базы на месте, значит гео-правила отработают.
func (s *Store) Ready() bool {
	status := s.Status()
	return status.GeoIP.Present && status.GeoSite.Present
}

// Download качает одну базу и подменяет файл атомарно: ядро может читать каталог
// прямо сейчас, и недокачанный файл сломал бы ему разбор.
func (s *Store) Download(ctx context.Context, name, url string) error {
	if name != GeoIPFile && name != GeoSiteFile {
		return fmt.Errorf("geoassets: unknown file %q", name)
	}
	if url == "" {
		switch name {
		case GeoIPFile:
			url = DefaultGeoIPURL
		default:
			url = DefaultGeoSiteURL
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "WINGSV-DeX")
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("geoassets: %s answered %s", url, resp.Status)
	}

	tmp, err := os.CreateTemp(s.dir, name+".*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()
	written, err := io.Copy(tmp, io.LimitReader(resp.Body, 128<<20))
	if err != nil {
		return err
	}
	if written == 0 {
		return errors.New("geoassets: empty body")
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(s.dir, name))
}

// Import копирует уже лежащий у человека файл: сеть бывает недоступна, а базу
// можно принести руками.
func (s *Store) Import(name, sourcePath string) error {
	if name != GeoIPFile && name != GeoSiteFile {
		return fmt.Errorf("geoassets: unknown file %q", name)
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return errors.New("geoassets: source file is empty")
	}
	tmp, err := os.CreateTemp(s.dir, name+".*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()
	if _, err := io.Copy(tmp, source); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, filepath.Join(s.dir, name))
}
