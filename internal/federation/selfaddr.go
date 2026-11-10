package federation

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

// selfAddressProbe returns the device's own public address and nothing else to parse.
const selfAddressProbe = "https://api.ipify.org"

// ResolveSelfAddress measures the device's public address as the outside world sees it. The
// caller must run it OFF the tunnel (before the tunnel is up, on the desktop): through the
// tunnel the answer would be the node's address, and the whole point is to tell the panel
// where we really are, so it can cross-check that against what the node reports and catch a
// second VPN stacked over ours or a profile used by someone other than its owner. The device
// gives itself up on purpose: that is the honest price of free access. An empty return means
// the probe did not answer; the caller then sends no client_ip and the panel skips the check.
func ResolveSelfAddress(ctx context.Context) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, selfAddressProbe, nil)
	if err != nil {
		return ""
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return ""
	}
	addr := strings.TrimSpace(string(data))
	if addr == "" || len(addr) > 45 { // 45 = longest textual IPv6
		return ""
	}
	return addr
}

// receiptSelfTTL is how long a measured self-address is trusted. A desktop address can change,
// and the measurement is only valid while it was taken off the tunnel, so it is re-measured on
// the next connect rather than mid-session.
const receiptSelfTTL = 5 * time.Minute
