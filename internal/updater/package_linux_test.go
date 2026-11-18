package updater

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPackageInstallUsesPolkit(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "args")
	t.Setenv("ARGS_LOG", log)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	if err := os.WriteFile(filepath.Join(dir, "pkexec"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$ARGS_LOG\"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		kind Kind
		file string
		want string
	}{
		{KindDeb, "/tmp/a b.deb", "apt-get\n--yes\n--no-remove\ninstall\n--\n/tmp/a b.deb\n"},
		{KindArch, "/tmp/a b.pkg.tar.zst", "pacman\n-U\n--noconfirm\n--\n/tmp/a b.pkg.tar.zst\n"},
	} {
		if err := packageInstall(Install{Kind: test.kind}, test.file); err != nil {
			t.Fatal(err)
		}
		out, err := os.ReadFile(log)
		if err != nil || string(out) != test.want {
			t.Fatalf("%s: %q, %v", test.kind, out, err)
		}
	}
}

func TestArchBuildFromProjectRecipe(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("makepkg runs as a regular user")
	}
	for _, name := range []string{"makepkg", "fakeroot", "pacman", "git"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("requires %s", name)
		}
	}
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "project")
	recipe := filepath.Join(repoDir, "build/linux/aur/wingsv-dex")
	if err := os.MkdirAll(recipe, 0755); err != nil {
		t.Fatal(err)
	}
	const fixture = `pkgname=wingsv-dex
pkgver=2.0
pkgrel=1
arch=(any)
options=(!debug !strip)
package() {
  install -Dm644 "$startdir/PKGBUILD" "$pkgdir/usr/share/test/recipe"
}
`
	if err := os.WriteFile(filepath.Join(recipe, "PKGBUILD"), []byte(fixture), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-b", "main"}, {"add", "."}, {"-c", "user.name=Test", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false", "commit", "-m", "fixture"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %s: %v", out, err)
		}
	}
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "url.file://"+repoDir+".insteadOf")
	t.Setenv("GIT_CONFIG_VALUE_0", archSource)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	// Inspect the finished archive through pacman without installing anything.
	stub := "#!/bin/sh\n[ \"$1 $2 $3 $4\" = 'pacman -U --noconfirm --' ] && pacman -Qp -- \"$5\"\n"
	if err := os.WriteFile(filepath.Join(dir, "pkexec"), []byte(stub), 0755); err != nil {
		t.Fatal(err)
	}
	if err := buildArchPackage(Install{Kind: KindArch, Package: "wingsv-dex"}, "2.0", func(Progress) {}); err != nil {
		t.Fatal(err)
	}
}
