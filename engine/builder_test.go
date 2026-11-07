package engine

import (
	stdjson "encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/sagernet/sing-box/option"
)

func testWarp() Warp {
	return Warp{
		PrivateKey:    "OP+0TxVB0xaYtLthaqIOxPuKdEv7FsWPuFcmk4INWHs=",
		Address:       []string{"172.16.0.2/32", "2606:4700:110:8a36:df92:102a:9602:fa18/128"},
		PeerPublicKey: "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=",
		Server:        "162.159.192.1",
		Port:          2408,
		Reserved:      []byte{1, 2, 3},
		MTU:           1280,
		Keepalive:     25,
	}
}

func mustRules(t *testing.T, action Action, values ...string) []Rule {
	t.Helper()
	rules, err := ParseRules(action, values)
	if err != nil {
		t.Fatal(err)
	}
	return rules
}

// buildAndParse builds a configuration and validates it with sing-box's
// strict decoder, which rejects unknown fields and invalid values.
func buildAndParse(t *testing.T, options Options) (object, option.Options) {
	t.Helper()
	config, err := Build(options)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	parsed, err := ParseObject(NewContext(), config)
	if err != nil {
		content, _ := stdjson.MarshalIndent(config, "", "  ")
		t.Fatalf("sing-box rejected the configuration: %v\n%s", err, content)
	}
	return config, parsed
}

func allRules(t *testing.T) []Rule {
	var rules []Rule
	rules = append(rules, mustRules(t, ActionBlock, "geosite:category-ads-all", "protocol:bittorrent")...)
	rules = append(rules, mustRules(t, ActionWarp, "domain:chat.openai.com", "app:chrome.exe")...)
	rules = append(rules, mustRules(t, ActionDirect,
		"geoip:ir", "suffix:ir", "keyword:bank", "regex:^cdn\\.", "10.0.0.0/8", "2001:db8::/32",
		`path:C:\Games\game.exe`, "path_regex:^/usr/bin/.+", "port:22", "port:6000-7000",
		"network:udp", "ruleset:https://example.com/custom.srs", "ruleset:./local.json", "private")...)
	return rules
}

func TestBuildProxy(t *testing.T) {
	config, parsed := buildAndParse(t, Options{
		Log:  Log{Level: "debug"},
		Warp: testWarp(),
		Listeners: []Listener{
			{Type: "mixed", Port: 1080, Users: []User{{"alice", "secret"}}, SetSystemProxy: true},
			{Type: "socks", Listen: "0.0.0.0", Port: 1081},
			{Type: "socks", Port: 1082},
			{Type: "http", Port: 8080},
		},
		Rules:     allRules(t),
		BypassLAN: true,
		DNS:       DNS{Remote: "tls://1.1.1.1", Direct: "udp://192.168.1.1:53", Strategy: "prefer_ipv4"},
		ClashAPI:  "127.0.0.1:9090",
		APISecret: "s",
		CacheFile: "cache.db",
	})

	if len(parsed.Inbounds) != 4 || parsed.Inbounds[2].Tag != "socks-in-2" {
		t.Errorf("unexpected inbounds: %+v", parsed.Inbounds)
	}
	if parsed.Route.Final != DefaultWarpTag || !parsed.Route.FindProcess || parsed.Route.AutoDetectInterface {
		t.Errorf("unexpected route options: %+v", parsed.Route)
	}
	if len(parsed.Route.RuleSet) != 4 {
		t.Errorf("want 4 rule sets, got %d", len(parsed.Route.RuleSet))
	}
	if config["http_clients"] == nil {
		t.Error("remote rule sets need an HTTP client")
	}

	// Domain and process rules come before the resolve action, IP rules after.
	rules := config["route"].(object)["rules"].([]any)
	resolveIndex := -1
	for i, rule := range rules {
		if rule.(object)["action"] == "resolve" {
			resolveIndex = i
		}
	}
	if resolveIndex < 0 {
		t.Fatal("missing resolve action")
	}
	for i, rule := range rules {
		r := rule.(object)
		_, isIP := r["ip_cidr"]
		_, isDomain := r["domain_suffix"]
		if isIP && i < resolveIndex {
			t.Errorf("IP rule %d placed before resolve", i)
		}
		if isDomain && i > resolveIndex {
			t.Errorf("domain rule %d placed after resolve", i)
		}
	}
	// Blocks are evaluated before WARP and direct rules.
	if first := rules[3].(object); first["action"] != "reject" {
		t.Errorf("first user rule should be a block, got %+v", first)
	}

	endpoint := config["endpoints"].([]any)[0].(object)
	peer := endpoint["peers"].([]any)[0].(object)
	if !reflect.DeepEqual(peer["reserved"], []int{1, 2, 3}) {
		t.Errorf("reserved = %v", peer["reserved"])
	}
	if _, ok := endpoint["domain_resolver"]; ok {
		t.Error("IP endpoints need no domain resolver")
	}
}

func TestBuildTunIncludeMode(t *testing.T) {
	config, parsed := buildAndParse(t, Options{
		Warp: testWarp(),
		Tun: &Tun{
			MTU: 9000, Stack: "mixed", StrictRoute: true,
			RouteExcludeAddress: []string{"192.168.0.0/16"},
		},
		Rules: mustRules(t, ActionWarp, "app:chrome.exe", "suffix:google.com"),
		Final: ActionDirect,
	})
	if parsed.Route.Final != TagDirect || !parsed.Route.AutoDetectInterface {
		t.Errorf("unexpected route options: %+v", parsed.Route)
	}
	if parsed.Inbounds[0].Type != "tun" {
		t.Errorf("want tun inbound, got %s", parsed.Inbounds[0].Type)
	}
	if final := config["dns"].(object)["final"]; final != tagDNSDirect {
		t.Errorf("include mode should resolve unmatched domains directly, got %v", final)
	}
	if config["http_clients"] != nil {
		t.Error("no remote rule sets, no HTTP client expected")
	}
}

func TestBuildFinalBlock(t *testing.T) {
	config, _ := buildAndParse(t, Options{
		Warp:      testWarp(),
		Listeners: []Listener{{Port: 1080}},
		Tun:       &Tun{},
		Rules:     mustRules(t, ActionWarp, "suffix:example.com"),
		Final:     ActionBlock,
	})
	rules := config["route"].(object)["rules"].([]any)
	last := rules[len(rules)-1].(object)
	if last["action"] != "reject" || !reflect.DeepEqual(last["inbound"], []string{"mixed-in", tagTun}) {
		t.Errorf("last rule should reject everything, got %+v", last)
	}
}

func TestBuildRemoteDNSByName(t *testing.T) {
	config, _ := buildAndParse(t, Options{
		Warp:      testWarp(),
		Listeners: []Listener{{Port: 1080}},
		DNS:       DNS{Remote: "https://dns.google/dns-query"},
	})
	rules := config["dns"].(object)["rules"].([]any)
	first := rules[0].(object)
	if !reflect.DeepEqual(first["domain"], []string{"dns.google"}) || first["server"] != tagDNSDirect {
		t.Errorf("remote DNS host must be resolved directly, got %+v", first)
	}
}

func TestBuildEndpointByName(t *testing.T) {
	warp := testWarp()
	warp.Server = "engage.cloudflareclient.com"
	config, _ := buildAndParse(t, Options{Warp: warp, Listeners: []Listener{{Port: 1080}}})
	endpoint := config["endpoints"].([]any)[0].(object)
	if endpoint["domain_resolver"] != tagDNSDirect {
		t.Errorf("hostname endpoint needs a domain resolver, got %v", endpoint["domain_resolver"])
	}
}

func TestBuildErrors(t *testing.T) {
	cases := map[string]Options{
		"nothing to run":     {Warp: testWarp()},
		"bad listener type":  {Warp: testWarp(), Listeners: []Listener{{Type: "ftp", Port: 1}}},
		"socks system proxy": {Warp: testWarp(), Listeners: []Listener{{Type: "socks", Port: 1, SetSystemProxy: true}}},
		"named direct DNS":   {Warp: testWarp(), Listeners: []Listener{{Port: 1}}, DNS: DNS{Direct: "tls://dns.google"}},
		"bad DNS scheme":     {Warp: testWarp(), Listeners: []Listener{{Port: 1}}, DNS: DNS{Remote: "ftp://1.1.1.1"}},
	}
	for name, options := range cases {
		if _, err := Build(options); err == nil {
			t.Errorf("%s: Build succeeded, want error", name)
		}
	}
}

func TestParseListener(t *testing.T) {
	tests := map[string]Listener{
		"127.0.0.1:1080":            {Type: "mixed", Listen: "127.0.0.1", Port: 1080},
		":8080":                     {Type: "mixed", Listen: "127.0.0.1", Port: 8080},
		"socks5://u:p@0.0.0.0:1080": {Type: "socks", Listen: "0.0.0.0", Port: 1080, Users: []User{{"u", "p"}}},
		"http://[::1]:3128":         {Type: "http", Listen: "::1", Port: 3128},
	}
	for input, want := range tests {
		got, err := ParseListener(input)
		if err != nil {
			t.Errorf("ParseListener(%q): %v", input, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ParseListener(%q) = %+v, want %+v", input, got, want)
		}
	}
	for _, input := range []string{"ftp://1.2.3.4:21", "1.2.3.4", "mixed://1.2.3.4:0", "http://1.2.3.4:80/path"} {
		if _, err := ParseListener(input); err == nil {
			t.Errorf("ParseListener(%q) succeeded", input)
		}
	}
}

func TestDNSServer(t *testing.T) {
	server, host, err := DNSServer("x", "https://dns.google/custom")
	if err != nil || host != "dns.google" || server["path"] != "/custom" || server["type"] != "https" {
		t.Errorf("https: %+v %q %v", server, host, err)
	}
	server, host, err = DNSServer("x", "8.8.8.8")
	if err != nil || host != "" || server["type"] != "udp" {
		t.Errorf("bare IP: %+v %q %v", server, host, err)
	}
	server, _, err = DNSServer("x", "tls://1.1.1.1:853")
	if err != nil || server["server_port"] != uint64(853) {
		t.Errorf("tls port: %+v %v", server, err)
	}
}

const userConfig = `{
  // comments are allowed
  "inbounds": [{"type": "mixed", "tag": "in", "listen": "127.0.0.1", "listen_port": 1080}], /* block */
  "route": {"rule_set": [{"type": "remote", "tag": "r", "format": "binary", "url": "https://example.com/r.srs"}]},
  "outbounds": [
    {"type": "socks", "tag": "warp", "server": "127.0.0.1", "server_port": 1},
    {"type": "direct", "tag": "direct"}
  ]
}`

func TestInjectWarp(t *testing.T) {
	ctx := NewContext()
	options, err := Merge(ctx, [][]byte{[]byte(userConfig)})
	if err != nil {
		t.Fatal(err)
	}
	if err := InjectWarp(ctx, &options, testWarp()); err != nil {
		t.Fatal(err)
	}
	if len(options.Endpoints) != 1 || options.Endpoints[0].Tag != DefaultWarpTag {
		t.Errorf("endpoint not injected: %+v", options.Endpoints)
	}
	if len(options.Outbounds) != 1 || options.Outbounds[0].Tag != TagDirect {
		t.Errorf("outbound with the WARP tag should be replaced: %+v", options.Outbounds)
	}
	if options.Route.Final != DefaultWarpTag {
		t.Errorf("final = %q", options.Route.Final)
	}
	content, err := Marshal(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	// The dump must be loadable again.
	if _, err := Parse(ctx, content); err != nil {
		t.Errorf("dumped configuration does not parse: %v\n%s", err, content)
	}
}

func TestMerge(t *testing.T) {
	ctx := NewContext()
	override := `{"route": {"final": "direct"}, "log": {"level": "warn"}}`
	options, err := Merge(ctx, [][]byte{[]byte(userConfig), []byte(override)})
	if err != nil {
		t.Fatal(err)
	}
	if options.Route.Final != "direct" || options.Log.Level != "warn" || len(options.Inbounds) != 1 {
		t.Errorf("unexpected merge result: route=%+v log=%+v", options.Route, options.Log)
	}
	if _, err := Merge(ctx, [][]byte{[]byte(`{"unknown": 1}`)}); err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("unknown fields must be rejected, got %v", err)
	}
}
