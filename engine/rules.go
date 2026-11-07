package engine

import (
	"bufio"
	"fmt"
	"io"
	"net/netip"
	"os"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

// Action is what happens to traffic matched by a rule.
type Action string

const (
	ActionWarp   Action = "warp"
	ActionDirect Action = "direct"
	ActionBlock  Action = "block"
)

// ParseAction validates an action name.
func ParseAction(value string) (Action, error) {
	switch action := Action(strings.ToLower(strings.TrimSpace(value))); action {
	case ActionWarp, ActionDirect, ActionBlock:
		return action, nil
	case "proxy":
		return ActionWarp, nil
	case "bypass":
		return ActionDirect, nil
	case "reject":
		return ActionBlock, nil
	default:
		return "", errors.Newf("unknown action %q (expected warp, direct or block)", value)
	}
}

// Kind selects which property of a connection a matcher looks at. The values
// are the sing-box route rule field names, except for the geo/rule-set kinds,
// which are turned into rule sets.
type Kind string

const (
	KindIPCIDR           Kind = "ip_cidr"
	KindPrivate          Kind = "ip_is_private"
	KindDomain           Kind = "domain"
	KindDomainSuffix     Kind = "domain_suffix"
	KindDomainKeyword    Kind = "domain_keyword"
	KindDomainRegex      Kind = "domain_regex"
	KindProcessName      Kind = "process_name"
	KindProcessPath      Kind = "process_path"
	KindProcessPathRegex Kind = "process_path_regex"
	KindPort             Kind = "port"
	KindPortRange        Kind = "port_range"
	KindProtocol         Kind = "protocol"
	KindNetwork          Kind = "network"
	KindGeoIP            Kind = "geoip"
	KindGeoSite          Kind = "geosite"
	KindRuleSet          Kind = "rule_set"
)

// needsResolve reports whether matching requires the destination IP. Such
// matchers are evaluated after domains are resolved.
func (k Kind) needsResolve() bool {
	switch k {
	case KindIPCIDR, KindPrivate, KindGeoIP, KindRuleSet:
		return true
	}
	return false
}

// matchesDomain reports whether the matcher can be applied to DNS queries.
func (k Kind) matchesDomain() bool {
	switch k {
	case KindDomain, KindDomainSuffix, KindDomainKeyword, KindDomainRegex, KindGeoSite:
		return true
	}
	return false
}

func (k Kind) isProcess() bool {
	switch k {
	case KindProcessName, KindProcessPath, KindProcessPathRegex:
		return true
	}
	return false
}

// Matcher matches one property of a connection.
type Matcher struct {
	Kind  Kind
	Value string
}

func (m Matcher) String() string {
	if m.Kind == KindPrivate {
		return "private"
	}
	return string(m.Kind) + ":" + m.Value
}

// Rule routes traffic matched by Matcher to Action.
type Rule struct {
	Action  Action
	Matcher Matcher
}

var prefixKinds = map[string]Kind{
	"ip":                 KindIPCIDR,
	"cidr":               KindIPCIDR,
	"ip_cidr":            KindIPCIDR,
	"domain":             KindDomain,
	"full":               KindDomain,
	"suffix":             KindDomainSuffix,
	"domain_suffix":      KindDomainSuffix,
	"keyword":            KindDomainKeyword,
	"domain_keyword":     KindDomainKeyword,
	"regex":              KindDomainRegex,
	"domain_regex":       KindDomainRegex,
	"app":                KindProcessName,
	"process":            KindProcessName,
	"process_name":       KindProcessName,
	"path":               KindProcessPath,
	"process_path":       KindProcessPath,
	"path_regex":         KindProcessPathRegex,
	"process_path_regex": KindProcessPathRegex,
	"port":               KindPort,
	"protocol":           KindProtocol,
	"network":            KindNetwork,
	"geoip":              KindGeoIP,
	"geosite":            KindGeoSite,
	"ruleset":            KindRuleSet,
	"rule_set":           KindRuleSet,
}

// ParseMatcher parses a matcher such as "10.0.0.0/8", "suffix:example.com",
// "app:chrome.exe" or "geoip:ir". A value without a prefix is treated as an
// IP/CIDR when it parses as one and as a domain suffix otherwise.
func ParseMatcher(value string) (Matcher, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Matcher{}, errors.New("empty matcher")
	}
	switch strings.ToLower(value) {
	case "private", "lan":
		return Matcher{Kind: KindPrivate}, nil
	}
	// Bare IPs and CIDRs first: IPv6 addresses contain colons.
	if prefix, ok := parsePrefix(value); ok {
		return Matcher{Kind: KindIPCIDR, Value: prefix}, nil
	}
	if name, rest, found := strings.Cut(value, ":"); found {
		if kind, ok := prefixKinds[strings.ToLower(name)]; ok {
			return newMatcher(kind, strings.TrimSpace(rest))
		}
	}
	if strings.ContainsAny(value, "/\\:") || strings.Contains(value, " ") {
		return Matcher{}, errors.Newf("cannot parse matcher %q; use a prefix such as ip:, suffix:, app: or path:", value)
	}
	return newMatcher(KindDomainSuffix, value)
}

func newMatcher(kind Kind, value string) (Matcher, error) {
	if value == "" {
		return Matcher{}, errors.Newf("missing value for %s matcher", kind)
	}
	switch kind {
	case KindIPCIDR:
		prefix, ok := parsePrefix(value)
		if !ok {
			return Matcher{}, errors.Newf("invalid IP or CIDR %q", value)
		}
		value = prefix
	case KindDomain, KindDomainSuffix:
		value = strings.ToLower(strings.TrimPrefix(value, "."))
	case KindPort:
		return parsePort(value)
	case KindGeoIP, KindGeoSite:
		value = strings.ToLower(value)
	case KindNetwork:
		value = strings.ToLower(value)
		if value != "tcp" && value != "udp" {
			return Matcher{}, errors.Newf("invalid network %q (expected tcp or udp)", value)
		}
	case KindProtocol:
		value = strings.ToLower(value)
	}
	return Matcher{Kind: kind, Value: value}, nil
}

func parsePrefix(value string) (string, bool) {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked().String(), true
	}
	if addr, err := netip.ParseAddr(value); err == nil {
		return netip.PrefixFrom(addr, addr.BitLen()).String(), true
	}
	return "", false
}

// parsePort accepts "443", "1000-2000", "1000:2000", ":3000" and "4000:".
func parsePort(value string) (Matcher, error) {
	if port, err := strconv.ParseUint(value, 10, 16); err == nil {
		return Matcher{Kind: KindPort, Value: strconv.FormatUint(port, 10)}, nil
	}
	start, end, found := strings.Cut(strings.Replace(value, "-", ":", 1), ":")
	if !found {
		return Matcher{}, errors.Newf("invalid port %q", value)
	}
	for _, bound := range []string{start, end} {
		if bound == "" {
			continue
		}
		if _, err := strconv.ParseUint(bound, 10, 16); err != nil {
			return Matcher{}, errors.Newf("invalid port range %q", value)
		}
	}
	if start == "" && end == "" {
		return Matcher{}, errors.Newf("invalid port range %q", value)
	}
	return Matcher{Kind: KindPortRange, Value: start + ":" + end}, nil
}

// SplitMatchers splits a comma separated list of matchers. Regex matchers are
// never split, as commas are meaningful inside regular expressions.
func SplitMatchers(value string) []string {
	trimmed := strings.TrimSpace(value)
	if name, _, found := strings.Cut(trimmed, ":"); found {
		if kind, ok := prefixKinds[strings.ToLower(name)]; ok &&
			(kind == KindDomainRegex || kind == KindProcessPathRegex) {
			return []string{trimmed}
		}
	}
	var result []string
	for _, part := range strings.Split(trimmed, ",") {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

// ParseRules parses matcher values for a single action.
func ParseRules(action Action, values []string) ([]Rule, error) {
	var rules []Rule
	for _, value := range values {
		for _, part := range SplitMatchers(value) {
			matcher, err := ParseMatcher(part)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			rules = append(rules, Rule{Action: action, Matcher: matcher})
		}
	}
	return rules, nil
}

// ParseRulesFile reads rules from a file with one "<action> <matcher>..."
// entry per line. Blank lines and lines starting with # are ignored.
func ParseRulesFile(path string) ([]Rule, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "open rules file")
	}
	defer file.Close()
	rules, err := ParseRulesReader(file)
	if err != nil {
		return nil, errors.Wrapf(err, "rules file %s", path)
	}
	return rules, nil
}

// splitLineMatchers splits the matchers of a rules file line at whitespace.
// Paths and regular expressions may contain spaces, so such a matcher
// extends to the end of the line.
func splitLineMatchers(rest string) []string {
	var result []string
	for {
		rest = strings.TrimSpace(rest)
		if rest == "" {
			return result
		}
		if name, _, found := strings.Cut(rest, ":"); found {
			switch prefixKinds[strings.ToLower(name)] {
			case KindProcessPath, KindProcessPathRegex, KindDomainRegex:
				return append(result, rest)
			}
		}
		end := strings.IndexAny(rest, " \t")
		if end < 0 {
			return append(result, rest)
		}
		result = append(result, rest[:end])
		rest = rest[end:]
	}
}

// ParseRulesReader is ParseRulesFile for an arbitrary reader.
func ParseRulesReader(reader io.Reader) ([]Rule, error) {
	var rules []Rule
	scanner := bufio.NewScanner(reader)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		actionName, rest, _ := strings.Cut(line, " ")
		if tab := strings.IndexByte(actionName, '\t'); tab >= 0 {
			actionName, rest = line[:tab], line[tab+1:]
		}
		matchers := splitLineMatchers(rest)
		if len(matchers) == 0 {
			return nil, fmt.Errorf("line %d: expected \"<action> <matcher>\"", lineNumber)
		}
		action, err := ParseAction(actionName)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		lineRules, err := ParseRules(action, matchers)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		rules = append(rules, lineRules...)
	}
	return rules, errors.WithStack(scanner.Err())
}
