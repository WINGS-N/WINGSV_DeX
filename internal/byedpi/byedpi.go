// Package byedpi builds the ciadpi argument list from settings and runs the local ByeDPI
// SOCKS proxy that xray chains its outbound through as a DPI-bypass front.
package byedpi

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/WINGS-N/wingsv-dex/internal/config"
)

// connBindIP is the source IP ciadpi binds its upstream connections to.
const connBindIP = "0.0.0.0"

// Args builds the ciadpi argument list for the given settings. Command mode passes the raw
// argument line through (prefixing the listen ip/port when absent); the editor mode maps the
// structured desync fields to concatenated short flags.
func Args(b config.ByeDPISettings) []string {
	b = b.Normalized()
	if b.UseCommandSettings {
		return commandArgs(b)
	}
	return editorArgs(b)
}

func commandArgs(b config.ByeDPISettings) []string {
	tokens := tokenize(b.Command)
	var a []string
	if !hasFlag(tokens, "-i", "--ip") {
		a = append(a, "--ip", b.ProxyIP)
	}
	if !hasFlag(tokens, "-p", "--port") {
		a = append(a, "--port", strconv.Itoa(b.ProxyPort))
	}
	a = append(a, "--conn-ip", connBindIP)
	a = append(a, tokens...)
	return appendAuth(a, b)
}

func editorArgs(b config.ByeDPISettings) []string {
	a := []string{"-i" + b.ProxyIP, "-p" + strconv.Itoa(b.ProxyPort), "-I" + connBindIP}
	if b.MaxConnections > 0 {
		a = append(a, "-c"+strconv.Itoa(b.MaxConnections))
	}
	if b.BufferSize > 0 {
		a = append(a, "-b"+strconv.Itoa(b.BufferSize))
	}

	var protocols []string
	if b.DesyncHTTPS {
		protocols = append(protocols, "t")
	}
	if b.DesyncHTTP {
		protocols = append(protocols, "h")
	}
	protoArg := func() { a = append(a, "-K"+strings.Join(protocols, ",")) }

	hosts := ""
	switch b.HostsMode {
	case "blacklist":
		hosts = strings.TrimSpace(b.HostsBlacklist)
	case "whitelist":
		hosts = strings.TrimSpace(b.HostsWhitelist)
	}
	if hosts != "" {
		hostArg := "-H:" + strings.ReplaceAll(hosts, "\n", " ")
		if b.HostsMode == "blacklist" {
			a = append(a, hostArg, "-An")
			if len(protocols) > 0 {
				protoArg()
			}
		} else {
			if len(protocols) > 0 {
				protoArg()
			}
			a = append(a, hostArg)
		}
	} else if len(protocols) > 0 {
		protoArg()
	}

	if b.DefaultTTL != 0 {
		a = append(a, "-g"+strconv.Itoa(b.DefaultTTL))
	}
	if b.NoDomain {
		a = append(a, "-N")
	}

	if b.SplitPosition != 0 {
		pos := strconv.Itoa(b.SplitPosition)
		if b.SplitAtHost {
			pos += "+h"
		}
		switch b.DesyncMethod {
		case "split":
			a = append(a, "-s"+pos)
		case "disorder":
			a = append(a, "-d"+pos)
		case "oob":
			a = append(a, "-o"+pos)
		case "disoob":
			a = append(a, "-q"+pos)
		case "fake":
			a = append(a, "-f"+pos)
		}
	}

	if b.DesyncMethod == "fake" {
		if b.FakeTTL != 0 {
			a = append(a, "-t"+strconv.Itoa(b.FakeTTL))
		}
		if sni := strings.TrimSpace(b.FakeSNI); sni != "" {
			a = append(a, "-n"+sni)
		}
		if b.FakeOffset != 0 {
			a = append(a, "-O"+strconv.Itoa(b.FakeOffset))
		}
	}
	if b.DesyncMethod == "oob" || b.DesyncMethod == "disoob" {
		oob := strings.TrimSpace(b.OOBData)
		if oob == "" {
			oob = "a"
		}
		// ciadpi takes the OOB byte value, not the character.
		a = append(a, "-e"+strconv.Itoa(int(oob[0])))
	}

	var mods []string
	if b.HostMixedCase {
		mods = append(mods, "h")
	}
	if b.DomainMixedCase {
		mods = append(mods, "d")
	}
	if b.HostRemoveSpaces {
		mods = append(mods, "r")
	}
	if len(mods) > 0 {
		a = append(a, "-M"+strings.Join(mods, ","))
	}

	if b.TLSRecordSplit && b.TLSRecordSplitPosition != 0 {
		r := "-r" + strconv.Itoa(b.TLSRecordSplitPosition)
		if b.TLSRecordSplitAtSNI {
			r += "+s"
		}
		a = append(a, r)
	}
	if b.TCPFastOpen {
		a = append(a, "-F")
	}
	if b.DropSACK {
		a = append(a, "-Y")
	}
	a = append(a, "-An")
	if b.DesyncUDP {
		a = append(a, "-Ku")
		if b.UDPFakeCount > 0 {
			a = append(a, "-a"+strconv.Itoa(b.UDPFakeCount))
		}
		a = append(a, "-An")
	}
	return appendAuth(a, b)
}

func appendAuth(a []string, b config.ByeDPISettings) []string {
	if b.AuthEnabled {
		if b.Username != "" {
			a = append(a, "--socks-user", b.Username)
		}
		if b.Password != "" {
			a = append(a, "--socks-pass", b.Password)
		}
	}
	return a
}

func hasFlag(tokens []string, short, long string) bool {
	for _, t := range tokens {
		if t == short || t == long || strings.HasPrefix(t, short) {
			return true
		}
	}
	return false
}

// Tokenize splits a raw ciadpi argument line into argv tokens (whitespace, simple quotes).
func Tokenize(s string) []string { return tokenize(s) }

// tokenize splits a raw command string on whitespace, honoring simple double quotes.
func tokenize(s string) []string {
	var out []string
	var cur []rune
	inQuote := false
	flush := func() {
		if len(cur) > 0 {
			out = append(out, string(cur))
			cur = cur[:0]
		}
	}
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
		case (r == ' ' || r == '\t' || r == '\n') && !inQuote:
			flush()
		default:
			cur = append(cur, r)
		}
	}
	flush()
	return out
}

// Process is a running local ciadpi (used in proxy-only mode; vpn mode runs it via the
// net-helper so its egress can bypass the tunnel).
type Process struct {
	cmd *exec.Cmd
}

// Start spawns ciadpi with the settings-derived args. bin is the path to bin/byedpi.
func Start(bin string, b config.ByeDPISettings) (*Process, error) {
	cmd := exec.Command(bin, Args(b)...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	hideWindow(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &Process{cmd: cmd}, nil
}

// Stop kills the process.
func (p *Process) Stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Kill()
	_ = p.cmd.Wait()
}
