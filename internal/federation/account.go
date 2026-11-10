// Package federation is the client for the WINGS Account panel: it signs in against the
// panel, restores a stored session, and reads the account's access state (allowance, speed
// caps, trust and per-server usage). The panel speaks JSON over HTTPS with a bearer token.
package federation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultPanel is the panel used when the account has no custom one configured.
const DefaultPanel = "https://v.wingsnet.org"

// Session is what a successful sign-in yields: the bearer token plus the identity the panel
// reports for the account.
type Session struct {
	Token         string `json:"token"`
	Username      string `json:"username"`
	AccountID     string `json:"accountId"`
	AvatarVersion int64  `json:"avatarVersion"`
	FederationID  string `json:"federationId"`
}

// ServerUsage is one federation server's traffic as the panel accounts it for this account.
type ServerUsage struct {
	Name      string `json:"name"`
	Transport string `json:"transport"`
	UpBytes   int64  `json:"upBytes"`
	DownBytes int64  `json:"downBytes"`
}

// Access is the account's current standing: how much it may still pull, its speed caps, its
// trust score and the per-server usage behind the totals.
type Access struct {
	SubscriptionURL string        `json:"subscriptionUrl"`
	UsedBytes       int64         `json:"usedBytes"`
	LimitBytes      int64         `json:"limitBytes"` // 0 means unlimited
	UplinkBps       int64         `json:"uplinkBps"`
	DownlinkBps     int64         `json:"downlinkBps"`
	TrustConfidence int           `json:"trustConfidence"`
	TrustBand       string        `json:"trustBand"`
	AvatarVersion   int64         `json:"avatarVersion"`
	Servers         []ServerUsage `json:"servers"`
}

// ErrSecondFactor is returned when the panel wants a second factor; Ticket carries the value
// the follow-up call must present.
type ErrSecondFactor struct{ Ticket string }

func (e *ErrSecondFactor) Error() string { return "federation: second factor required" }

// Client talks to one panel. The zero value is not usable; build it with New.
type Client struct {
	panel string
	http  *http.Client
}

// New returns a client for panel (DefaultPanel when empty).
func New(panel string) *Client {
	panel = strings.TrimRight(strings.TrimSpace(panel), "/")
	if panel == "" {
		panel = DefaultPanel
	}
	return &Client{panel: panel, http: &http.Client{Timeout: 20 * time.Second}}
}

// Panel is the base URL this client signs in against.
func (c *Client) Panel() string { return c.panel }

// AvatarURL is where the panel serves an account's avatar; version busts the cache when the
// avatar changes.
func (c *Client) AvatarURL(accountID string, version int64) string {
	if accountID == "" {
		return ""
	}
	return c.panel + "/api/admin/avatars/" + accountID + ".png?v=" + strconv.FormatInt(version, 10)
}

// Login signs in with a username and password. A panel that demands a second factor returns
// ErrSecondFactor instead of a session.
func (c *Client) Login(ctx context.Context, login, password string) (Session, error) {
	var resp struct {
		Token   string `json:"token"`
		Ticket  string `json:"ticket"`
		Account struct {
			ID            int64  `json:"id"`
			Username      string `json:"username"`
			AvatarVersion int64  `json:"avatar_version"`
			FederationID  string `json:"federation_id"`
		} `json:"account"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/app/account/login", "", map[string]string{
		"login":    login,
		"password": password,
	}, &resp); err != nil {
		return Session{}, err
	}
	if resp.Ticket != "" && resp.Token == "" {
		return Session{}, &ErrSecondFactor{Ticket: resp.Ticket}
	}
	return sessionFrom(resp.Token, resp.Account.ID, resp.Account.Username, login, resp.Account.AvatarVersion, resp.Account.FederationID), nil
}

// Restore revalidates a stored token and returns a refreshed session.
func (c *Client) Restore(ctx context.Context, token string) (Session, error) {
	var resp sessionResp
	if err := c.do(ctx, http.MethodPost, "/api/app/session", "", map[string]string{"token": token}, &resp); err != nil {
		return Session{}, err
	}
	return resp.session(token, ""), nil
}

// Access reads the account's current access state with a bearer token.
func (c *Client) Access(ctx context.Context, token string) (Access, error) {
	var resp struct {
		SubscriptionURL string `json:"subscription_url"`
		UsedBytes       int64  `json:"used_bytes"`
		LimitBytes      int64  `json:"limit_bytes"`
		UplinkBps       int64  `json:"uplink_bps"`
		DownlinkBps     int64  `json:"downlink_bps"`
		AvatarVersion   int64  `json:"avatar_version"`
		Trust           struct {
			Confidence int    `json:"confidence"`
			Band       string `json:"band"`
		} `json:"trust"`
		Servers []struct {
			Name      string `json:"name"`
			Transport string `json:"transport"`
			UpBytes   int64  `json:"up_bytes"`
			DownBytes int64  `json:"down_bytes"`
		} `json:"servers"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/app/access", token, nil, &resp); err != nil {
		return Access{}, err
	}
	out := Access{
		SubscriptionURL: resp.SubscriptionURL,
		UsedBytes:       resp.UsedBytes,
		LimitBytes:      resp.LimitBytes,
		UplinkBps:       resp.UplinkBps,
		DownlinkBps:     resp.DownlinkBps,
		TrustConfidence: resp.Trust.Confidence,
		TrustBand:       resp.Trust.Band,
		AvatarVersion:   resp.AvatarVersion,
	}
	for _, s := range resp.Servers {
		out.Servers = append(out.Servers, ServerUsage{Name: s.Name, Transport: s.Transport, UpBytes: s.UpBytes, DownBytes: s.DownBytes})
	}
	return out, nil
}

// BrowserLoginURL is the panel page that links this device to an account: the user opens it in
// a browser, approves the named device, and the panel hands back a one-time code. The device
// label is shown on the consent screen so the user sees what is asking for access.
func (c *Client) BrowserLoginURL(device string) string {
	u, err := url.Parse(c.panel + "/app/link")
	if err != nil {
		return c.panel + "/app/link"
	}
	q := u.Query()
	if strings.TrimSpace(device) != "" {
		q.Set("device", device)
	}
	q.Set("app", "wingsvdex")
	u.RawQuery = q.Encode()
	return u.String()
}

// CompleteBrowserLogin exchanges the one-time code from an approved browser link for a session.
func (c *Client) CompleteBrowserLogin(ctx context.Context, code string) (Session, error) {
	var resp sessionResp
	if err := c.do(ctx, http.MethodPost, "/api/app/session", "", map[string]string{"code": code}, &resp); err != nil {
		return Session{}, err
	}
	return resp.session(resp.Token, ""), nil
}

// Logout revokes the session on the panel. Best effort: the caller clears local state either
// way, since a revoked-but-not-cleared session would wedge the account screen.
func (c *Client) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return c.do(ctx, http.MethodPost, "/api/app/logout", token, map[string]string{}, nil)
}

// ChangePassword sets a new account password, authenticating with the current one.
func (c *Client) ChangePassword(ctx context.Context, token, current, next string) error {
	return c.do(ctx, http.MethodPost, "/api/app/password", token, map[string]string{
		"current": current,
		"new":     next,
	}, nil)
}

// Receipt is a signed statement of how many payload bytes a device carried through one
// node over one window. The device signs it with a key the node does not hold, so the node
// cannot forge usage it did not serve. The field names are the canonical wire form: the
// signed-byte layout in receiptKey.sign depends on them, do not rename without the server.
type Receipt struct {
	ClientID        string `json:"client_id"`
	NodeID          string `json:"node_id"`
	Transport       string `json:"transport"`
	WindowStartUnix int64  `json:"window_start_unix"`
	WindowEndUnix   int64  `json:"window_end_unix"`
	PayloadUpBytes  int64  `json:"payload_up_bytes"`
	PayloadDown     int64  `json:"payload_down_bytes"`
	Nonce           string `json:"nonce"`
	Signature       string `json:"signature"`
}

// UploadAvatar sets the account avatar from raw image bytes and returns the new version the
// panel assigned (used to bust the avatar URL cache). mime is the image's content type.
func (c *Client) UploadAvatar(ctx context.Context, token string, image []byte, mime string) (int64, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": []string{`form-data; name="avatar"; filename="avatar.png"`},
		"Content-Type":        []string{mime},
	})
	if err != nil {
		return 0, err
	}
	if _, err := part.Write(image); err != nil {
		return 0, err
	}
	if err := w.Close(); err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.panel+"/api/app/avatar", &body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("federation: upload avatar: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if msg := panelError(data); msg != "" {
			return 0, errors.New("federation: " + msg)
		}
		return 0, fmt.Errorf("federation: upload avatar: http %d", resp.StatusCode)
	}
	var out struct {
		AvatarVersion int64 `json:"avatar_version"`
	}
	_ = json.Unmarshal(data, &out)
	return out.AvatarVersion, nil
}

// RemoveAvatar drops the account avatar, returning it to the panel placeholder.
func (c *Client) RemoveAvatar(ctx context.Context, token string) error {
	return c.do(ctx, http.MethodDelete, "/api/app/avatar", token, nil, nil)
}

// SendKey registers the device's receipt-signing public key with the panel. The caller sends
// it once per account; the server keeps it to verify every later receipt.
func (c *Client) SendKey(ctx context.Context, token, publicKeyB64 string) error {
	return c.do(ctx, http.MethodPost, "/api/app/federation/key", token, map[string]string{"public_key": publicKeyB64}, nil)
}

// SendReceipts carries signed receipts to the panel and returns how many it accepted.
// clientIP is this device's own address as measured off the tunnel; empty omits it.
func (c *Client) SendReceipts(ctx context.Context, token string, receipts []Receipt, clientIP string) (int, error) {
	if len(receipts) == 0 {
		return 0, nil
	}
	body := map[string]any{"receipts": receipts}
	if clientIP != "" {
		body["client_ip"] = clientIP
	}
	var resp struct {
		Accepted int `json:"accepted"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/app/federation/receipts", token, body, &resp); err != nil {
		return 0, err
	}
	return resp.Accepted, nil
}

// DonorNode is one node an account has enrolled into the federation.
type DonorNode struct {
	NodeID              string  `json:"nodeId"`
	Hostname            string  `json:"hostname"`
	State               string  `json:"state"`
	Online              bool    `json:"online"`
	XrayVersion         string  `json:"xrayVersion"`
	VktpVersion         string  `json:"vktpVersion"`
	DeclaredBudgetBytes int64   `json:"declaredBudgetBytes"`
	UsedBytes           int64   `json:"usedBytes"`
	ProbeBytes          int64   `json:"probeBytes"`
	Sessions            int     `json:"sessions"`
	UpRateBps           float64 `json:"upRateBps"`
	DownRateBps         float64 `json:"downRateBps"`
}

// Donor is what the account contributes to the federation and how it is being used: the
// totals across its nodes plus the per-node breakdown.
type Donor struct {
	Enabled             bool        `json:"enabled"`
	Nodes               int         `json:"nodes"`
	NodesOnline         int         `json:"nodesOnline"`
	Sessions            int         `json:"sessions"`
	DeclaredBudgetBytes int64       `json:"declaredBudgetBytes"`
	UsedBytes           int64       `json:"usedBytes"`
	ProbeBytes          int64       `json:"probeBytes"`
	Error               string      `json:"error"`
	List                []DonorNode `json:"list"`
}

// Donor reads the account's donor summary.
func (c *Client) Donor(ctx context.Context, token string) (Donor, error) {
	var resp struct {
		Enabled             bool   `json:"enabled"`
		Nodes               int    `json:"nodes"`
		NodesOnline         int    `json:"nodes_online"`
		Sessions            int    `json:"sessions"`
		DeclaredBudgetBytes int64  `json:"declared_budget_bytes"`
		UsedBytes           int64  `json:"used_bytes"`
		ProbeBytes          int64  `json:"probe_bytes"`
		Error               string `json:"error"`
		NodeList            []struct {
			ID                  string  `json:"id"`
			NodeID              string  `json:"node_id"`
			Hostname            string  `json:"hostname"`
			State               string  `json:"state"`
			Online              bool    `json:"online"`
			XrayVersion         string  `json:"xray_version"`
			VktpVersion         string  `json:"vktp_version"`
			DeclaredBudgetBytes int64   `json:"declared_budget_bytes"`
			UsedBytes           int64   `json:"used_bytes"`
			ProbeBytes          int64   `json:"probe_bytes"`
			Sessions            int     `json:"sessions"`
			UpRateBps           float64 `json:"up_rate_bps"`
			DownRateBps         float64 `json:"down_rate_bps"`
		} `json:"node_list"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/app/federation/summary", token, nil, &resp); err != nil {
		return Donor{}, err
	}
	out := Donor{
		Enabled: resp.Enabled, Nodes: resp.Nodes, NodesOnline: resp.NodesOnline, Sessions: resp.Sessions,
		DeclaredBudgetBytes: resp.DeclaredBudgetBytes, UsedBytes: resp.UsedBytes, ProbeBytes: resp.ProbeBytes,
		Error: resp.Error,
	}
	for _, n := range resp.NodeList {
		id := n.ID
		if id == "" {
			id = n.NodeID
		}
		out.List = append(out.List, DonorNode{
			NodeID: id, Hostname: n.Hostname, State: n.State, Online: n.Online,
			XrayVersion: n.XrayVersion, VktpVersion: n.VktpVersion,
			DeclaredBudgetBytes: n.DeclaredBudgetBytes, UsedBytes: n.UsedBytes, ProbeBytes: n.ProbeBytes,
			Sessions: n.Sessions, UpRateBps: n.UpRateBps, DownRateBps: n.DownRateBps,
		})
	}
	return out, nil
}

// EnrollCommand asks the panel for the shell command that enrolls a new node with a budget
// (in gigabytes). The user runs the returned command on the server they want to donate.
func (c *Client) EnrollCommand(ctx context.Context, token string, budgetGB int64) (string, error) {
	if budgetGB < 0 {
		budgetGB = 0
	}
	var resp struct {
		Command string `json:"command"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/app/federation/enroll", token, map[string]int64{"budget_gb": budgetGB}, &resp); err != nil {
		return "", err
	}
	return resp.Command, nil
}

// SetNodeBudget sets a donated node's declared budget in bytes and returns the stored value.
func (c *Client) SetNodeBudget(ctx context.Context, token, nodeID string, bytes int64) (int64, error) {
	var resp struct {
		DeclaredBudgetBytes int64 `json:"declared_budget_bytes"`
	}
	if err := c.do(ctx, http.MethodPut, "/api/app/federation/nodes/"+url.PathEscape(nodeID)+"/budget", token, map[string]int64{"declared_budget_bytes": bytes}, &resp); err != nil {
		return 0, err
	}
	return resp.DeclaredBudgetBytes, nil
}

// Invite is one of an account's invite codes.
type Invite struct {
	Token    string `json:"token"`
	Link     string `json:"link"`
	UseCount int    `json:"useCount"`
	MaxUses  int    `json:"maxUses"`
	Spent    bool   `json:"spent"`
}

// Invites is the account's invite state: whether it may invite, and its existing codes.
type Invites struct {
	MayInvite bool     `json:"mayInvite"`
	Reason    string   `json:"reason"`
	List      []Invite `json:"list"`
}

// Invites reads the account's invite codes.
func (c *Client) Invites(ctx context.Context, token string) (Invites, error) {
	var resp struct {
		MayInvite bool   `json:"may_invite"`
		Reason    string `json:"reason"`
		Invites   []struct {
			Token    string `json:"token"`
			Link     string `json:"link"`
			UseCount int    `json:"use_count"`
			MaxUses  int    `json:"max_uses"`
			Spent    bool   `json:"spent"`
		} `json:"invites"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/app/invites", token, nil, &resp); err != nil {
		return Invites{}, err
	}
	out := Invites{MayInvite: resp.MayInvite, Reason: resp.Reason}
	for _, i := range resp.Invites {
		out.List = append(out.List, Invite{Token: i.Token, Link: i.Link, UseCount: i.UseCount, MaxUses: i.MaxUses, Spent: i.Spent})
	}
	return out, nil
}

// CreateInvite mints a new invite code.
func (c *Client) CreateInvite(ctx context.Context, token string) (Invite, error) {
	var resp struct {
		Token    string `json:"token"`
		Link     string `json:"link"`
		UseCount int    `json:"use_count"`
		MaxUses  int    `json:"max_uses"`
		Spent    bool   `json:"spent"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/app/invites", token, map[string]string{}, &resp); err != nil {
		return Invite{}, err
	}
	return Invite{Token: resp.Token, Link: resp.Link, UseCount: resp.UseCount, MaxUses: resp.MaxUses, Spent: resp.Spent}, nil
}

// RedeemInvite redeems an invite code onto this account.
func (c *Client) RedeemInvite(ctx context.Context, token, code string) error {
	return c.do(ctx, http.MethodPost, "/api/app/invites/redeem", token, map[string]string{"token": code}, nil)
}

// sessionResp is the identity payload the login and session endpoints return.
type sessionResp struct {
	Token   string `json:"token"`
	Account struct {
		ID            int64  `json:"id"`
		Username      string `json:"username"`
		AvatarVersion int64  `json:"avatar_version"`
		FederationID  string `json:"federation_id"`
	} `json:"account"`
}

func (r sessionResp) session(tokenFallback, nameFallback string) Session {
	tok := r.Token
	if tok == "" {
		tok = tokenFallback
	}
	return sessionFrom(tok, r.Account.ID, r.Account.Username, nameFallback, r.Account.AvatarVersion, r.Account.FederationID)
}

func sessionFrom(token string, id int64, username, fallbackName string, avatarVersion int64, federationID string) Session {
	if username == "" {
		username = fallbackName
	}
	accountID := ""
	if id != 0 {
		accountID = strconv.FormatInt(id, 10)
	}
	return Session{Token: token, Username: username, AccountID: accountID, AvatarVersion: avatarVersion, FederationID: federationID}
}

// do performs one JSON request. body is marshalled when non-nil; out is decoded from the
// response when non-nil. A non-2xx status is turned into an error carrying the panel's message.
func (c *Client) do(ctx context.Context, method, path, token string, body any, out any) error {
	if _, err := url.Parse(c.panel + path); err != nil {
		return fmt.Errorf("federation: bad url: %w", err)
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.panel+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("federation: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if msg := panelError(data); msg != "" {
			return errors.New("federation: " + msg)
		}
		return fmt.Errorf("federation: %s %s: http %d", method, path, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("federation: decode %s: %w", path, err)
	}
	return nil
}

func panelError(data []byte) string {
	var e struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(data, &e)
	if e.Error != "" {
		return e.Error
	}
	return e.Message
}
