// Package updater implements the in-app self-update: it works out how the app was
// installed (deb, rpm, AppImage, Windows setup/portable, or a plain binary), checks the
// GitHub releases for a newer version, and installs the matching asset in place - the app
// binary and the vkturn child alongside it - then restarts.
package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	repo         = "WINGS-N/WINGSV_Dex"
	releasesAPI  = "https://api.github.com/repos/" + repo + "/releases/latest"
	ReleasesPage = "https://github.com/WINGS-N/WINGSV_DeX/releases"
)

// Kind is how the running app was installed, which decides the update mechanism.
type Kind string

const (
	KindDeb             Kind = "deb"
	KindRPM             Kind = "rpm"
	KindAppImage        Kind = "appimage"
	KindWindowsSetup    Kind = "windows-setup"
	KindWindowsPortable Kind = "windows-portable"
	KindBinary          Kind = "binary" // a plain ELF/exe not owned by a package manager
)

// Install describes where and how this instance runs.
type Install struct {
	Kind       Kind
	ExePath    string
	VkturnPath string
	Dir        string
	Arch       string
	OS         string
}

// Asset is one downloadable file from a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// Result is what the About screen renders and what Apply consumes.
type Result struct {
	Status   string `json:"status"` // uptodate | available | error
	Current  string `json:"current"`
	Latest   string `json:"latest"`
	Kind     Kind   `json:"kind"`
	AppAsset *Asset `json:"appAsset"`
	PageURL  string `json:"pageUrl"`
	Error    string `json:"error,omitempty"`
}

// Detect works out the install kind, exe/vkturn paths and target arch.
func Detect() Install {
	exe, err := os.Executable()
	if err == nil {
		if resolved, rerr := filepath.EvalSymlinks(exe); rerr == nil {
			exe = resolved
		}
	}
	kind := detectKind(exe)
	// A running AppImage reports its extracted AppRun as the executable; the file to
	// replace is the .AppImage itself, exposed via $APPIMAGE.
	if kind == KindAppImage {
		if ai := os.Getenv("APPIMAGE"); ai != "" {
			exe = ai
		}
	}
	dir := filepath.Dir(exe)
	vk := filepath.Join(dir, "vkturn")
	if runtime.GOOS == "windows" {
		vk += ".exe"
	}
	return Install{
		Kind:       kind,
		ExePath:    exe,
		Dir:        dir,
		VkturnPath: vk,
		Arch:       runtime.GOARCH,
		OS:         runtime.GOOS,
	}
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

// Check queries the latest release and, if it is newer than current, selects the assets
// matching this install kind (the app) and arch (vkturn).
func Check(current string) Result {
	inst := Detect()
	res := Result{Status: "uptodate", Current: current, Latest: current, Kind: inst.Kind, PageURL: ReleasesPage}

	rel, err := fetchLatest()
	if err != nil {
		res.Status = "error"
		res.Error = err.Error()
		return res
	}
	if rel == nil {
		return res // no published release -> treat as up to date
	}
	if rel.HTMLURL != "" {
		res.PageURL = rel.HTMLURL
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	res.Latest = latest
	if !newer(latest, current) {
		return res
	}
	res.Status = "available"
	res.AppAsset = pickAppAsset(inst, rel)
	return res
}

// httpClient returns a client whose sockets are created fresh per request (no connection
// reuse), so when the tunnel is active the update traffic exits through it instead of an
// idle connection pinned to the physical link.
func httpClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{DisableKeepAlives: true, Proxy: http.ProxyFromEnvironment},
	}
}

func fetchLatest() (*ghRelease, error) {
	req, err := http.NewRequest(http.MethodGet, releasesAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "WINGSV-Dex")
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := httpClient(10 * time.Second).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("releases: status %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// archTokens returns the arch strings that can appear in asset names.
func archTokens(arch string) []string {
	switch arch {
	case "amd64":
		return []string{"amd64", "x86_64", "x64"}
	case "arm64":
		return []string{"arm64", "aarch64"}
	}
	return []string{arch}
}

func hasArch(name, arch string) bool {
	l := strings.ToLower(name)
	for _, t := range archTokens(arch) {
		if strings.Contains(l, t) {
			return true
		}
	}
	return false
}

func pickAppAsset(inst Install, rel *ghRelease) *Asset {
	match := func(pred func(name string) bool) *Asset {
		for _, a := range rel.Assets {
			if strings.Contains(strings.ToLower(a.Name), "vkturn") {
				continue
			}
			if pred(a.Name) && hasArch(a.Name, inst.Arch) {
				return &Asset{Name: a.Name, URL: a.BrowserDownloadURL, Size: a.Size}
			}
		}
		return nil
	}
	ends := func(suffix string) func(string) bool {
		return func(n string) bool { return strings.HasSuffix(strings.ToLower(n), suffix) }
	}
	switch inst.Kind {
	case KindDeb:
		return match(ends(".deb"))
	case KindRPM:
		return match(ends(".rpm"))
	case KindAppImage:
		return match(ends(".appimage"))
	case KindWindowsSetup:
		// The NSIS setup is a universal (both-arch) installer, so its name carries no arch.
		for _, a := range rel.Assets {
			l := strings.ToLower(a.Name)
			if strings.Contains(l, "setup") && (strings.HasSuffix(l, ".exe") || strings.HasSuffix(l, ".msi")) {
				return &Asset{Name: a.Name, URL: a.BrowserDownloadURL, Size: a.Size}
			}
		}
		return nil
	case KindWindowsPortable:
		return match(func(n string) bool {
			l := strings.ToLower(n)
			return strings.Contains(l, "portable") || strings.HasSuffix(l, ".zip")
		})
	default: // KindBinary: a raw linux binary or a tar/zip carrying it
		return match(func(n string) bool {
			l := strings.ToLower(n)
			if strings.HasSuffix(l, ".deb") || strings.HasSuffix(l, ".rpm") || strings.HasSuffix(l, ".appimage") {
				return false
			}
			return strings.Contains(l, "linux")
		})
	}
}

// newer reports whether version a is strictly greater than b (dotted numeric compare).
func newer(a, b string) bool {
	pa, pb := parseVer(a), parseVer(b)
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func parseVer(v string) []int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	fields := strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '-' || r == '+' })
	out := make([]int, 0, len(fields))
	for _, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}
