package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/WINGS-N/wingsv-dex/internal/config"
	"github.com/WINGS-N/wingsv-dex/internal/federation"
)

// AccountStatusEvent fires whenever the signed-in state changes so the frontend can refresh
// the top-bar avatar and the account screen without polling.
const AccountStatusEvent = "account:status"

// accountState is the persisted session: enough to revalidate the token and address the panel
// on the next launch. The password is never stored; only the bearer token the panel issued.
type accountState struct {
	Panel         string `json:"panel"`
	Token         string `json:"token"`
	Username      string `json:"username"`
	AccountID     string `json:"accountId"`
	AvatarVersion int64  `json:"avatarVersion"`
	FederationID  string `json:"federationId"`
}

// AccountStatus is the shape the frontend reads to render the account UI.
type AccountStatus struct {
	SignedIn  bool   `json:"signedIn"`
	Username  string `json:"username"`
	Panel     string `json:"panel"`
	AvatarURL string `json:"avatarUrl"`
}

// AccountService owns the WINGS Account session: sign-in (by password or by approving a device
// in the browser), the live access figures, and sign-out. It persists the session next to the
// other app state so a restart keeps the user signed in.
type AccountService struct {
	mu       sync.Mutex
	path     string
	app      *application.App
	store    *config.Store
	receipts *federation.ReceiptEngine
	st       accountState
}

// NewAccountService loads any stored session from configDir and wires the federation receipt
// engine: it registers the current-account provider and keeps a reference so a sign-out can
// forget the signing key, queue and window mark. The engine is a constructor argument rather
// than an exported setter so Wails does not surface it to the frontend as a binding.
func NewAccountService(configDir string, store *config.Store, receipts *federation.ReceiptEngine) *AccountService {
	s := &AccountService{path: filepath.Join(configDir, "federation.json"), store: store, receipts: receipts}
	if raw, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(raw, &s.st)
	}
	if receipts != nil {
		receipts.SetAccount(s.receiptAccount)
	}
	return s
}

func (s *AccountService) SetApp(app *application.App) { s.app = app }

// receiptAccount reports the current account the receipt engine signs and sends for, or false
// when signed out. The client id is the account's federation subject id. Safe for the engine
// to poll each window: a sign-out flips it to false at once. Unexported so Wails does not
// surface it to the frontend as a binding.
func (s *AccountService) receiptAccount() (federation.ReceiptAccount, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.st.Token == "" || s.st.FederationID == "" {
		return federation.ReceiptAccount{}, false
	}
	return federation.ReceiptAccount{
		Client:   s.client(),
		Token:    s.st.Token,
		ClientID: s.st.FederationID,
	}, true
}

func (s *AccountService) client() *federation.Client {
	return federation.New(s.st.Panel)
}

func (s *AccountService) saveLocked() {
	if raw, err := json.MarshalIndent(s.st, "", "  "); err == nil {
		_ = os.WriteFile(s.path, raw, 0o600)
	}
}

func (s *AccountService) persist(sess federation.Session) {
	if sess.Username != "" {
		s.st.Username = sess.Username
	}
	s.st.Token = sess.Token
	s.st.AccountID = sess.AccountID
	s.st.AvatarVersion = sess.AvatarVersion
	s.st.FederationID = sess.FederationID
	s.saveLocked()
	go s.syncSubscription()
}

// syncSubscription pulls the access figures in the background, which also keeps the account's
// federation subscription in the list. Best effort: a network failure retries on the next read.
func (s *AccountService) syncSubscription() {
	_, _ = s.Access()
}

func (s *AccountService) clearLocked() {
	s.st = accountState{Panel: s.st.Panel}
	s.saveLocked()
}

func (s *AccountService) emit() {
	if s.app != nil {
		s.app.Event.Emit(AccountStatusEvent, s.statusLocked())
	}
}

// Status reports whether an account is signed in and the identity to show.
func (s *AccountService) Status() AccountStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked()
}

func (s *AccountService) statusLocked() AccountStatus {
	st := AccountStatus{
		SignedIn: s.st.Token != "",
		Username: s.st.Username,
		Panel:    s.client().Panel(),
	}
	if st.SignedIn && s.st.AccountID != "" {
		st.AvatarURL = s.client().AvatarURL(s.st.AccountID, s.st.AvatarVersion)
	}
	return st
}

// SetPanel points the account at a custom panel before sign-in; empty restores the default.
func (s *AccountService) SetPanel(panel string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Panel = panel
	s.saveLocked()
}

// LoginPassword signs in with a login and password.
func (s *AccountService) LoginPassword(login, password string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	sess, err := s.client().Login(ctx, login, password)
	if err != nil {
		var sf *federation.ErrSecondFactor
		if errors.As(err, &sf) {
			return errors.New("second factor required")
		}
		return err
	}
	s.persist(sess)
	s.emit()
	return nil
}

// BrowserLoginURL returns the panel link the user opens to approve this device; BrowserLogin
// also launches the system browser at it. The device label is this machine's hostname.
func (s *AccountService) BrowserLoginURL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	host, _ := os.Hostname()
	return s.client().BrowserLoginURL(host)
}

// BrowserLogin opens the browser-approval page and returns the URL it opened.
func (s *AccountService) BrowserLogin() (string, error) {
	url := s.BrowserLoginURL()
	return url, openURL(url)
}

// CompleteBrowserLogin exchanges the one-time code from an approved browser link for a session.
// main.go calls it when the panel's callback reaches the app.
func (s *AccountService) CompleteBrowserLogin(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	sess, err := s.client().CompleteBrowserLogin(ctx, code)
	if err != nil {
		return err
	}
	s.persist(sess)
	s.emit()
	return nil
}

// Access fetches the live access figures (allowance, speed caps, trust, per-server usage).
func (s *AccountService) Access() (federation.Access, error) {
	s.mu.Lock()
	token := s.st.Token
	client := s.client()
	s.mu.Unlock()
	if token == "" {
		return federation.Access{}, errors.New("not signed in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	access, err := client.Access(ctx, token)
	if err != nil {
		return federation.Access{}, err
	}
	if access.AvatarVersion != 0 {
		s.mu.Lock()
		s.st.AvatarVersion = access.AvatarVersion
		s.saveLocked()
		s.mu.Unlock()
	}
	if s.store != nil && strings.TrimSpace(access.SubscriptionURL) != "" {
		_ = s.store.EnsureFederationSubscription(access.SubscriptionURL)
	}
	return access, nil
}

// Donor reports what this account contributes to the federation and how it is being used.
func (s *AccountService) Donor() (federation.Donor, error) {
	s.mu.Lock()
	token := s.st.Token
	client := s.client()
	s.mu.Unlock()
	if token == "" {
		return federation.Donor{}, errors.New("not signed in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return client.Donor(ctx, token)
}

// EnrollNode returns the shell command that enrolls a new donated server with the given budget
// (in gigabytes); the user runs it on the server they want to contribute.
func (s *AccountService) EnrollNode(budgetGB int64) (string, error) {
	s.mu.Lock()
	token := s.st.Token
	client := s.client()
	s.mu.Unlock()
	if token == "" {
		return "", errors.New("not signed in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return client.EnrollCommand(ctx, token, budgetGB)
}

// SetNodeBudget sets a donated node's declared budget. The budget is given in gigabytes and
// stored in bytes; it returns the stored byte value.
func (s *AccountService) SetNodeBudget(nodeID string, budgetGB int64) (int64, error) {
	s.mu.Lock()
	token := s.st.Token
	client := s.client()
	s.mu.Unlock()
	if token == "" {
		return 0, errors.New("not signed in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return client.SetNodeBudget(ctx, token, nodeID, budgetGB*1024*1024*1024)
}

// Invites reads the account's invite codes and whether it may mint more.
func (s *AccountService) Invites() (federation.Invites, error) {
	s.mu.Lock()
	token := s.st.Token
	client := s.client()
	s.mu.Unlock()
	if token == "" {
		return federation.Invites{}, errors.New("not signed in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return client.Invites(ctx, token)
}

// CreateInvite mints a new invite code.
func (s *AccountService) CreateInvite() (federation.Invite, error) {
	s.mu.Lock()
	token := s.st.Token
	client := s.client()
	s.mu.Unlock()
	if token == "" {
		return federation.Invite{}, errors.New("not signed in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return client.CreateInvite(ctx, token)
}

// RedeemInvite redeems an invite code onto this account.
func (s *AccountService) RedeemInvite(code string) error {
	s.mu.Lock()
	token := s.st.Token
	client := s.client()
	s.mu.Unlock()
	if token == "" {
		return errors.New("not signed in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return client.RedeemInvite(ctx, token, code)
}

// OpenPanelSection opens a panel page (such as the client-management section) in the system
// browser. The path is appended to the account's panel base URL.
func (s *AccountService) OpenPanelSection(path string) error {
	s.mu.Lock()
	base := s.client().Panel()
	s.mu.Unlock()
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return openURL(strings.TrimRight(base, "/") + path)
}

// ChangeAvatar opens a native image picker, uploads the chosen file as the account avatar and
// refreshes the stored avatar version so the UI busts its cache. An empty return with no error
// means the user cancelled the picker.
func (s *AccountService) ChangeAvatar() error {
	s.mu.Lock()
	token := s.st.Token
	client := s.client()
	app := s.app
	s.mu.Unlock()
	if token == "" {
		return errors.New("not signed in")
	}
	if app == nil {
		return errors.New("no window")
	}
	path, err := app.Dialog.OpenFile().
		SetTitle("Сменить аватар").
		AddFilter("Изображения", "*.png;*.jpg;*.jpeg;*.webp").
		PromptForSingleSelection()
	if err != nil {
		return err
	}
	if strings.TrimSpace(path) == "" {
		return nil // cancelled
	}
	image, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// The panel caps avatars at 2 MB; reject early with the app's wording rather than on a
	// server error.
	if len(image) > 2*1024*1024 {
		return errors.New("Картинка больше 2 MB")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	version, err := client.UploadAvatar(ctx, token, image, avatarMIME(path))
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.st.AvatarVersion = version
	s.saveLocked()
	s.emit()
	s.mu.Unlock()
	return nil
}

// RemoveAvatar drops the account avatar back to the panel placeholder.
func (s *AccountService) RemoveAvatar() error {
	s.mu.Lock()
	token := s.st.Token
	client := s.client()
	s.mu.Unlock()
	if token == "" {
		return errors.New("not signed in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := client.RemoveAvatar(ctx, token); err != nil {
		return err
	}
	s.mu.Lock()
	s.st.AvatarVersion = 0
	s.saveLocked()
	s.emit()
	s.mu.Unlock()
	return nil
}

// avatarMIME guesses the image content type from the file extension, defaulting to PNG.
func avatarMIME(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

// ChangePassword updates the account password.
func (s *AccountService) ChangePassword(current, next string) error {
	s.mu.Lock()
	token := s.st.Token
	client := s.client()
	s.mu.Unlock()
	if token == "" {
		return errors.New("not signed in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return client.ChangePassword(ctx, token, current, next)
}

// Logout revokes the session on the panel and clears it locally either way.
func (s *AccountService) Logout() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	token := s.st.Token
	client := s.client()
	s.clearLocked()
	s.emit()
	if s.receipts != nil {
		s.receipts.Reset()
	}
	if s.store != nil {
		_ = s.store.RemoveFederationSubscription()
	}
	if token == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return client.Logout(ctx, token)
}

// Restore revalidates a stored token at startup, in the background. A failure leaves the
// cached identity in place rather than signing the user out on a flaky network.
func (s *AccountService) Restore() {
	s.mu.Lock()
	token := s.st.Token
	client := s.client()
	s.mu.Unlock()
	if token == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	sess, err := client.Restore(ctx, token)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.persist(sess)
	s.emit()
	s.mu.Unlock()
}

// MatchBrowserCallback inspects a link that reached the app (via the scheme handler or the
// loopback channel); if it carries this flow's one-time approval code it exchanges it and
// reports true, so the caller does not also feed it to the profile-import path.
func (s *AccountService) MatchBrowserCallback(link string) bool {
	code := browserCallbackCode(link)
	if code == "" {
		return false
	}
	go func() { _ = s.CompleteBrowserLogin(code) }()
	return true
}

// browserCallbackCode pulls the one-time code out of a panel approval callback, whether it
// arrives as a panel /link/CODE address or a wingsv://link|account callback. Anything else
// (a vless:// or a wingsv:// import link) yields the empty string.
func browserCallbackCode(link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return ""
	}
	if i := strings.Index(u.Path, "/link/"); i >= 0 {
		return strings.Trim(u.Path[i+len("/link/"):], "/")
	}
	if u.Scheme == "wingsv" && (u.Host == "account" || u.Host == "link") {
		if c := u.Query().Get("code"); c != "" {
			return c
		}
		return strings.Trim(u.Path, "/")
	}
	return ""
}

// openURL launches the system browser at url.
func openURL(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
