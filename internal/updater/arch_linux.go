package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func buildArchPackage(inst Install, latest string, cb func(Progress)) error {
	switch inst.Package {
	case "wingsv-dex", "wingsv-dex-bin", "wingsv-dex-git":
	default:
		return fmt.Errorf("unsupported Arch package owner: %s", inst.Package)
	}
	if os.Geteuid() == 0 {
		return fmt.Errorf("makepkg must run as the desktop user")
	}
	tmp, err := os.MkdirTemp("", "wingsv-dex-makepkg-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	log, err := os.Create(filepath.Join(tmp, "build.log"))
	if err != nil {
		return err
	}
	defer log.Close()
	run := func(dir, name string, args ...string) error {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Stdout, cmd.Stderr = log, log
		cmd.Env = append(os.Environ(), "PKGDEST="+tmp, "GIT_TERMINAL_PROMPT=0")
		if err := cmd.Run(); err != nil {
			info, statErr := log.Stat()
			if statErr != nil {
				return err
			}
			tail := make([]byte, min(info.Size(), 2048))
			_, _ = log.ReadAt(tail, info.Size()-int64(len(tail)))
			return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(tail)))
		}
		return nil
	}
	if err := run(tmp, "git", "clone", "--depth=1", "--filter=blob:none", "--sparse", "--branch", "main", archSource, "repo"); err != nil {
		return err
	}
	checkout := filepath.Join(tmp, "repo")
	recipe := filepath.Join("build", "linux", "aur", inst.Package)
	if err := run(checkout, "git", "sparse-checkout", "set", "build/linux/aur"); err != nil {
		return err
	}
	dir := filepath.Join(checkout, recipe)
	if inst.Package != "wingsv-dex-git" {
		info := exec.Command("makepkg", "--printsrcinfo")
		info.Dir = dir
		out, err := info.Output()
		if err != nil {
			return fmt.Errorf("read project PKGBUILD: %w", err)
		}
		if !strings.Contains(string(out), "\n\tpkgver = "+latest+"\n") {
			if err := run(checkout, "bash", "build/linux/aur/update-pkgbuild.sh", latest); err != nil {
				return err
			}
		}
	}
	conf := filepath.Join(tmp, "makepkg.conf")
	// Load the same system and user settings as an ordinary makepkg invocation,
	// then use the desktop polkit agent for dependency installation.
	if err := os.WriteFile(conf, []byte("source_makepkg_config /etc/makepkg.conf\nPACMAN_AUTH=(pkexec)\n"), 0600); err != nil {
		return err
	}
	if err := run(dir, "makepkg", "--config", conf, "--syncdeps", "--noconfirm", "--nocolor"); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(tmp, "*.pkg.tar*"))
	if err != nil {
		return err
	}
	for _, file := range files {
		out, err := exec.Command("pacman", "-Qp", "--", file).Output()
		fields := strings.Fields(string(out))
		if err != nil || len(fields) != 2 || fields[0] != inst.Package {
			continue
		}
		version, _, _ := strings.Cut(fields[1], "-")
		if inst.Package == "wingsv-dex-git" {
			_, rev, _ := strings.Cut(version, ".g")
			if !strings.HasPrefix(rev, latest) {
				return fmt.Errorf("built revision %s differs from %s; check for updates again", version, latest)
			}
		} else if version != latest {
			return fmt.Errorf("the project PKGBUILD produced %s, expected %s", version, latest)
		}
		cb(Progress{Phase: "install"})
		return packageInstall(inst, file)
	}
	return fmt.Errorf("makepkg did not produce package %s", inst.Package)
}
