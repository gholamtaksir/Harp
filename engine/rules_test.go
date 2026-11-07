package engine

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseMatcher(t *testing.T) {
	tests := []struct {
		input string
		want  Matcher
	}{
		{"10.0.0.1/8", Matcher{KindIPCIDR, "10.0.0.0/8"}},
		{"1.2.3.4", Matcher{KindIPCIDR, "1.2.3.4/32"}},
		{"2001:db8::1", Matcher{KindIPCIDR, "2001:db8::1/128"}},
		{"ip:192.168.1.0/24", Matcher{KindIPCIDR, "192.168.1.0/24"}},
		{"example.com", Matcher{KindDomainSuffix, "example.com"}},
		{".Example.COM", Matcher{KindDomainSuffix, "example.com"}},
		{"domain:chat.openai.com", Matcher{KindDomain, "chat.openai.com"}},
		{"keyword:google", Matcher{KindDomainKeyword, "google"}},
		{"regex:^a{1,3}\\.com$", Matcher{KindDomainRegex, "^a{1,3}\\.com$"}},
		{"app:chrome.exe", Matcher{KindProcessName, "chrome.exe"}},
		{`path:C:\Program Files\App\app.exe`, Matcher{KindProcessPath, `C:\Program Files\App\app.exe`}},
		{"port:443", Matcher{KindPort, "443"}},
		{"port:1000-2000", Matcher{KindPortRange, "1000:2000"}},
		{"port:4000:", Matcher{KindPortRange, "4000:"}},
		{"geoip:IR", Matcher{KindGeoIP, "ir"}},
		{"geosite:category-ads-all", Matcher{KindGeoSite, "category-ads-all"}},
		{"ruleset:https://example.com/list.srs", Matcher{KindRuleSet, "https://example.com/list.srs"}},
		{"protocol:QUIC", Matcher{KindProtocol, "quic"}},
		{"network:udp", Matcher{KindNetwork, "udp"}},
		{"private", Matcher{Kind: KindPrivate}},
		{"LAN", Matcher{Kind: KindPrivate}},
	}
	for _, test := range tests {
		got, err := ParseMatcher(test.input)
		if err != nil {
			t.Errorf("ParseMatcher(%q) error: %v", test.input, err)
			continue
		}
		if got != test.want {
			t.Errorf("ParseMatcher(%q) = %+v, want %+v", test.input, got, test.want)
		}
	}
}

func TestParseMatcherErrors(t *testing.T) {
	for _, input := range []string{
		"", "foo:bar", `C:\Windows\app.exe`, "port:abc", "port:-", "port:70000",
		"ip:300.1.1.1", "network:icmp", "app:", "two words",
	} {
		if _, err := ParseMatcher(input); err == nil {
			t.Errorf("ParseMatcher(%q) succeeded, want error", input)
		}
	}
}

func TestSplitMatchers(t *testing.T) {
	if got := SplitMatchers(" a.com, ,geoip:ir ,"); !reflect.DeepEqual(got, []string{"a.com", "geoip:ir"}) {
		t.Errorf("SplitMatchers = %q", got)
	}
	if got := SplitMatchers("regex:^a{1,3}$"); !reflect.DeepEqual(got, []string{"regex:^a{1,3}$"}) {
		t.Errorf("regex was split: %q", got)
	}
}

func TestParseAction(t *testing.T) {
	for input, want := range map[string]Action{
		"warp": ActionWarp, "PROXY": ActionWarp, "direct": ActionDirect,
		"bypass": ActionDirect, "block": ActionBlock, "reject": ActionBlock,
	} {
		if got, err := ParseAction(input); err != nil || got != want {
			t.Errorf("ParseAction(%q) = %q, %v", input, got, err)
		}
	}
	if _, err := ParseAction("drop"); err == nil {
		t.Error("ParseAction(drop) succeeded")
	}
}

func TestParseRulesReader(t *testing.T) {
	input := `
# comment
direct geoip:ir suffix:ir
block  geosite:category-ads-all

warp app:chrome.exe,app:firefox.exe
direct	suffix:a.com path:C:\Program Files\My App\app.exe
`
	rules, err := ParseRulesReader(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	want := []Rule{
		{ActionDirect, Matcher{KindGeoIP, "ir"}},
		{ActionDirect, Matcher{KindDomainSuffix, "ir"}},
		{ActionBlock, Matcher{KindGeoSite, "category-ads-all"}},
		{ActionWarp, Matcher{KindProcessName, "chrome.exe"}},
		{ActionWarp, Matcher{KindProcessName, "firefox.exe"}},
		{ActionDirect, Matcher{KindDomainSuffix, "a.com"}},
		{ActionDirect, Matcher{KindProcessPath, `C:\Program Files\My App\app.exe`}},
	}
	if !reflect.DeepEqual(rules, want) {
		t.Errorf("rules = %+v\nwant %+v", rules, want)
	}

	for _, bad := range []string{"direct", "allow 1.2.3.4", "direct foo:bar"} {
		if _, err := ParseRulesReader(strings.NewReader(bad)); err == nil {
			t.Errorf("ParseRulesReader(%q) succeeded", bad)
		}
	}
}
