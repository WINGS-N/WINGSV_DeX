package services

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

var avatarNameRe = regexp.MustCompile(`^[A-Za-z0-9-]{1,39}$`)

// avatarTTL is how long a cached avatar is served without touching the network. After it
// the entry is revalidated with a conditional request so a changed avatar is picked up.
const avatarTTL = 12 * time.Hour

// AvatarService fetches GitHub avatars once and caches them on disk. The WebKitGTK image
// cache is not persistent, so without this the About screen re-downloads every avatar on
// each open; the disk cache mirrors the Android GithubAvatarLoader.
type AvatarService struct {
	dir string
	mu  sync.Mutex
}

// NewAvatarService prepares the on-disk avatar cache under the app config directory.
func NewAvatarService(configDir string) *AvatarService {
	dir := filepath.Join(configDir, "github_avatars")
	_ = os.MkdirAll(dir, 0o755)
	return &AvatarService{dir: dir}
}

// Get returns a data URL for the user's GitHub avatar. A fresh cache entry is served
// straight from disk; a stale one is revalidated (If-Modified-Since) so a changed avatar
// is refreshed, falling back to the stale copy when offline. Empty string on total failure
// (the frontend shows initials then).
func (a *AvatarService) Get(username string) string {
	if !avatarNameRe.MatchString(username) {
		return ""
	}
	path := filepath.Join(a.dir, username+".png")
	info, statErr := os.Stat(path)
	cached, readErr := os.ReadFile(path)
	hasCache := statErr == nil && readErr == nil && len(cached) > 0
	if hasCache && time.Since(info.ModTime()) < avatarTTL {
		return dataURL("image/png", cached)
	}

	var since time.Time
	if hasCache {
		since = info.ModTime()
	}
	ct, b, notModified, err := a.download(username, since)
	if err != nil {
		if hasCache {
			return dataURL("image/png", cached)
		}
		return ""
	}
	if notModified {
		now := time.Now()
		_ = os.Chtimes(path, now, now)
		return dataURL("image/png", cached)
	}
	a.mu.Lock()
	_ = os.WriteFile(path, b, 0o644)
	a.mu.Unlock()
	return dataURL(ct, b)
}

func (a *AvatarService) download(username string, since time.Time) (string, []byte, bool, error) {
	url := fmt.Sprintf("https://github.com/%s.png?size=128", username)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", nil, false, err
	}
	req.Header.Set("User-Agent", "WINGSV-Dex")
	if !since.IsZero() {
		req.Header.Set("If-Modified-Since", since.UTC().Format(http.TimeFormat))
	}
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return "", nil, true, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, false, fmt.Errorf("avatar: status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", nil, false, err
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/png"
	}
	return ct, b, false, nil
}

func dataURL(contentType string, b []byte) string {
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(b)
}
