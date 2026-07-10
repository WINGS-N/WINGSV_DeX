package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Progress is streamed to the UI while an update runs.
type Progress struct {
	Phase string `json:"phase"` // download | download-vk | install
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

// Apply downloads the selected assets and installs them in place: package installs go
// through the package manager, loose kinds (plain binary, AppImage, Windows portable) swap
// the executable at its current path and refresh the vkturn child beside it.
func Apply(res Result, cb func(Progress)) error {
	if res.AppAsset == nil {
		return errors.New("no matching release asset for this install")
	}
	inst := Detect()
	tmp, err := os.MkdirTemp("", "wingsv-dex-update-*")
	if err != nil {
		return err
	}

	appFile := filepath.Join(tmp, res.AppAsset.Name)
	if err := download(res.AppAsset.URL, appFile, func(d, t int64) { cb(Progress{Phase: "download", Done: d, Total: t}) }); err != nil {
		return err
	}

	cb(Progress{Phase: "install"})
	switch inst.Kind {
	case KindDeb, KindRPM, KindWindowsSetup:
		return packageInstall(inst, appFile)
	default:
		// Loose kinds ship a single archive holding the app, the vkturn child and (on
		// Windows) wintun.dll; a raw binary asset carries just the app.
		if !isArchive(appFile) {
			return replaceExe(inst, appFile)
		}
		appBin, ok := extractFromArchive(appFile, filepath.Base(inst.ExePath))
		if !ok {
			return fmt.Errorf("updater: %s not found in %s", filepath.Base(inst.ExePath), res.AppAsset.Name)
		}
		if err := replaceExe(inst, appBin); err != nil {
			return err
		}
		if vkBin, ok := extractFromArchive(appFile, filepath.Base(inst.VkturnPath)); ok {
			_ = replacePlain(inst, inst.VkturnPath, vkBin)
		}
		if inst.OS == "windows" {
			if dll, ok := extractFromArchive(appFile, "wintun.dll"); ok {
				_ = replacePlain(inst, filepath.Join(inst.Dir, "wintun.dll"), dll)
			}
		}
		return nil
	}
}

func download(url, dest string, cb func(done, total int64)) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "WINGSV-Dex")
	resp, err := httpClient(5 * time.Minute).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download: status %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	total := resp.ContentLength
	var done int64
	buf := make([]byte, 64*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			if cb != nil {
				cb(done, total)
			}
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

func isArchive(p string) bool {
	l := strings.ToLower(p)
	return strings.HasSuffix(l, ".tar.gz") || strings.HasSuffix(l, ".tgz") || strings.HasSuffix(l, ".zip")
}

// extractFromArchive pulls the entry whose base name matches want (ignoring a .exe suffix)
// out of a tar.gz/zip to a temp file. The bool reports whether it was found.
func extractFromArchive(archivePath, want string) (string, bool) {
	var (
		path string
		err  error
	)
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		path, err = extractZip(archivePath, want)
	} else {
		path, err = extractTarGz(archivePath, want)
	}
	if err != nil {
		return "", false
	}
	return path, true
}

func stem(name string) string {
	return strings.TrimSuffix(strings.ToLower(filepath.Base(name)), ".exe")
}

func extractTarGz(archivePath, want string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if h.Typeflag == tar.TypeReg && stem(h.Name) == stem(want) {
			return writeTemp(want, tr)
		}
	}
	return "", fmt.Errorf("archive: %s not found", want)
}

func extractZip(archivePath, want string) (string, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() || stem(zf.Name) != stem(want) {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		return writeTemp(want, rc)
	}
	return "", fmt.Errorf("archive: %s not found", want)
}

func writeTemp(name string, r io.Reader) (string, error) {
	out, err := os.CreateTemp("", "wingsv-dex-bin-*-"+filepath.Base(name))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return "", err
	}
	out.Close()
	return out.Name(), nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".wr-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	_ = os.Remove(name)
	return true
}
