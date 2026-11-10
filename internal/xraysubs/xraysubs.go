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
func Fetch(ctx context.Context, sub config.Subscription) (Result, error) {
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
		res, err := fetchFrom(ctx, sub, candidate, client)
		if err == nil {
			return res, nil
		}
		failures = append(failures, fmt.Errorf("candidate %d: %w", i+1, err))
	}
	return Result{}, errors.Join(failures...)
}

func fetchFrom(ctx context.Context, sub config.Subscription, rawURL string, client *http.Client) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return Result{}, errors.New("invalid subscription URL")
	}
	req.Header.Set("User-Agent", "WINGSV-DeX")
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
	nodes := ParseProfiles(string(body), sub.ID, sub.Title)
	if len(nodes) == 0 {
		return Result{}, errors.New("subscription response contains no valid nodes")
	}
	return Result{
		Nodes: nodes,
		Quota: ParseQuota(resp.Header.Get("Subscription-Userinfo")),
	}, nil
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
