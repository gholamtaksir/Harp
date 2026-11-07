package shared

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gholamtaksir/Harp/engine"
	"github.com/gholamtaksir/Harp/privilege"
	"github.com/gholamtaksir/Harp/scanner"
	"github.com/gholamtaksir/Harp/warp"
	"github.com/sagernet/sing-box/option"
	"github.com/spf13/cobra"
)

// WarpFlags configure the WARP WireGuard endpoint.
type WarpFlags struct {
	Tag        string
	Endpoint   string
	MTU        uint32
	Keepalive  uint16
	Refresh    bool
	NoReserved bool
}

func (f *WarpFlags) Register(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.StringVar(&f.Endpoint, "endpoint", "", `WARP endpoint (host:port), e.g. 188.114.98.2:1701, or "auto" to use one that answers (see "harp scan")`)
	flags.Uint32Var(&f.MTU, "warp-mtu", 1280, "MTU of the WARP WireGuard tunnel")
	flags.Uint16Var(&f.Keepalive, "keepalive", 25, "WireGuard persistent keepalive interval in seconds (0 disables)")
	flags.BoolVar(&f.Refresh, "refresh", false, "Fetch fresh peer configuration from the Cloudflare API before connecting")
	flags.BoolVar(&f.NoReserved, "no-reserved", false, "Do not send the WARP client id in the WireGuard reserved bytes")
}

// Build loads the current account and returns its WARP endpoint.
func (f *WarpFlags) Build() (engine.Warp, error) {
	if err := EnsureConfigValidAccount(); err != nil {
		return engine.Warp{}, errors.WithStack(err)
	}
	profile, err := warp.Load(CreateContext(), f.Refresh)
	if err != nil {
		return engine.Warp{}, errors.WithStack(err)
	}
	reserved := profile.Reserved
	if f.NoReserved {
		reserved = nil
	}
	endpoint, fallback := profile.Endpoint, profile.EndpointIP
	if strings.EqualFold(f.Endpoint, "auto") {
		chosen, err := autoEndpoint(profile, reserved)
		if err != nil {
			return engine.Warp{}, err
		}
		endpoint, fallback = chosen, ""
	} else if f.Endpoint != "" {
		endpoint, fallback = f.Endpoint, ""
	}
	server, port, err := engine.ResolveEndpoint(endpoint, fallback)
	if err != nil {
		return engine.Warp{}, errors.WithStack(err)
	}
	address, err := interfaceAddresses(profile)
	if err != nil {
		return engine.Warp{}, err
	}
	result := engine.Warp{
		Tag:           f.Tag,
		PrivateKey:    profile.PrivateKey,
		Address:       address,
		PeerPublicKey: profile.PeerPublicKey,
		Server:        server,
		Port:          port,
		Reserved:      reserved,
		MTU:           f.MTU,
		Keepalive:     f.Keepalive,
	}
	return result, nil
}

// KnownEndpoints returns the account's own endpoint and well-known ones.
func KnownEndpoints(profile *warp.Profile) []netip.AddrPort {
	var result []netip.AddrPort
	if server, port, err := engine.ResolveEndpoint(profile.Endpoint, profile.EndpointIP); err == nil {
		if address, err := netip.ParseAddr(server); err == nil {
			result = append(result, netip.AddrPortFrom(address, port))
		}
	}
	return append(result, scanner.WellKnown...)
}

// autoEndpoint returns the fastest known endpoint that answers a WireGuard
// handshake, scanning for one if none does.
func autoEndpoint(profile *warp.Profile, reserved []byte) (string, error) {
	keys, err := scanner.NewKeys(profile.PrivateKey, profile.PeerPublicKey, reserved)
	if err != nil {
		return "", errors.WithStack(err)
	}
	ctx := context.Background()
	if answered := keys.Quick(ctx, KnownEndpoints(profile), 2*time.Second); len(answered) != 0 {
		log.Printf("Using endpoint %s (%d ms)", answered[0].Endpoint, answered[0].RTT.Milliseconds())
		return answered[0].Endpoint.String(), nil
	}
	log.Println("No well-known endpoint answered, scanning for one")
	candidates := scanner.Candidates(300, scanner.IPv4Prefixes, scanner.Ports, nil)
	results := keys.Scan(ctx, scanner.Options{Candidates: candidates, Measure: 10}, nil)
	if len(results) == 0 {
		return "", errors.New("no WARP endpoint answers on this network; it may block WireGuard entirely")
	}
	log.Printf("Using endpoint %s (%d ms, %d of %d answered)", results[0].Endpoint, results[0].RTT.Milliseconds(), len(results), len(candidates))
	return results[0].Endpoint.String(), nil
}

func interfaceAddresses(profile *warp.Profile) ([]string, error) {
	var result []string
	for _, value := range []string{profile.IPv4, profile.IPv6} {
		if value == "" {
			continue
		}
		address, err := netip.ParseAddr(value)
		if err != nil {
			return nil, errors.Newf("invalid WARP interface address %q in account file", value)
		}
		result = append(result, netip.PrefixFrom(address, address.BitLen()).String())
	}
	if len(result) == 0 {
		return nil, errors.New("account file has no WARP interface address; run with --refresh")
	}
	return result, nil
}

// EngineFlags configure proxy listeners, routing, DNS and logging.
type EngineFlags struct {
	Warp WarpFlags

	Listeners   []string
	Users       []string
	SystemProxy bool

	Direct     []string
	WarpRules  []string
	Block      []string
	RulesFiles []string
	Final      string
	NoLAN      bool

	RemoteDNS   string
	DirectDNS   string
	DNSStrategy string

	LogLevel   string
	LogFile    string
	API        string
	APISecret  string
	CacheFile  string
	GeoIPURL   string
	GeoSiteURL string

	Dump bool
}

// Register adds the flags to cmd. defaultListeners are used when no
// --listen flag is given.
func (f *EngineFlags) Register(cmd *cobra.Command, defaultListeners []string) {
	f.Warp.Register(cmd)
	flags := cmd.Flags()

	flags.StringArrayVarP(&f.Listeners, "listen", "l", defaultListeners,
		"Proxy listener as [mixed|socks|http://][user:pass@]host:port (repeatable)")
	flags.StringArrayVarP(&f.Users, "user", "u", nil,
		"Require username:password on listeners without their own credentials (repeatable)")
	flags.BoolVar(&f.SystemProxy, "system-proxy", false, "Set the first mixed/http listener as the system proxy")

	flags.StringArrayVar(&f.Direct, "direct", nil, "Bypass WARP for matching traffic (repeatable, comma separated)")
	flags.StringArrayVar(&f.WarpRules, "warp", nil, "Send matching traffic through WARP (repeatable, comma separated)")
	flags.StringArrayVar(&f.Block, "block", nil, "Block matching traffic (repeatable, comma separated)")
	flags.StringArrayVar(&f.RulesFiles, "rules", nil, "Read rules from a file with \"<action> <matcher>\" lines (repeatable)")
	flags.StringVar(&f.Final, "final", "warp", "Action for traffic matching no rule: warp, direct or block")
	flags.BoolVar(&f.NoLAN, "no-bypass-lan", false, "Do not bypass WARP for private and LAN addresses")

	flags.StringVar(&f.RemoteDNS, "dns", "https://1.1.1.1/dns-query", "DNS server used through WARP (udp://, tcp://, tls://, quic://, https://, h3://)")
	flags.StringVar(&f.DirectDNS, "direct-dns", "local", "DNS server for bypassed traffic: local (system) or a server IP/URL")
	flags.StringVar(&f.DNSStrategy, "dns-strategy", "", "prefer_ipv4, prefer_ipv6, ipv4_only or ipv6_only")

	flags.StringVar(&f.LogLevel, "log-level", "info", "trace, debug, info, warn, error, fatal or panic")
	flags.StringVar(&f.LogFile, "log-file", "", "Write logs to this file instead of the console")
	flags.StringVar(&f.API, "api", "", "Enable the Clash compatible API on this address, e.g. 127.0.0.1:9090")
	flags.StringVar(&f.APISecret, "api-secret", "", "Secret required by the Clash API")
	flags.StringVar(&f.CacheFile, "cache-file", "harp-cache.db", "Cache file for rule sets (empty disables)")
	flags.StringVar(&f.GeoIPURL, "geoip-url", engine.DefaultGeoIPURL, "URL template of geoip:<code> rule sets")
	flags.StringVar(&f.GeoSiteURL, "geosite-url", engine.DefaultGeoSiteURL, "URL template of geosite:<name> rule sets")

	flags.BoolVar(&f.Dump, "dump", false, "Print the generated sing-box configuration and exit (contains your private key)")
}

// Options assembles engine options. tun is nil for proxy-only operation.
func (f *EngineFlags) Options(tun *engine.Tun) (engine.Options, error) {
	listeners, err := f.listeners()
	if err != nil {
		return engine.Options{}, err
	}
	rules, err := f.rules()
	if err != nil {
		return engine.Options{}, err
	}
	final, err := engine.ParseAction(f.Final)
	if err != nil {
		return engine.Options{}, NewUserError(err.Error())
	}
	if !strings.Contains(f.GeoIPURL, "%s") || !strings.Contains(f.GeoSiteURL, "%s") {
		return engine.Options{}, NewUserError("--geoip-url and --geosite-url must contain %s")
	}
	warpEndpoint, err := f.Warp.Build()
	if err != nil {
		return engine.Options{}, err
	}
	return engine.Options{
		Log:        engine.Log{Level: f.LogLevel, Output: f.LogFile, Timestamp: f.LogFile != ""},
		Warp:       warpEndpoint,
		Listeners:  listeners,
		Tun:        tun,
		Rules:      rules,
		Final:      final,
		BypassLAN:  !f.NoLAN,
		DNS:        engine.DNS{Remote: f.RemoteDNS, Direct: f.DirectDNS, Strategy: f.DNSStrategy},
		ClashAPI:   f.API,
		APISecret:  f.APISecret,
		CacheFile:  f.CacheFile,
		GeoIPURL:   f.GeoIPURL,
		GeoSiteURL: f.GeoSiteURL,
	}, nil
}

func (f *EngineFlags) listeners() ([]engine.Listener, error) {
	var users []engine.User
	for _, value := range f.Users {
		user, err := engine.ParseUser(value)
		if err != nil {
			return nil, NewUserError(err.Error())
		}
		users = append(users, user)
	}
	var listeners []engine.Listener
	systemProxySet := false
	for _, value := range f.Listeners {
		listener, err := engine.ParseListener(value)
		if err != nil {
			return nil, NewUserError(err.Error())
		}
		if len(listener.Users) == 0 {
			listener.Users = users
		}
		if f.SystemProxy && !systemProxySet && listener.Type != "socks" {
			listener.SetSystemProxy = true
			systemProxySet = true
		}
		if len(listener.Users) == 0 && !isLoopback(listener.Listen) {
			log.Printf("Warning: %s://%s:%d accepts connections from other machines without authentication; consider --user",
				listener.Type, listener.Listen, listener.Port)
		}
		listeners = append(listeners, listener)
	}
	if f.SystemProxy && !systemProxySet {
		return nil, NewUserError("--system-proxy requires a mixed or http listener")
	}
	return listeners, nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	address, err := netip.ParseAddr(host)
	return err == nil && address.IsLoopback()
}

func (f *EngineFlags) rules() ([]engine.Rule, error) {
	var rules []engine.Rule
	for _, path := range f.RulesFiles {
		fileRules, err := engine.ParseRulesFile(path)
		if err != nil {
			return nil, NewUserError(err.Error())
		}
		rules = append(rules, fileRules...)
	}
	for _, group := range []struct {
		action engine.Action
		values []string
	}{
		{engine.ActionBlock, f.Block},
		{engine.ActionWarp, f.WarpRules},
		{engine.ActionDirect, f.Direct},
	} {
		groupRules, err := engine.ParseRules(group.action, group.values)
		if err != nil {
			return nil, NewUserError(err.Error())
		}
		rules = append(rules, groupRules...)
	}
	return rules, nil
}

// WarnIfNotElevated logs a hint when TUN mode lacks the required privileges.
func WarnIfNotElevated() {
	if !privilege.IsElevated() {
		log.Printf("Warning: TUN mode needs elevated privileges; %s", privilege.Hint)
	}
}

// RunEngine builds the configuration and either prints it or runs it.
func RunEngine(options engine.Options, dump bool) error {
	config, err := engine.Build(options)
	if err != nil {
		return NewUserError(err.Error())
	}
	ctx := engine.NewContext()
	singboxOptions, err := engine.ParseObject(ctx, config)
	if err != nil {
		return errors.WithStack(err)
	}
	return RunOptions(ctx, singboxOptions, dump)
}

// RunOptions prints or runs a parsed sing-box configuration.
func RunOptions(ctx context.Context, options option.Options, dump bool) error {
	if dump {
		content, err := engine.Marshal(ctx, options)
		if err != nil {
			return errors.WithStack(err)
		}
		_, err = fmt.Fprint(os.Stdout, string(content))
		return errors.WithStack(err)
	}
	if err := engine.Run(ctx, options); err != nil {
		// sing-box errors are descriptive; a stack trace would only add noise.
		return NewUserError(err.Error())
	}
	return nil
}
