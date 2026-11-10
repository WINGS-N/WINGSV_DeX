// Package xraysubs fetches and parses xray subscription bodies into profiles, and reads
// the per-subscription traffic quota from the Subscription-Userinfo response header.
package xraysubs

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/WINGS-N/wingsv-dex/internal/config"
	wingsvpb "github.com/WINGS-N/wingsv-dex/internal/gen/wingsvpb"
	"github.com/WINGS-N/wingsv-dex/internal/wingsv"
)

// shareLinkRe matches the xray share links a subscription body may contain.
var shareLinkRe = regexp.MustCompile(`(?i)(?:vless|vmess|socks|ss|trojan|hysteria2|hy2)://[^\s"']+`)

// userinfoRe pulls the upload/download/total/expire counters from a Subscription-Userinfo
// header (e.g. "upload=0; download=1234; total=5000; expire=1700000000").
var userinfoRe = regexp.MustCompile(`(?i)(upload|download|total|expire)\s*=\s*([0-9]+)`)

// Quota is the advertised per-subscription traffic usage from Subscription-Userinfo.
type Quota struct {
	Upload   int64
	Download int64
	Total    int64
	Expire   int64
}

// Result is a fetched, parsed subscription.
type Result struct {
	Nodes []config.XrayProfile
	Quota Quota
}

// Fetch downloads the subscription and parses its nodes + quota. Fresh sockets (keep-alives
// disabled) are used so that, when a tunnel is up, the request exits through it rather than
// an idle connection pinned to the physical link.
// Fetch обходит адреса подписки по порядку и отдаёт первый ответивший. Ошибка
// возвращается только когда не ответил ни один, и тогда она несёт причины всех.
func Fetch(ctx context.Context, sub config.Subscription, hwid config.DeviceInfo) (Result, error) {
	urls := sub.AllURLs()
	if len(urls) == 0 {
		return Result{}, errors.New("subscription has no url")
	}
	transport := &http.Transport{DisableKeepAlives: true, Proxy: http.ProxyFromEnvironment}
	if sub.ProxyURL != "" {
		validatedProxyURL, err := config.ValidateSubscriptionProxyURL(sub.ProxyURL)
		if err != nil {
			return Result{}, errors.New("invalid subscription proxy")
		}
		proxyURL, err := url.Parse(validatedProxyURL)
		if err != nil {
			return Result{}, errors.New("invalid subscription proxy")
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	defer transport.CloseIdleConnections()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 30 * time.Second, Transport: transport, Jar: jar}
	var failures []error
	for i, candidate := range urls {
		res, err := fetchFrom(ctx, sub, candidate, client, hwid)
		if err == nil {
			return res, nil
		}
		failures = append(failures, fmt.Errorf("candidate %d: %w", i+1, err))
	}
	return Result{}, errors.Join(failures...)
}

func fetchFrom(ctx context.Context, sub config.Subscription, rawURL string, client *http.Client, hwid config.DeviceInfo) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return Result{}, errors.New("invalid subscription URL")
	}
	req.Header.Set("User-Agent", "WINGSV-DeX")
	// A custom Accept keeps the panel from serving an HTML page on the default text/html the
	// stdlib would otherwise send.
	req.Header.Set("Accept", "application/x-wingsv-config, text/plain, */*")
	// The caller resolved the device fingerprint for this subscription (empty when the user
	// turned sending off, forced to the real one for the federation link). The panel counts
	// device slots from these headers, and the federation link returns 403 without them.
	if hwid.HWID != "" {
		req.Header.Set("x-hwid", hwid.HWID)
	}
	if hwid.OS != "" {
		req.Header.Set("x-device-os", hwid.OS)
	}
	if hwid.OSVersion != "" {
		req.Header.Set("x-ver-os", hwid.OSVersion)
	}
	if hwid.Model != "" {
		req.Header.Set("x-device-model", hwid.Model)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, errors.New("subscription request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return Result{}, fmt.Errorf("subscription returned HTTP status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Result{}, err
	}
	nodes := parseSubscriptionBody(body, resp.Header.Get("Content-Type"), sub.ID, sub.Title)
	if len(nodes) == 0 {
		return Result{}, errors.New("subscription response contains no valid nodes")
	}
	return Result{
		Nodes: nodes,
		Quota: ParseQuota(resp.Header.Get("Subscription-Userinfo")),
	}, nil
}

// parseSubscriptionBody turns a subscription response into xray nodes. The federation link
// serves a WINGS binary config frame (content type application/x-wingsv-config) rather than a
// plain vless:// list, so that frame is decoded and its xray raw links pulled out; an ordinary
// subscription falls through to the plain-text parse. A body may also carry wingsv:// links as
// text, which are decoded the same way.
func parseSubscriptionBody(body []byte, contentType, subID, subTitle string) []config.XrayProfile {
	text := string(body)
	if strings.Contains(strings.ToLower(contentType), "application/x-wingsv-config") {
		if cfg, err := wingsv.DecodeFrame(body); err == nil {
			if nodes := ParseProfiles(strings.Join(xrayRawLinks(cfg), "\n"), subID, subTitle); len(nodes) > 0 {
				return nodes
			}
		}
	}
	if nodes := ParseProfiles(text, subID, subTitle); len(nodes) > 0 {
		return nodes
	}
	var links []string
	for _, l := range wingsv.ExtractLinks(text) {
		if cfg, err := wingsv.Decode(l); err == nil {
			links = append(links, xrayRawLinks(cfg)...)
		}
	}
	if len(links) > 0 {
		return ParseProfiles(strings.Join(links, "\n"), subID, subTitle)
	}
	return nil
}

// xrayRawLinks pulls the vless share links out of a decoded WINGS config.
func xrayRawLinks(cfg *wingsvpb.Config) []string {
	var out []string
	for _, p := range cfg.GetXray().GetProfiles() {
		if rl := strings.TrimSpace(p.GetRawLink()); rl != "" {
			out = append(out, rl)
		}
	}
	return out
}

// ParseProfiles extracts xray nodes from a subscription body: direct share links first,
// then a whole-body base64 decode when none are found (the common v2rayN format).
func ParseProfiles(body, subID, subTitle string) []config.XrayProfile {
	nodes := collect(body, subID, subTitle)
	if len(nodes) == 0 {
		if decoded, err := decodeBase64Loose(strings.TrimSpace(body)); err == nil {
			nodes = collect(decoded, subID, subTitle)
		}
	}
	return nodes
}

func collect(text, subID, subTitle string) []config.XrayProfile {
	var out []config.XrayProfile
	seen := map[string]bool{}
	for _, link := range shareLinkRe.FindAllString(text, -1) {
		p, ok := config.ParseShareLink(link)
		if !ok {
			continue
		}
		if seen[p.DedupKey()] {
			continue
		}
		seen[p.DedupKey()] = true
		p.SubscriptionID = subID
		p.SubscriptionTitle = subTitle
		out = append(out, p)
	}
	return out
}

// ParseQuota reads the Subscription-Userinfo header counters.
func ParseQuota(header string) Quota {
	var q Quota
	for _, m := range userinfoRe.FindAllStringSubmatch(header, -1) {
		v, _ := strconv.ParseInt(m[2], 10, 64)
		switch strings.ToLower(m[1]) {
		case "upload":
			q.Upload = v
		case "download":
			q.Download = v
		case "total":
			q.Total = v
		case "expire":
			q.Expire = v
		}
	}
	return q
}

func decodeBase64Loose(s string) (string, error) {
	s = strings.TrimSpace(s)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return string(b), nil
		}
	}
	return "", errInvalidBase64
}

type invalidBase64 struct{}

func (invalidBase64) Error() string { return "invalid base64" }

var errInvalidBase64 = invalidBase64{}
