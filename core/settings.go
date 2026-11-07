// Package core implements the Harp desktop application logic independently
// of any user interface: persistent settings, the connection lifecycle,
// traffic statistics and account management.
package core

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/gholamtaksir/Harp/engine"
)

const (
	ModeProxy = "proxy"
	ModeTun   = "tun"
)

// Listener is a local proxy server.
type Listener struct {
	Type    string `json:"type"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Enabled bool   `json:"enabled"`
}

// User is a proxy credential.
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ProxySettings configure proxy mode.
type ProxySettings struct {
	Listeners   []Listener `json:"listeners"`
	AuthEnabled bool       `json:"authEnabled"`
	Users       []User     `json:"users"`
	SystemProxy bool       `json:"systemProxy"`
}

// TunSettings configure TUN mode.
type TunSettings struct {
	Stack         string   `json:"stack"`
	MTU           int      `json:"mtu"`
	StrictRoute   bool     `json:"strictRoute"`
	AutoRedirect  bool     `json:"autoRedirect"`
	IncludeRoutes []string `json:"includeRoutes"`
	ExcludeRoutes []string `json:"excludeRoutes"`
	// ServeProxy keeps the proxy listeners running in TUN mode.
	ServeProxy bool `json:"serveProxy"`
}

// Rule is a routing rule. Type is a matcher prefix understood by
// engine.ParseMatcher (ip, suffix, domain, keyword, regex, app, path,
// path_regex, port, protocol, network, geoip, geosite, ruleset, private).
type Rule struct {
	ID      string `json:"id"`
	Action  string `json:"action"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
	Note    string `json:"note"`
}

// Matcher returns the rule in engine.ParseMatcher syntax.
func (r Rule) Matcher() string {
	if r.Type == "private" {
		return "private"
	}
	return r.Type + ":" + strings.TrimSpace(r.Value)
}

// RoutingSettings configure routing in both modes.
type RoutingSettings struct {
	Final      string `json:"final"`
	BypassLAN  bool   `json:"bypassLan"`
	Rules      []Rule `json:"rules"`
	GeoIPURL   string `json:"geoipUrl"`
	GeoSiteURL string `json:"geositeUrl"`
}

// DNSSettings configure name resolution.
type DNSSettings struct {
	Remote   string `json:"remote"`
	Direct   string `json:"direct"`
	Strategy string `json:"strategy"`
}

// WarpSettings configure the WARP tunnel.
type WarpSettings struct {
	// Endpoint overrides the server; empty means automatic.
	Endpoint  string `json:"endpoint"`
	MTU       int    `json:"mtu"`
	Keepalive int    `json:"keepalive"`
	Reserved  bool   `json:"reserved"`
}

// AppSettings configure the application itself.
type AppSettings struct {
	StartMinimized  bool   `json:"startMinimized"`
	ConnectOnLaunch bool   `json:"connectOnLaunch"`
	CloseToTray     bool   `json:"closeToTray"`
	Theme           string `json:"theme"`
	LogLevel        string `json:"logLevel"`
	ClashAPI        bool   `json:"clashApi"`
	ClashAPIAddress string `json:"clashApiAddress"`
	ClashAPISecret  string `json:"clashApiSecret"`
}

// Settings is everything the user can configure.
type Settings struct {
	Mode    string          `json:"mode"`
	Proxy   ProxySettings   `json:"proxy"`
	Tun     TunSettings     `json:"tun"`
	Routing RoutingSettings `json:"routing"`
	DNS     DNSSettings     `json:"dns"`
	Warp    WarpSettings    `json:"warp"`
	App     AppSettings     `json:"app"`
}

// DefaultSettings returns the settings of a fresh installation.
func DefaultSettings() Settings {
	return Settings{
		Mode: ModeProxy,
		Proxy: ProxySettings{
			Listeners: []Listener{{Type: "mixed", Address: "127.0.0.1", Port: 1080, Enabled: true}},
			Users:     []User{},
		},
		Tun: TunSettings{
			Stack:         "mixed",
			MTU:           9000,
			StrictRoute:   true,
			IncludeRoutes: []string{},
			ExcludeRoutes: []string{},
		},
		Routing: RoutingSettings{
			Final:      string(engine.ActionWarp),
			BypassLAN:  true,
			Rules:      []Rule{},
			GeoIPURL:   engine.DefaultGeoIPURL,
			GeoSiteURL: engine.DefaultGeoSiteURL,
		},
		DNS: DNSSettings{
			Remote: "https://1.1.1.1/dns-query",
			Direct: "local",
		},
		Warp: WarpSettings{MTU: 1280, Keepalive: 25, Reserved: true},
		App: AppSettings{
			CloseToTray:     true,
			Theme:           "system",
			LogLevel:        "info",
			ClashAPIAddress: "127.0.0.1:9090",
		},
	}
}

// FieldError points at an invalid setting.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError lists every invalid setting.
type ValidationError struct {
	Fields []FieldError
}

func (e *ValidationError) Error() string {
	messages := make([]string, len(e.Fields))
	for i, field := range e.Fields {
		messages[i] = field.Field + ": " + field.Message
	}
	return strings.Join(messages, "; ")
}

// Validate checks the settings and returns a *ValidationError when they are
// invalid.
func (s Settings) Validate() error {
	var fields []FieldError
	add := func(field, format string, args ...any) {
		fields = append(fields, FieldError{Field: field, Message: fmt.Sprintf(format, args...)})
	}

	if s.Mode != ModeProxy && s.Mode != ModeTun {
		add("mode", "must be proxy or tun")
	}
	seenPorts := map[string]bool{}
	for i, listener := range s.Proxy.Listeners {
		field := fmt.Sprintf("proxy.listeners[%d]", i)
		if _, err := engine.ParseListener(fmt.Sprintf("%s://%s", listener.Type, joinHostPort(listener.Address, listener.Port))); err != nil {
			add(field, "%v", err)
		}
		key := fmt.Sprintf("%s:%d", listener.Address, listener.Port)
		if listener.Enabled && seenPorts[key] {
			add(field, "port %d is used by another listener", listener.Port)
		}
		seenPorts[key] = listener.Enabled || seenPorts[key]
	}
	if s.Proxy.AuthEnabled {
		if len(s.Proxy.Users) == 0 {
			add("proxy.users", "add at least one user or disable authentication")
		}
		for i, user := range s.Proxy.Users {
			if user.Username == "" || strings.Contains(user.Username, ":") {
				add(fmt.Sprintf("proxy.users[%d]", i), "username must be non-empty and contain no colon")
			}
		}
	}
	switch s.Tun.Stack {
	case "system", "gvisor", "mixed":
	default:
		add("tun.stack", "must be system, gvisor or mixed")
	}
	if s.Tun.MTU < 576 || s.Tun.MTU > 65535 {
		add("tun.mtu", "must be between 576 and 65535")
	}
	for name, routes := range map[string][]string{"tun.includeRoutes": s.Tun.IncludeRoutes, "tun.excludeRoutes": s.Tun.ExcludeRoutes} {
		for _, route := range routes {
			if _, err := netip.ParsePrefix(route); err != nil {
				add(name, "invalid CIDR %q", route)
			}
		}
	}
	if _, err := engine.ParseAction(s.Routing.Final); err != nil {
		add("routing.final", "%v", err)
	}
	for _, rule := range s.Routing.Rules {
		field := "routing.rules." + rule.ID
		if _, err := engine.ParseAction(rule.Action); err != nil {
			add(field, "%v", err)
		}
		if _, err := engine.ParseMatcher(rule.Matcher()); err != nil {
			add(field, "%v", err)
		}
	}
	for name, url := range map[string]string{"routing.geoipUrl": s.Routing.GeoIPURL, "routing.geositeUrl": s.Routing.GeoSiteURL} {
		if !strings.Contains(url, "%s") {
			add(name, "must contain %%s")
		}
	}
	if _, _, err := engine.DNSServer("remote", s.DNS.Remote); err != nil {
		add("dns.remote", "%v", err)
	}
	if _, host, err := engine.DNSServer("direct", s.DNS.Direct); err != nil {
		add("dns.direct", "%v", err)
	} else if host != "" {
		add("dns.direct", "must be an IP address or local")
	}
	switch s.DNS.Strategy {
	case "", "prefer_ipv4", "prefer_ipv6", "ipv4_only", "ipv6_only":
	default:
		add("dns.strategy", "unknown strategy %q", s.DNS.Strategy)
	}
	if s.Warp.Endpoint != "" {
		if _, _, err := splitHostPort(s.Warp.Endpoint); err != nil {
			add("warp.endpoint", "%v", err)
		}
	}
	if s.Warp.MTU < 576 || s.Warp.MTU > 1500 {
		add("warp.mtu", "must be between 576 and 1500")
	}
	if s.Warp.Keepalive < 0 || s.Warp.Keepalive > 65535 {
		add("warp.keepalive", "must be between 0 and 65535")
	}
	if s.App.ClashAPI {
		if _, _, err := splitHostPort(s.App.ClashAPIAddress); err != nil {
			add("app.clashApiAddress", "%v", err)
		}
	}
	if len(fields) != 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// SettingsStore persists settings as JSON.
type SettingsStore struct {
	path string
}

func NewSettingsStore(path string) *SettingsStore {
	return &SettingsStore{path: path}
}

// Load reads the settings, falling back to defaults for a missing file and
// for fields missing in it.
func (s *SettingsStore) Load() (Settings, error) {
	settings := DefaultSettings()
	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return settings, errors.WithStack(err)
	}
	if err := json.Unmarshal(content, &settings); err != nil {
		return DefaultSettings(), errors.Wrap(err, "parse settings")
	}
	normalize(&settings)
	return settings, nil
}

// Save atomically writes the settings.
func (s *SettingsStore) Save(settings Settings) error {
	content, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return errors.WithStack(err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return errors.WithStack(err)
	}
	temporary := s.path + ".tmp"
	if err := os.WriteFile(temporary, content, 0o600); err != nil {
		return errors.WithStack(err)
	}
	return errors.WithStack(os.Rename(temporary, s.path))
}

// Clone returns a deep copy of s.
func (s Settings) Clone() Settings {
	clone := s
	clone.Proxy.Listeners = append([]Listener(nil), s.Proxy.Listeners...)
	clone.Proxy.Users = append([]User(nil), s.Proxy.Users...)
	clone.Tun.IncludeRoutes = append([]string(nil), s.Tun.IncludeRoutes...)
	clone.Tun.ExcludeRoutes = append([]string(nil), s.Tun.ExcludeRoutes...)
	clone.Routing.Rules = append([]Rule(nil), s.Routing.Rules...)
	normalize(&clone)
	return clone
}

// normalize replaces nil slices so the frontend always receives arrays.
func normalize(s *Settings) {
	if s.Proxy.Listeners == nil {
		s.Proxy.Listeners = []Listener{}
	}
	if s.Proxy.Users == nil {
		s.Proxy.Users = []User{}
	}
	if s.Tun.IncludeRoutes == nil {
		s.Tun.IncludeRoutes = []string{}
	}
	if s.Tun.ExcludeRoutes == nil {
		s.Tun.ExcludeRoutes = []string{}
	}
	if s.Routing.Rules == nil {
		s.Routing.Rules = []Rule{}
	}
}
