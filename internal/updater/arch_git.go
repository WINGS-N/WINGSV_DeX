package updater

import (
	"context"
	"encoding/hex"
	"os/exec"
	"strings"
	"time"
)

const archSource = "https://github.com/" + repo + ".git"

func checkArchGit(res Result) Result {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "ls-remote", "--exit-code", archSource, "refs/heads/main").Output()
	fields := strings.Fields(string(out))
	if err != nil || len(fields) != 2 || len(fields[0]) != 40 {
		res.Status, res.Error = "error", "cannot read the DeX main branch from GitHub"
		return res
	}
	hash := fields[0]
	_, revision, ok := strings.Cut(res.Current, ".g")
	revision, _, _ = strings.Cut(revision, "-")
	if ok && len(revision) >= 7 && strings.HasPrefix(hash, revision) {
		return res
	}
	if _, err := hex.DecodeString(hash); err != nil {
		res.Status, res.Error = "error", "invalid DeX revision from GitHub"
		return res
	}
	res.Status, res.Latest = "available", hash[:7]
	return res
}
