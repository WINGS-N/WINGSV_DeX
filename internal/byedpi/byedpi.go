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
	for _, step := range b.DesyncSteps {
		if strings.TrimSpace(step.Flag) == "" {
			continue
		}
		a = append(a, step.Arg())
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
