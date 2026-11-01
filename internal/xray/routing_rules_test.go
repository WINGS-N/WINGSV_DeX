package xray

import (
	"testing"

	"github.com/WINGS-N/wingsv-dex/internal/config"
)

// Правила пользователя обязаны стоять ДО общего "всё в proxy": Xray берёт первое
// совпавшее, и за общим правилом они бы не отработали никогда.
func TestCustomRulesComeBeforeTheCatchAll(t *testing.T) {
	routing := buildRouting(config.DefaultXraySettings(), []config.XrayRoutingRule{
		{Match: config.RoutingMatchGeoIP, Code: "ru", Action: config.RoutingActionDirect, Enabled: true},
	})
	rules, ok := routing["rules"].([]any)
	if !ok {
		t.Fatalf("rules имеет тип %T", routing["rules"])
	}

	custom, catchAll := -1, -1
	for i, raw := range rules {
		rule, _ := raw.(map[string]any)
		if ips, hit := rule["ip"].([]string); hit && len(ips) == 1 && ips[0] == "geoip:ru" {
			custom = i
		}
		if rule["network"] == "tcp,udp" && rule["outboundTag"] == "proxy" {
			catchAll = i
		}
	}
	if custom < 0 {
		t.Fatal("правило geoip в конфиг не попало")
	}
	if catchAll < 0 {
		t.Fatal("общее правило пропало")
	}
	if custom > catchAll {
		t.Fatalf("правило пользователя после общего: %d > %d", custom, catchAll)
	}
}

// Каждый тип матчера уезжает в том виде, который понимает Xray: geo с префиксом,
// списки массивом, порт и network строкой, протокол массивом.
func TestEveryMatchTypeIsTranslated(t *testing.T) {
	tests := []struct {
		name  string
		rule  config.XrayRoutingRule
		key   string
		check func(t *testing.T, value any)
	}{
		{
			name: "geosite",
			rule: config.XrayRoutingRule{Match: config.RoutingMatchGeoSite, Code: "category-ads-all", Enabled: true},
			key:  "domain",
			check: func(t *testing.T, value any) {
				got := value.([]string)
				if len(got) != 1 || got[0] != "geosite:category-ads-all" {
					t.Fatalf("domain = %v", got)
				}
			},
		},
		{
			name: "domain list",
			rule: config.XrayRoutingRule{Match: config.RoutingMatchDomain, Code: "a.com, b.com", Enabled: true},
			key:  "domain",
			check: func(t *testing.T, value any) {
				got := value.([]string)
				if len(got) != 2 || got[0] != "a.com" || got[1] != "b.com" {
					t.Fatalf("domain = %v", got)
				}
			},
		},
		{
			name: "port string",
			rule: config.XrayRoutingRule{Match: config.RoutingMatchPort, Code: "80, 443", Enabled: true},
			key:  "port",
			check: func(t *testing.T, value any) {
				if value != "80,443" {
					t.Fatalf("port = %v", value)
				}
			},
		},
		{
			name: "network lowercased",
			rule: config.XrayRoutingRule{Match: config.RoutingMatchNetwork, Code: "TCP, UDP", Enabled: true},
			key:  "network",
			check: func(t *testing.T, value any) {
				if value != "tcp,udp" {
					t.Fatalf("network = %v", value)
				}
			},
		},
		{
			name: "protocol array",
			rule: config.XrayRoutingRule{Match: config.RoutingMatchProtocol, Code: "BitTorrent", Enabled: true},
			key:  "protocol",
			check: func(t *testing.T, value any) {
				got := value.([]string)
				if len(got) != 1 || got[0] != "bittorrent" {
					t.Fatalf("protocol = %v", got)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := customRoutingRules([]config.XrayRoutingRule{test.rule})
			if len(out) != 1 {
				t.Fatalf("правил на выходе %d, ждали одно", len(out))
			}
			rule := out[0].(map[string]any)
			value, ok := rule[test.key]
			if !ok {
				t.Fatalf("ключа %q нет: %v", test.key, rule)
			}
			test.check(t, value)
		})
	}
}

// Выключенное правило и правило с пустым кодом в конфиг не попадают: пустой
// матчер у Xray означает "совпало со всем", и такое правило забрало бы весь трафик.
func TestDisabledAndEmptyRulesAreDropped(t *testing.T) {
	out := customRoutingRules([]config.XrayRoutingRule{
		{Match: config.RoutingMatchGeoIP, Code: "ru", Enabled: false},
		{Match: config.RoutingMatchGeoIP, Code: "   ", Enabled: true},
		{Match: config.RoutingMatchDomain, Code: ",, ", Enabled: true},
	})
	if len(out) != 0 {
		t.Fatalf("в конфиг просочилось %d правил: %v", len(out), out)
	}
}

// Действие переводится в тег аутбаунда, неизвестное - в proxy.
func TestActionMapsToOutboundTag(t *testing.T) {
	cases := map[config.XrayRoutingAction]string{
		config.RoutingActionDirect:        "direct",
		config.RoutingActionBlock:         "block",
		config.RoutingActionProxy:         "proxy",
		config.XrayRoutingAction("нечто"): "proxy",
	}
	for action, want := range cases {
		rule := config.XrayRoutingRule{Match: config.RoutingMatchGeoIP, Code: "ru", Action: action, Enabled: true}
		out := customRoutingRules([]config.XrayRoutingRule{rule})
		if len(out) != 1 {
			t.Fatalf("%v: правил %d", action, len(out))
		}
		if got := out[0].(map[string]any)["outboundTag"]; got != want {
			t.Fatalf("%v -> %v, ждали %v", action, got, want)
		}
	}
}
