package services

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const DeepLinkImportEvent = "deeplink:import"

// DeepLinkService funnels import links that arrive from the OS URL-scheme handler or the
// loopback callback into the frontend. A link captured before the UI is ready (a cold launch
// from a wingsv:// click) is held in pending and pulled by the frontend on mount; a link that
// arrives while the app is already running is pushed as a DeepLinkImportEvent.
type DeepLinkService struct {
	mu      sync.Mutex
	pending string
	app     *application.App
}

func NewDeepLinkService() *DeepLinkService { return &DeepLinkService{} }

func (s *DeepLinkService) SetApp(app *application.App) { s.app = app }

// SetPending stores a link captured at startup for the frontend to pull once it mounts.
func (s *DeepLinkService) SetPending(link string) {
	s.mu.Lock()
	s.pending = link
	s.mu.Unlock()
}

// Pending returns and clears any startup link. The frontend calls it once on mount.
func (s *DeepLinkService) Pending() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	link := s.pending
	s.pending = ""
	return link
}

// Deliver pushes a link that arrived while the app is running to the frontend.
func (s *DeepLinkService) Deliver(link string) {
	if s.app != nil {
		s.app.Event.Emit(DeepLinkImportEvent, link)
	}
}
