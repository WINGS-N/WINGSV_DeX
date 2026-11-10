package federation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/proxy"
)

// receiptsNodeOff is what the door returns when the node answered 410: it runs without
// payouts and does not want receipts.
const receiptsNodeOff = -1

const (
	// nodeDoorHost is the door's name. There is no real DNS for it; the node's own kernel
	// resolves it on the far side of the tunnel.
	nodeDoorHost = "receipts.wingsv.internal"
	// nodeDoorPort is the door's port, the same on the node's loopback and on its tunnel
	// address.
	nodeDoorPort = 8909
)

// NodeDoorRoute is the last path for receipts: hand them to the node itself, which carries
// them to the head over its own session. Used when the panel is unreachable by every other
// path. Deliver returns how many were taken (0 on failure), or receiptsNodeOff on a 410.
type NodeDoorRoute interface {
	Deliver(receipts []Receipt) int
}

// nodeDoor delivers receipts to the node either through the Xray control SOCKS proxy (so the
// request lands inside the tunnel) or, failing that, straight to the node's tunnel gateway.
type nodeDoor struct {
	// controlPort is the Xray control SOCKS port, or 0 when there is none (VK TURN has no
	// Xray). gateway is the node's address inside the tunnel, or empty when unknown.
	controlPort func() int
	gateway     func() string
}

// NewNodeDoor builds a node-door route. controlPort returns the Xray control SOCKS port (0
// if none); gateway returns the node's tunnel-side address (empty if unknown).
func NewNodeDoor(controlPort func() int, gateway func() string) NodeDoorRoute {
	return &nodeDoor{controlPort: controlPort, gateway: gateway}
}

func (d *nodeDoor) Deliver(receipts []Receipt) int {
	if len(receipts) == 0 {
		return 0
	}
	body, err := json.Marshal(map[string]any{"receipts": receipts})
	if err != nil {
		return 0
	}

	// First the SOCKS path: the request goes through the node we are sitting behind.
	if port := d.controlPort(); port > 0 {
		url := fmt.Sprintf("http://%s:%d/receipts", nodeDoorHost, nodeDoorPort)
		if code := d.post(url, port, body); code == receiptsNodeOff || code > 0 {
			return code
		}
	}

	// Then the node's tunnel gateway directly.
	if gw := d.gateway(); gw != "" {
		url := fmt.Sprintf("http://%s:%d/receipts", gw, nodeDoorPort)
		return d.post(url, 0, body)
	}
	return 0
}

// post sends the body to url, routed through a loopback SOCKS proxy on socksPort when it is
// non-zero. It returns 1 on a 2xx, receiptsNodeOff on a 410, and 0 otherwise.
func (d *nodeDoor) post(url string, socksPort int, body []byte) int {
	transport := &http.Transport{}
	if socksPort > 0 {
		dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("127.0.0.1:%d", socksPort), nil, proxy.Direct)
		if err != nil {
			return 0
		}
		if ctxDialer, ok := dialer.(proxy.ContextDialer); ok {
			transport.DialContext = ctxDialer.DialContext
		} else {
			transport.DialContext = func(_ context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		}
	}
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusGone {
		return receiptsNodeOff
	}
	if resp.StatusCode/100 != 2 {
		return 0
	}
	return 1
}
