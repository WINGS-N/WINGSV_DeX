//go:build linux

package nethelper

import (
	"fmt"
	"os"
	"path/filepath"
)

func validateBundledBinary(candidate, name string) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve helper executable: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return "", fmt.Errorf("resolve helper executable: %w", err)
	}
	return validateSiblingBinary(self, candidate, name)
}

func validateSiblingBinary(self, candidate, name string) (string, error) {
	if candidate == "" || !filepath.IsAbs(candidate) {
		return "", fmt.Errorf("refusing %s binary: path must be absolute sibling of helper", name)
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("refusing %s binary: cannot resolve path: %w", name, err)
	}
	expected := filepath.Join(filepath.Dir(self), name)
	if resolved != expected {
		return "", fmt.Errorf("refusing %s binary: expected %s", name, expected)
	}
	return expected, nil
}
