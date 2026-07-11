package byedpi

import (
	_ "embed"
	"strings"

	"github.com/WINGS-N/wingsv-dex/internal/config"
)

//go:embed assets/strategies.list
var bundledStrategies string

// Strategies returns the candidate ciadpi command lines the strategy finder tries: the
// user's custom list when enabled and non-empty, otherwise the bundled list. The {sni}
// placeholder is replaced with the configured test SNI.
func Strategies(s config.ByeDPISettings) []string {
	sni := strings.TrimSpace(s.ProxyTestSNI)
	if sni == "" {
		sni = "max.ru"
	}
	src := bundledStrategies
	if s.ProxyTestUseCustomStrategies && strings.TrimSpace(s.ProxyTestCustomStrategies) != "" {
		src = s.ProxyTestCustomStrategies
	}
	var out []string
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, strings.ReplaceAll(line, "{sni}", sni))
	}
	return out
}

// Targets parses the newline-separated probe target list into distinct hostnames.
func Targets(s config.ByeDPISettings) []string {
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(s.ProxyTestTargets, "\n") {
		h := strings.TrimSpace(line)
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	return out
}
