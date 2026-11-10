package federation

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	command "github.com/WINGS-N/wingsv-dex/internal/gen/statscommandpb"
)

// The counters app/proxyman/outbound/handler.go mints once policy.system.statsOutboundUplink
// and statsOutboundDownlink are set, for the outbound tagged "proxy".
const (
	xrayUplinkCounter   = "outbound>>>proxy>>>traffic>>>uplink"
	xrayDownlinkCounter = "outbound>>>proxy>>>traffic>>>downlink"
	xrayStatsCallBudget = 800 * time.Millisecond
)

// XrayStats reads xray's own exact per-direction byte counters over the StatsService gRPC
// commander. The counters are exact where the interface counters are not: the interface also
// carries local DNS, keepalive and tunnel overhead, so the two sides never agree on payload.
// There is no token: xray's commander authenticates nothing, so the unix socket's own 0600
// mode is the access control (on Windows the loopback-only TCP bind stands in for it).
type XrayStats struct {
	conn *grpc.ClientConn
	stub command.StatsServiceClient
}

// DialXrayStats connects to the commander at a gRPC target ("unix:///path" off Windows, or
// "127.0.0.1:port" on Windows). The connection is lazy; a dial error only surfaces on a read.
func DialXrayStats(target string) (*XrayStats, error) {
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &XrayStats{conn: conn, stub: command.NewStatsServiceClient(conn)}, nil
}

// Uplink returns the absolute bytes sent through the proxy outbound, or -1 when unavailable.
func (x *XrayStats) Uplink() int64 { return x.read(xrayUplinkCounter) }

// Downlink returns the absolute bytes received through the proxy outbound, or -1 when
// unavailable.
func (x *XrayStats) Downlink() int64 { return x.read(xrayDownlinkCounter) }

func (x *XrayStats) read(name string) int64 {
	ctx, cancel := context.WithTimeout(context.Background(), xrayStatsCallBudget)
	defer cancel()
	// A counter that has seen no traffic yet does not exist and comes back NOT_FOUND rather
	// than zero. Report that as unavailable (-1) so the caller keeps its previous sample
	// instead of taking a bogus drop to zero for a counter reset.
	resp, err := x.stub.GetStats(ctx, &command.GetStatsRequest{Name: name, Reset_: false})
	if err != nil {
		return -1
	}
	return resp.GetStat().GetValue()
}

// Close drops the connection.
func (x *XrayStats) Close() {
	if x.conn != nil {
		_ = x.conn.Close()
	}
}
