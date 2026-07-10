package services

import (
	"os"
	"path/filepath"
)

// OnboardingService persists whether the first-launch onboarding has run, so the SUW
// intro shows once per install. The flag is a marker file in the app config dir.
type OnboardingService struct {
	flagPath string
}

// NewOnboardingService prepares the service against the app config directory.
func NewOnboardingService(configDir string) *OnboardingService {
	return &OnboardingService{flagPath: filepath.Join(configDir, "onboarding.seen")}
}

// Seen reports whether the onboarding has already been completed.
func (o *OnboardingService) Seen() bool {
	_, err := os.Stat(o.flagPath)
	return err == nil
}

// MarkSeen records that onboarding is complete so it will not show again.
func (o *OnboardingService) MarkSeen() error {
	if err := os.MkdirAll(filepath.Dir(o.flagPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(o.flagPath, []byte("1\n"), 0o644)
}
