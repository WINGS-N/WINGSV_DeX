package services

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/WINGS-N/wingsv-dex/internal/updater"
)

// AppVersion is the WINGS V DeX release string shown on the About screen. Keep it in
// lockstep with the git tag on release.
const AppVersion = "0.1.1"

// Update event names the About screen subscribes to.
const (
	UpdateProgressEvent = "update:progress"
	UpdateStateEvent    = "update:state"
)

// AboutService backs the About screen: the app version and the self-updater.
type AboutService struct {
	mu         sync.Mutex
	app        *application.App
	stopVkturn func()
	applying   bool
}

// NewAboutService constructs the service. stopVkturn is called before an update installs
// so the vkturn child releases its binary (required on Windows; harmless on Linux).
func NewAboutService(stopVkturn func()) *AboutService {
	return &AboutService{stopVkturn: stopVkturn}
}

// SetApp wires the Wails app used to emit update events and to quit on restart.
func (a *AboutService) SetApp(app *application.App) {
	a.mu.Lock()
	a.app = app
	a.mu.Unlock()
}

// Version returns the version string rendered under the app name on the About screen.
func (a *AboutService) Version() string { return AppVersion }

// CheckUpdate reports the install kind and whether a newer release is available.
func (a *AboutService) CheckUpdate() updater.Result {
	return updater.Check(AppVersion)
}

// ApplyUpdate downloads and installs the newest release in the background (streaming
// progress via events), then restarts the app. Safe to call once; repeat calls no-op
// while one is running.
func (a *AboutService) ApplyUpdate() {
	a.mu.Lock()
	if a.applying {
		a.mu.Unlock()
		return
	}
	a.applying = true
	a.mu.Unlock()

	go func() {
		res := updater.Check(AppVersion)
		if res.Status != "available" || res.AppAsset == nil {
			a.emit(UpdateStateEvent, map[string]any{"state": "uptodate"})
			a.setApplying(false)
			return
		}
		if a.stopVkturn != nil {
			a.stopVkturn()
		}
		a.emit(UpdateStateEvent, map[string]any{"state": "installing"})
		if err := updater.Apply(res, func(p updater.Progress) { a.emit(UpdateProgressEvent, p) }); err != nil {
			a.emit(UpdateStateEvent, map[string]any{"state": "error", "error": err.Error()})
			a.setApplying(false)
			return
		}
		a.emit(UpdateStateEvent, map[string]any{"state": "restarting"})
		_ = updater.Restart(updater.Detect())
		a.quit()
	}()
}

func (a *AboutService) setApplying(v bool) {
	a.mu.Lock()
	a.applying = v
	a.mu.Unlock()
}

func (a *AboutService) emit(name string, data any) {
	a.mu.Lock()
	app := a.app
	a.mu.Unlock()
	if app != nil {
		app.Event.Emit(name, data)
	}
}

func (a *AboutService) quit() {
	a.mu.Lock()
	app := a.app
	a.mu.Unlock()
	if app != nil {
		app.Quit()
	}
}
