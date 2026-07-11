package config

import (
	"strconv"
	"strings"
)

// DefaultByeDPICommand is the preset ciadpi argument line used when command mode is on.
const DefaultByeDPICommand = "-o1 -d1 -a1 -At,r,s -s1 -d1 -s5+s -s10+s -s15+s -s20+s -r1+s -S -a1 -As -s1 -d1 -s5+s -s10+s -s15+s -s20+s -S -a1"

// ByeDPISettings configures the ByeDPI (ciadpi) local SOCKS proxy that xray can chain its
// outbound through as a DPI-bypass front.
type ByeDPISettings struct {
	Enabled            bool   `json:"enabled"`            // run ByeDPI and chain the xray outbound through it
	UseCommandSettings bool   `json:"useCommandSettings"` // use the raw command line instead of the editor fields
	Command            string `json:"command"`            // raw ciadpi argument line (command mode)

	ProxyIP     string `json:"proxyIp"`
	ProxyPort   int    `json:"proxyPort"`
	AuthEnabled bool   `json:"authEnabled"`
	Username    string `json:"username"`
	Password    string `json:"password"`

	MaxConnections int  `json:"maxConnections"`
	BufferSize     int  `json:"bufferSize"`
	DefaultTTL     int  `json:"defaultTtl"`
	NoDomain       bool `json:"noDomain"`
	TCPFastOpen    bool `json:"tcpFastOpen"`
	DropSACK       bool `json:"dropSack"`

	DesyncHTTP  bool `json:"desyncHttp"`
	DesyncHTTPS bool `json:"desyncHttps"`
	DesyncUDP   bool `json:"desyncUdp"`

	DesyncMethod  string `json:"desyncMethod"` // none | split | disorder | fake | oob | disoob
	SplitPosition int    `json:"splitPosition"`
	SplitAtHost   bool   `json:"splitAtHost"`
	FakeTTL       int    `json:"fakeTtl"`
	FakeSNI       string `json:"fakeSni"`
	FakeOffset    int    `json:"fakeOffset"`
	OOBData       string `json:"oobData"`
	UDPFakeCount  int    `json:"udpFakeCount"`

	HostMixedCase    bool `json:"hostMixedCase"`
	DomainMixedCase  bool `json:"domainMixedCase"`
	HostRemoveSpaces bool `json:"hostRemoveSpaces"`

	TLSRecordSplit         bool `json:"tlsRecordSplit"`
	TLSRecordSplitPosition int  `json:"tlsRecordSplitPosition"`
	TLSRecordSplitAtSNI    bool `json:"tlsRecordSplitAtSni"`

	HostsMode      string `json:"hostsMode"` // disable | blacklist | whitelist
	HostsBlacklist string `json:"hostsBlacklist"`
	HostsWhitelist string `json:"hostsWhitelist"`

	// Strategy finder (proxytest) parameters.
	ProxyTestDelaySeconds        int    `json:"proxyTestDelaySeconds"`
	ProxyTestRequests            int    `json:"proxyTestRequests"`
	ProxyTestConcurrencyLimit    int    `json:"proxyTestConcurrencyLimit"`
	ProxyTestTimeoutSeconds      int    `json:"proxyTestTimeoutSeconds"`
	ProxyTestSNI                 string `json:"proxyTestSni"`
	ProxyTestUseCustomStrategies bool   `json:"proxyTestUseCustomStrategies"`
	ProxyTestCustomStrategies    string `json:"proxyTestCustomStrategies"`
	ProxyTestTargets             string `json:"proxyTestTargets"` // newline-separated domains to probe
}

// DefaultByeDPITargets is the built-in list of domains the strategy finder probes.
const DefaultByeDPITargets = "youtube.com\nwww.youtube.com\ngooglevideo.com\ndiscord.com\nx.com"

// DefaultByeDPISettings returns the ByeDPI defaults.
func DefaultByeDPISettings() ByeDPISettings {
	return ByeDPISettings{
		Command:                DefaultByeDPICommand,
		ProxyIP:                "127.0.0.1",
		ProxyPort:              1080,
		AuthEnabled:            true,
		MaxConnections:         512,
		BufferSize:             16384,
		DefaultTTL:             0,
		DesyncHTTP:             true,
		DesyncHTTPS:            true,
		DesyncUDP:              true,
		DesyncMethod:           "oob",
		SplitPosition:          1,
		FakeTTL:                8,
		FakeSNI:                "www.iana.org",
		OOBData:                "a",
		UDPFakeCount:           1,
		TLSRecordSplit:         true,
		TLSRecordSplitPosition: 1,
		TLSRecordSplitAtSNI:    true,
		HostsMode:              "disable",

		ProxyTestDelaySeconds:     1,
		ProxyTestRequests:         1,
		ProxyTestConcurrencyLimit: 20,
		ProxyTestTimeoutSeconds:   5,
		ProxyTestSNI:              "max.ru",
		ProxyTestTargets:          DefaultByeDPITargets,
	}
}

// Normalized returns the settings with scalar defaults backstopped (used by the runner).
func (b ByeDPISettings) Normalized() ByeDPISettings { return b.withDefaults() }

func (b ByeDPISettings) withDefaults() ByeDPISettings {
	d := DefaultByeDPISettings()
	if b.ProxyIP == "" {
		b.ProxyIP = d.ProxyIP
	}
	if b.ProxyPort == 0 {
		b.ProxyPort = d.ProxyPort
	}
	if b.MaxConnections == 0 {
		b.MaxConnections = d.MaxConnections
	}
	if b.BufferSize == 0 {
		b.BufferSize = d.BufferSize
	}
	if b.DesyncMethod == "" {
		b.DesyncMethod = d.DesyncMethod
	}
	if b.SplitPosition == 0 {
		b.SplitPosition = d.SplitPosition
	}
	if b.FakeTTL == 0 {
		b.FakeTTL = d.FakeTTL
	}
	if b.FakeSNI == "" {
		b.FakeSNI = d.FakeSNI
	}
	if b.OOBData == "" {
		b.OOBData = d.OOBData
	}
	if b.UDPFakeCount == 0 {
		b.UDPFakeCount = d.UDPFakeCount
	}
	if b.TLSRecordSplitPosition == 0 {
		b.TLSRecordSplitPosition = d.TLSRecordSplitPosition
	}
	if b.HostsMode == "" {
		b.HostsMode = d.HostsMode
	}
	if b.Command == "" {
		b.Command = d.Command
	}
	if b.ProxyTestDelaySeconds == 0 {
		b.ProxyTestDelaySeconds = d.ProxyTestDelaySeconds
	}
	if b.ProxyTestRequests == 0 {
		b.ProxyTestRequests = d.ProxyTestRequests
	}
	if b.ProxyTestConcurrencyLimit == 0 {
		b.ProxyTestConcurrencyLimit = d.ProxyTestConcurrencyLimit
	}
	if b.ProxyTestTimeoutSeconds == 0 {
		b.ProxyTestTimeoutSeconds = d.ProxyTestTimeoutSeconds
	}
	if b.ProxyTestSNI == "" {
		b.ProxyTestSNI = d.ProxyTestSNI
	}
	if b.ProxyTestTargets == "" {
		b.ProxyTestTargets = d.ProxyTestTargets
	}
	return b
}

// ensureCreds generates a login/password once when auth is enabled.
func (b *ByeDPISettings) ensureCreds() bool {
	if !b.AuthEnabled {
		return false
	}
	changed := false
	if b.Username == "" {
		b.Username = randomToken()
		changed = true
	}
	if b.Password == "" {
		b.Password = randomToken()
		changed = true
	}
	return changed
}

// ListenAddr is the host:port the ByeDPI SOCKS listens on.
func (b ByeDPISettings) ListenAddr() string {
	ip := strings.TrimSpace(b.ProxyIP)
	if ip == "" {
		ip = "127.0.0.1"
	}
	port := b.ProxyPort
	if port == 0 {
		port = 1080
	}
	return ip + ":" + strconv.Itoa(port)
}
