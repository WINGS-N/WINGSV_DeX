//go:build linux

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func packageInstall(inst Install, file string) error {
	l := strings.ToLower(file)
	switch {
	case strings.HasSuffix(l, ".deb"):
		return runPkexec("dpkg", "-i", file)
	case strings.HasSuffix(l, ".rpm"):
		return runPkexec("rpm", "-U", "--force", file)
	}
	return fmt.Errorf("updater: unsupported package %s", filepath.Base(file))
}

func replaceExe(inst Install, newBin string) error {
	return installInPlace(inst.ExePath, newBin)
}

func replacePlain(inst Install, path, newBin string) error {
	return installInPlace(path, newBin)
}

// installInPlace swaps target for newBin. In a writable dir it stages in the same
// directory and renames over the target, so a running ELF keeps its old inode; otherwise
// it elevates via pkexec (unlink first, then copy, for the same reason).
func installInPlace(target, newBin string) error {
	dir := filepath.Dir(target)
	if dirWritable(dir) {
		staged := filepath.Join(dir, "."+filepath.Base(target)+".new")
		if err := copyFile(newBin, staged, 0o755); err != nil {
			return err
		}
		if err := os.Rename(staged, target); err != nil {
			_ = os.Remove(staged)
			return err
		}
		return nil
	}
	return runPkexec("sh", "-c", fmt.Sprintf("rm -f %q && cp %q %q && chmod 755 %q", target, newBin, target, target))
}

func runPkexec(args ...string) error {
	out, err := exec.Command("pkexec", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("pkexec %s: %v: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Restart launches the (now updated) executable detached from this process.
func Restart(inst Install) error {
	cmd := exec.Command(inst.ExePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}
