// Package engine builds and runs sing-box configurations that route traffic
// through Cloudflare WARP.
package engine

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

const (
	DefaultWarpTag = "warp"
	TagDirect      = "direct"
	tagDNSRemote   = "dns-remote"
	tagDNSDirect   = "dns-direct"
	tagHTTPClient  = "harp-http"
	tagTun         = "tun-in"

	DefaultGeoIPURL   = "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-%s.srs"
	DefaultGeoSiteURL = "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-%s.srs"
)

// Warp describes the WARP WireGuard endpoint.
type Warp struct {
	Tag           string
	PrivateKey    string
	Address       []string
	PeerPublicKey string
	Server        string
	Port          uint16
	Reserved      []byte
	MTU           uint32
	Keepalive     uint16
}

// User is a username/password pair for proxy authentication.
type User struct {
	Username string
	Password string
}

// Listener is a local proxy server.
type Listener struct {
	// Type is mixed (SOCKS4/5 and HTTP on one port), socks or http.
	Type           string
	Listen         string
	Port           uint16
	Users          []User
	SetSystemProxy bool
}

// Tun configures system-wide tunneling through a virtual interface.
type Tun struct {
	InterfaceName       string
	MTU                 uint32
	Stack               string
	Address             []string
	StrictRoute         bool
	AutoRedirect        bool
	RouteAddress        []string
	RouteExcludeAddress []string
}

// DNS configures name resolution.
type DNS struct {
	// Remote is used for traffic sent through WARP and is itself queried
	// through WARP.
	Remote string
	// Direct is used for traffic that bypasses WARP: "local" for the system
	// resolver or a server URL.
	Direct   string
	Strategy string
}

// Log configures logging.
type Log struct {
	Level     string
	Output    string
	Timestamp bool
}

// Options is everything needed to generate a sing-box configuration.
type Options struct {
	Log        Log
	Warp       Warp
	Listeners  []Listener
	Tun        *Tun
	Rules      []Rule
	Final      Action
	BypassLAN  bool
	DNS        DNS
	ClashAPI   string
	APISecret  string
	CacheFile  string
	GeoIPURL   string
	GeoSiteURL string
}

type object = map[string]any

// Build generates a sing-box configuration.
func Build(o Options) (object, error) {
	if o.Warp.Tag == "" {
		o.Warp.Tag = DefaultWarpTag
	}
	if o.Final == "" {
		o.Final = ActionWarp
	}
	if o.GeoIPURL == "" {
		o.GeoIPURL = DefaultGeoIPURL
	}
	if o.GeoSiteURL == "" {
		o.GeoSiteURL = DefaultGeoSiteURL
	}
	if len(o.Listeners) == 0 && o.Tun == nil {
		return nil, errors.New("nothing to run: configure at least one proxy listener or TUN")
	}

	inbounds, inboundTags, err := buildInbounds(o)
	if err != nil {
		return nil, err
	}
	b := &builder{options: o, ruleSetIndex: map[string]bool{}, customSources: map[string]string{}}
	route, err := b.route(inboundTags)
	if err != nil {
		return nil, err
	}
	dns, err := b.dns()
	if err != nil {
		return nil, err
	}

	config := object{
		"log":       buildLog(o.Log),
		"dns":       dns,
		"endpoints": []any{WarpEndpoint(o.Warp)},
		"inbounds":  inbounds,
		"outbounds": []any{object{"type": "direct", "tag": TagDirect}},
		"route":     route,
	}
	if b.remoteRuleSets {
		// Rule sets are downloaded through WARP, which also works where
		// GitHub is filtered.
		config["http_clients"] = []any{object{"tag": tagHTTPClient, "detour": o.Warp.Tag}}
		route["default_http_client"] = tagHTTPClient
	}
	if experimental := buildExperimental(o); len(experimental) != 0 {
		config["experimental"] = experimental
	}
	return config, nil
}

// WarpEndpoint returns the sing-box WireGuard endpoint for WARP.
func WarpEndpoint(w Warp) object {
	if w.Tag == "" {
		w.Tag = DefaultWarpTag
	}
	peer := object{
		"address":     w.Server,
		"port":        w.Port,
		"public_key":  w.PeerPublicKey,
		"allowed_ips": []string{"0.0.0.0/0", "::/0"},
	}
	if w.Keepalive != 0 {
		peer["persistent_keepalive_interval"] = w.Keepalive
	}
	if len(w.Reserved) != 0 {
		// Encoded as numbers: a []byte would be marshalled as base64.
		reserved := make([]int, len(w.Reserved))
		for i, b := range w.Reserved {
			reserved[i] = int(b)
		}
		peer["reserved"] = reserved
	}
	endpoint := object{
		"type":        "wireguard",
		"tag":         w.Tag,
		"address":     w.Address,
		"private_key": w.PrivateKey,
		"peers":       []any{peer},
		// sing-box sets Don't Fragment on WireGuard sockets. Networks that
		// lower the path MTU (common where WireGuard is throttled) then make
		// every full sized packet fail with "message too long"; letting the
		// OS fragment keeps the tunnel working.
		"udp_fragment": true,
	}
	if w.MTU != 0 {
		endpoint["mtu"] = w.MTU
	}
	if _, err := netip.ParseAddr(w.Server); err != nil {
		endpoint["domain_resolver"] = tagDNSDirect
	}
	return endpoint
}

func buildLog(l Log) object {
	result := object{}
	if l.Level != "" {
		result["level"] = l.Level
	}
	if l.Output != "" {
		result["output"] = l.Output
	}
	if l.Timestamp {
		result["timestamp"] = true
	}
	return result
}

func buildExperimental(o Options) object {
	result := object{}
	if o.CacheFile != "" {
		result["cache_file"] = object{"enabled": true, "path": o.CacheFile}
	}
	if o.ClashAPI != "" {
		api := object{"external_controller": o.ClashAPI}
		if o.APISecret != "" {
			api["secret"] = o.APISecret
		}
		result["clash_api"] = api
	}
	return result
}

func buildInbounds(o Options) ([]any, []string, error) {
	var inbounds []any
	var tags []string
	counts := map[string]int{}
	for _, listener := range o.Listeners {
		listenerType, err := normalizeListenerType(listener.Type)
		if err != nil {
			return nil, nil, err
		}
		counts[listenerType]++
		tag := listenerType + "-in"
		if counts[listenerType] > 1 {
			tag += "-" + strconv.Itoa(counts[listenerType])
		}
		listen := listener.Listen
		if listen == "" {
			listen = "127.0.0.1"
		}
		inbound := object{
			"type":        listenerType,
			"tag":         tag,
			"listen":      listen,
			"listen_port": listener.Port,
		}
		if len(listener.Users) != 0 {
			users := make([]any, len(listener.Users))
			for i, user := range listener.Users {
				users[i] = object{"username": user.Username, "password": user.Password}
			}
			inbound["users"] = users
		}
		if listener.SetSystemProxy {
			if listenerType == "socks" {
				return nil, nil, errors.New("system proxy requires a mixed or http listener")
			}
			inbound["set_system_proxy"] = true
		}
		inbounds = append(inbounds, inbound)
		tags = append(tags, tag)
	}
	if t := o.Tun; t != nil {
		address := t.Address
		if len(address) == 0 {
			address = []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"}
		}
		inbound := object{
			"type":       "tun",
			"tag":        tagTun,
			"address":    address,
			"auto_route": true,
		}
		if t.InterfaceName != "" {
			inbound["interface_name"] = t.InterfaceName
		}
		if t.MTU != 0 {
			inbound["mtu"] = t.MTU
		}
		if t.Stack != "" {
			inbound["stack"] = t.Stack
		}
		if t.StrictRoute {
			inbound["strict_route"] = true
		}
		if t.AutoRedirect {
			inbound["auto_redirect"] = true
		}
		if len(t.RouteAddress) != 0 {
			inbound["route_address"] = t.RouteAddress
		}
		if len(t.RouteExcludeAddress) != 0 {
			inbound["route_exclude_address"] = t.RouteExcludeAddress
		}
		inbounds = append(inbounds, inbound)
		tags = append(tags, tagTun)
	}
	return inbounds, tags, nil
}

func normalizeListenerType(value string) (string, error) {
	switch strings.ToLower(value) {
	case "", "mixed":
		return "mixed", nil
	case "socks", "socks5", "socks5h", "socks4", "socks4a":
		return "socks", nil
	case "http":
		return "http", nil
	default:
		return "", errors.Newf("unsupported listener type %q (expected mixed, socks or http)", value)
	}
}

// ParseListener parses "[type://][user:pass@]host:port". The type defaults
// to mixed and the host to 127.0.0.1.
func ParseListener(value string) (Listener, error) {
	if !strings.Contains(value, "://") {
		value = "mixed://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return Listener{}, errors.Wrapf(err, "invalid listener %q", value)
	}
	listenerType, err := normalizeListenerType(parsed.Scheme)
	if err != nil {
		return Listener{}, err
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return Listener{}, errors.Newf("invalid listener %q: unexpected path", value)
	}
	host, portString, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		return Listener{}, errors.Wrapf(err, "invalid listener %q", value)
	}
	port, err := strconv.ParseUint(portString, 10, 16)
	if err != nil || port == 0 {
		return Listener{}, errors.Newf("invalid listener %q: bad port", value)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	listener := Listener{Type: listenerType, Listen: host, Port: uint16(port)}
	if parsed.User != nil {
		password, _ := parsed.User.Password()
		listener.Users = append(listener.Users, User{Username: parsed.User.Username(), Password: password})
	}
	return listener, nil
}

// ParseUser parses "username:password".
func ParseUser(value string) (User, error) {
	username, password, found := strings.Cut(value, ":")
	if !found || username == "" {
		return User{}, errors.Newf("invalid user %q (expected username:password)", value)
	}
	return User{Username: username, Password: password}, nil
}

type builder struct {
	options        Options
	ruleSets       []any
	ruleSetIndex   map[string]bool
	remoteRuleSets bool
	// customSources maps a custom rule-set path or URL to its tag.
	customSources map[string]string
}

// actionOrder is the order in which actions are evaluated: explicit blocks
// first, then WARP overrides, then bypasses.
var actionOrder = []Action{ActionBlock, ActionWarp, ActionDirect}

// group collects the values of the rules matching filter, grouped by action
// and kind, preserving first-seen order.
type ruleGroup struct {
	action Action
	kind   Kind
	values []string
}

func (b *builder) groups(filter func(Kind) bool) []ruleGroup {
	var groups []ruleGroup
	for _, action := range actionOrder {
		var kinds []Kind
		values := map[Kind][]string{}
		for _, rule := range b.options.Rules {
			if rule.Action != action || !filter(rule.Matcher.Kind) {
				continue
			}
			kind := rule.Matcher.Kind
			if _, seen := values[kind]; !seen {
				kinds = append(kinds, kind)
			}
			if !slices.Contains(values[kind], rule.Matcher.Value) {
				values[kind] = append(values[kind], rule.Matcher.Value)
			}
		}
		for _, kind := range kinds {
			groups = append(groups, ruleGroup{action: action, kind: kind, values: values[kind]})
		}
	}
	return groups
}

func (b *builder) route(inboundTags []string) (object, error) {
	o := b.options
	rules := []any{
		object{"action": "sniff"},
		object{"protocol": "dns", "action": "hijack-dns"},
	}
	if o.BypassLAN {
		rules = append(rules, object{"ip_is_private": true, "outbound": TagDirect})
	}
	addGroups := func(groups []ruleGroup) error {
		for _, group := range groups {
			rule, err := b.matchFields(group.kind, group.values)
			if err != nil {
				return err
			}
			for key, value := range routeAction(group.action, o.Warp.Tag) {
				rule[key] = value
			}
			rules = append(rules, rule)
		}
		return nil
	}
	if err := addGroups(b.groups(func(k Kind) bool { return !k.needsResolve() })); err != nil {
		return nil, err
	}
	ipGroups := b.groups(Kind.needsResolve)
	if len(ipGroups) != 0 {
		// Resolve domains so that IP based rules also apply to them.
		rules = append(rules, object{"action": "resolve"})
		if err := addGroups(ipGroups); err != nil {
			return nil, err
		}
	}

	final := o.Warp.Tag
	switch o.Final {
	case ActionDirect:
		final = TagDirect
	case ActionBlock:
		rules = append(rules, object{"inbound": inboundTags, "action": "reject"})
		final = TagDirect
	}

	route := object{
		"rules":                   rules,
		"final":                   final,
		"default_domain_resolver": tagDNSDirect,
	}
	if len(b.ruleSets) != 0 {
		route["rule_set"] = b.ruleSets
	}
	if slices.ContainsFunc(o.Rules, func(r Rule) bool { return r.Matcher.Kind.isProcess() }) {
		route["find_process"] = true
	}
	if o.Tun != nil {
		route["auto_detect_interface"] = true
	}
	return route, nil
}

func routeAction(action Action, warpTag string) object {
	switch action {
	case ActionBlock:
		return object{"action": "reject"}
	case ActionDirect:
		return object{"action": "route", "outbound": TagDirect}
	default:
		return object{"action": "route", "outbound": warpTag}
	}
}

// matchFields returns the sing-box rule fields matching values of kind.
func (b *builder) matchFields(kind Kind, values []string) (object, error) {
	switch kind {
	case KindPrivate:
		return object{"ip_is_private": true}, nil
	case KindPort:
		ports := make([]int, len(values))
		for i, value := range values {
			port, err := strconv.Atoi(value)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			ports[i] = port
		}
		return object{"port": ports}, nil
	case KindNetwork:
		if len(values) == 1 {
			return object{"network": values[0]}, nil
		}
		return object{"network": values}, nil
	case KindGeoIP, KindGeoSite, KindRuleSet:
		tags := make([]string, len(values))
		for i, value := range values {
			tag, err := b.ruleSet(kind, value)
			if err != nil {
				return nil, err
			}
			tags[i] = tag
		}
		return object{"rule_set": tags}, nil
	default:
		return object{string(kind): values}, nil
	}
}

// ruleSet registers the rule set for a geo/rule-set matcher and returns its tag.
func (b *builder) ruleSet(kind Kind, value string) (string, error) {
	var tag string
	var definition object
	switch kind {
	case KindGeoIP:
		tag = "geoip-" + value
		definition = remoteRuleSet(tag, fmt.Sprintf(b.options.GeoIPURL, value))
	case KindGeoSite:
		tag = "geosite-" + value
		definition = remoteRuleSet(tag, fmt.Sprintf(b.options.GeoSiteURL, value))
	default:
		if existing, ok := b.customSources[value]; ok {
			return existing, nil
		}
		tag = "custom-" + strconv.Itoa(len(b.customSources)+1)
		b.customSources[value] = tag
		if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
			definition = remoteRuleSet(tag, value)
		} else {
			definition = object{"type": "local", "tag": tag, "format": ruleSetFormat(value), "path": value}
		}
	}
	if !b.ruleSetIndex[tag] {
		b.ruleSetIndex[tag] = true
		if definition["type"] == "remote" {
			b.remoteRuleSets = true
		}
		b.ruleSets = append(b.ruleSets, definition)
	}
	return tag, nil
}

func remoteRuleSet(tag, url string) object {
	return object{"type": "remote", "tag": tag, "format": ruleSetFormat(url), "url": url}
}

func ruleSetFormat(source string) string {
	if strings.HasSuffix(strings.ToLower(source), ".json") {
		return "source"
	}
	return "binary"
}

func (b *builder) dns() (object, error) {
	o := b.options
	remoteValue := o.DNS.Remote
	if remoteValue == "" {
		remoteValue = "https://1.1.1.1/dns-query"
	}
	directValue := o.DNS.Direct
	if directValue == "" {
		directValue = "local"
	}
	remote, remoteHost, err := DNSServer(tagDNSRemote, remoteValue)
	if err != nil {
		return nil, errors.Wrap(err, "remote DNS")
	}
	remote["detour"] = o.Warp.Tag
	direct, directHost, err := DNSServer(tagDNSDirect, directValue)
	if err != nil {
		return nil, errors.Wrap(err, "direct DNS")
	}
	if directHost != "" {
		// The direct resolver cannot resolve its own name; use the system.
		return nil, errors.Newf("direct DNS server must be an IP address or \"local\", got %q", directHost)
	}

	var rules []any
	if remoteHost != "" {
		// Resolving the remote server's own name through itself would loop.
		rules = append(rules, object{"domain": []string{remoteHost}, "server": tagDNSDirect})
	}
	for _, group := range b.groups(Kind.matchesDomain) {
		rule, err := b.matchFields(group.kind, group.values)
		if err != nil {
			return nil, err
		}
		switch group.action {
		case ActionBlock:
			rule["action"] = "reject"
		case ActionDirect:
			rule["server"] = tagDNSDirect
		default:
			rule["server"] = tagDNSRemote
		}
		rules = append(rules, rule)
	}

	final := tagDNSRemote
	if o.Final == ActionDirect {
		final = tagDNSDirect
	}
	dns := object{
		"servers": []any{remote, direct},
		"final":   final,
	}
	if len(rules) != 0 {
		dns["rules"] = rules
	}
	if o.DNS.Strategy != "" {
		dns["strategy"] = o.DNS.Strategy
	}
	return dns, nil
}

// DNSServer converts a server URL into a sing-box DNS server. Supported forms:
// "local", "1.1.1.1", "udp://1.1.1.1:53", "tcp://", "tls://", "quic://",
// "https://1.1.1.1/dns-query" and "h3://". The returned host is non-empty
// when the server is addressed by a domain name.
func DNSServer(tag, value string) (object, string, error) {
	if value == "local" || value == "system" {
		return object{"type": "local", "tag": tag}, "", nil
	}
	if !strings.Contains(value, "://") {
		value = "udp://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, "", errors.Wrapf(err, "invalid DNS server %q", value)
	}
	serverType := strings.ToLower(parsed.Scheme)
	switch serverType {
	case "udp", "tcp", "tls", "quic", "https", "h3":
	default:
		return nil, "", errors.Newf("unsupported DNS server scheme %q", parsed.Scheme)
	}
	host := parsed.Hostname()
	if host == "" {
		return nil, "", errors.Newf("invalid DNS server %q: missing host", value)
	}
	server := object{"type": serverType, "tag": tag, "server": host}
	if portString := parsed.Port(); portString != "" {
		port, err := strconv.ParseUint(portString, 10, 16)
		if err != nil {
			return nil, "", errors.Newf("invalid DNS server %q: bad port", value)
		}
		server["server_port"] = port
	}
	if (serverType == "https" || serverType == "h3") && parsed.Path != "" && parsed.Path != "/dns-query" {
		server["path"] = parsed.Path
	}
	domain := ""
	if _, err := netip.ParseAddr(host); err != nil {
		domain = host
	}
	return server, domain, nil
}
