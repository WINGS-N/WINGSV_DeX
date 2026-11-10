package xraysubs

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/WINGS-N/wingsv-dex/internal/config"
)

func TestFetchRejectsBadResponsesWithoutLeakingURL(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
		body string
		want string
	}{{"forbidden", http.StatusForbidden, "secret body", "HTTP status 403"}, {"html", http.StatusOK, "<html>login</html>", "no valid nodes"}} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			_, err := Fetch(t.Context(), config.Subscription{URL: srv.URL + "/token-secret"}, config.DeviceInfo{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Fetch error = %v, want %q", err, tc.want)
			}
			if strings.Contains(err.Error(), "token-secret") || strings.Contains(err.Error(), srv.URL) {
				t.Fatalf("error leaked URL: %v", err)
			}
		})
	}
}

func TestFetchCookieChallengeRedirect(t *testing.T) {
	inner := strings.Builder{}
	for i := 0; i < 23; i++ {
		inner.WriteString("vless://uuid@node" + strconv.Itoa(i) + ".example:443#Node\n")
	}
	body := base64.StdEncoding.EncodeToString([]byte(inner.String()))
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits == 1 {
			http.SetCookie(w, &http.Cookie{Name: "challenge", Value: "passed", Path: "/"})
			http.Redirect(w, r, r.URL.String(), http.StatusTemporaryRedirect)
			return
		}
		cookie, err := r.Cookie("challenge")
		if err != nil || cookie.Value != "passed" {
			http.Error(w, "challenge missing", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	got, err := Fetch(t.Context(), config.Subscription{URL: srv.URL + "/start"}, config.DeviceInfo{})
	if err != nil || len(got.Nodes) != 23 {
		t.Fatalf("Fetch = (%d nodes, %v)", len(got.Nodes), err)
	}
	if hits != 2 {
		t.Fatalf("request count = %d, want 2", hits)
	}
}

func TestFetchUsesConfiguredHTTPProxyAndDoesNotFallBack(t *testing.T) {
	var proxyRequests int
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyRequests++
		if r.Method != http.MethodConnect {
			t.Errorf("proxy method = %s, want CONNECT", r.Method)
		}
		http.Error(w, "proxy unavailable", http.StatusBadGateway)
	}))
	defer proxy.Close()
	proxyURL := proxy.URL
	// CONNECT destinations must be host:port; the configured proxy must stay loopback.
	directRequests := 0
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		directRequests++
		_, _ = w.Write([]byte("vless://id@node.example:443"))
	}))
	defer origin.Close()
	got, err := Fetch(t.Context(), config.Subscription{URL: origin.URL, ProxyURL: proxyURL}, config.DeviceInfo{})
	if err == nil || len(got.Nodes) != 0 {
		t.Fatalf("Fetch = (%d nodes, %v), want proxy failure", len(got.Nodes), err)
	}
	if proxyRequests == 0 || directRequests != 0 {
		t.Fatalf("proxy requests=%d direct requests=%d", proxyRequests, directRequests)
	}
}

func TestFetchFailsOverToMirror(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	defer primary.Close()
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("vless://uuid@node.example:443#Node"))
	}))
	defer mirror.Close()
	got, err := Fetch(t.Context(), config.Subscription{URL: primary.URL, MirrorURLs: []string{mirror.URL}}, config.DeviceInfo{})
	if err != nil || len(got.Nodes) != 1 {
		t.Fatalf("Fetch = (%d nodes, %v)", len(got.Nodes), err)
	}
}

func TestParseProfilesPlain(t *testing.T) {
	body := "vless://uuid@a.example:443#A\nvless://uuid@b.example:8443#B\n"
	nodes := ParseProfiles(body, "sub1", "Sub")
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].SubscriptionID != "sub1" || nodes[0].Title != "A" {
		t.Errorf("node0 = %+v", nodes[0])
	}
}

func TestParseProfilesBase64(t *testing.T) {
	inner := "vless://uuid@a.example:443#A\nvless://uuid@b.example:8443#B"
	body := base64.StdEncoding.EncodeToString([]byte(inner))
	nodes := ParseProfiles(body, "sub1", "Sub")
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes from base64 body, got %d", len(nodes))
	}
}

func TestParseProfilesDedupes(t *testing.T) {
	body := "vless://uuid@a.example:443#A\nvless://uuid@a.example:443#A\n"
	if n := ParseProfiles(body, "s", "S"); len(n) != 1 {
		t.Errorf("expected dedupe to 1, got %d", len(n))
	}
}

func TestParseQuota(t *testing.T) {
	q := ParseQuota("upload=100; download=200; total=1000; expire=1700000000")
	if q.Upload != 100 || q.Download != 200 || q.Total != 1000 || q.Expire != 1700000000 {
		t.Errorf("quota = %+v", q)
	}
}
