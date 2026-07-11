package config

import (
	"strconv"
	"strings"
)

// DefaultByeDPICommand is the recommended ciadpi desync line. In the editor it is parsed
// into steps; in command mode it is used verbatim.
const DefaultByeDPICommand = "-o1 -d1 -a1 -At,r,s -s1 -d1 -s5+s -s10+s -s15+s -s20+s -r1+s -S -a1 -As -s1 -d1 -s5+s -s10+s -s15+s -s20+s -S -a1"

// ByeDPIStep is one ciadpi desync argument in the editor's ordered step list. Flag is the
// option (e.g. "-s", "-o", "-A", "-S"); Value is its argument (e.g. "5+s", "t,r,s"), empty
// for flags that take none. Rendered back concatenated ("-s"+"5+s" -> "-s5+s") so the step
// list round-trips the command exactly.
type ByeDPIStep struct {
	Flag  string `json:"flag"`
	Value string `json:"value"`
}

// Arg renders the step as a single ciadpi token.
func (s ByeDPIStep) Arg() string {
	if s.Value == "" {
		return s.Flag
	}
	return s.Flag + s.Value
}

// ByeDPISettings configures the ByeDPI (ciadpi) local SOCKS proxy that xray can chain its
// outbound through as a DPI-bypass front.
type ByeDPISettings struct {
	Enabled            bool   `json:"enabled"`            // run ByeDPI and chain the xray outbound through it
	UseCommandSettings bool   `json:"useCommandSettings"` // use the raw command line instead of the step editor
	Command            string `json:"command"`            // raw ciadpi argument line (command mode)

	ProxyIP     string `json:"proxyIp"`
	ProxyPort   int    `json:"proxyPort"`
	AuthEnabled bool   `json:"authEnabled"`
	Username    string `json:"username"`
	Password    string `json:"password"`

	// DesyncSteps is the ordered desync argument list the editor builds and encodes.
	DesyncSteps []ByeDPIStep `json:"desyncSteps"`

	// Strategy finder (proxytest) parameters.
	ProxyTestConcurrencyLimit    int    `json:"proxyTestConcurrencyLimit"`
	ProxyTestTimeoutSeconds      int    `json:"proxyTestTimeoutSeconds"`
	ProxyTestSNI                 string `json:"proxyTestSni"`
	ProxyTestUseCustomStrategies bool   `json:"proxyTestUseCustomStrategies"`
	ProxyTestCustomStrategies    string `json:"proxyTestCustomStrategies"`
	ProxyTestTargets             string `json:"proxyTestTargets"` // newline-separated domains to probe
}

// DefaultByeDPITargets is the built-in list of domains the strategy finder probes.
const DefaultByeDPITargets = "youtube.com\nwww.youtube.com\ngooglevideo.com\ndiscord.com\nx.com"

// ParseByeDPISteps splits a ciadpi command line into editor steps.
func ParseByeDPISteps(command string) []ByeDPIStep {
	var steps []ByeDPIStep
	for _, tok := range strings.Fields(command) {
		flag, value := splitByeDPIToken(tok)
		steps = append(steps, ByeDPIStep{Flag: flag, Value: value})
	}
	return steps
}

func splitByeDPIToken(tok string) (string, string) {
	if strings.HasPrefix(tok, "--") {
		return tok, ""
	}
	if len(tok) >= 2 && tok[0] == '-' {
		return tok[:2], tok[2:]
	}
	return tok, ""
}

// DefaultByeDPISettings returns the ByeDPI defaults; the editor starts on the recommended
// desync steps.
func DefaultByeDPISettings() ByeDPISettings {
	return ByeDPISettings{
		Command:     DefaultByeDPICommand,
		ProxyIP:     "127.0.0.1",
		ProxyPort:   1080,
		AuthEnabled: true,
		DesyncSteps: ParseByeDPISteps(DefaultByeDPICommand),

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
	if b.Command == "" {
		b.Command = d.Command
	}
	if b.DesyncSteps == nil {
		b.DesyncSteps = d.DesyncSteps
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
