package core

import (
	"net"
	"net/netip"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/gholamtaksir/Harp/engine"
	"github.com/gholamtaksir/Harp/warp"
)

func joinHostPort(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func splitHostPort(value string) (string, uint16, error) {
	host, portString, err := net.SplitHostPort(value)
	if err != nil {
		return "", 0, errors.Newf("expected host:port, got %q", value)
	}
	port, err := strconv.ParseUint(portString, 10, 16)
	if err != nil || port == 0 {
		return "", 0, errors.Newf("invalid port in %q", value)
	}
	return host, uint16(port), nil
}

// WarpEndpoint resolves the WARP endpoint for profile according to s.
func WarpEndpoint(s WarpSettings, profile *warp.Profile) (engine.Warp, error) {
	endpoint, fallback := profile.Endpoint, profile.EndpointIP
	if s.Endpoint != "" {
		endpoint, fallback = s.Endpoint, ""
	}
	server, port, err := engine.ResolveEndpoint(endpoint, fallback)
	if err != nil {
		return engine.Warp{}, errors.WithStack(err)
	}
	var address []string
	for _, value := range []string{profile.IPv4, profile.IPv6} {
		if value == "" {
			continue
		}
		parsed, err := netip.ParseAddr(value)
		if err != nil {
			return engine.Warp{}, errors.Newf("invalid WARP interface address %q", value)
		}
		address = append(address, netip.PrefixFrom(parsed, parsed.BitLen()).String())
	}
	if len(address) == 0 {
		return engine.Warp{}, errors.New("the account has no WARP interface address; refresh the peer configuration")
	}
	result := engine.Warp{
		PrivateKey:    profile.PrivateKey,
		Address:       address,
		PeerPublicKey: profile.PeerPublicKey,
		Server:        server,
		Port:          port,
		MTU:           uint32(s.MTU),
		Keepalive:     uint16(s.Keepalive),
	}
	if s.Reserved {
		result.Reserved = profile.Reserved
	}
	return result, nil
}

// EngineOptions converts validated settings into engine options.
func EngineOptions(s Settings, w engine.Warp, cacheFile string) (engine.Options, error) {
	var rules []engine.Rule
	for _, rule := range s.Routing.Rules {
		if !rule.Enabled {
			continue
		}
		action, err := engine.ParseAction(rule.Action)
		if err != nil {
			return engine.Options{}, err
		}
		matcher, err := engine.ParseMatcher(rule.Matcher())
		if err != nil {
			return engine.Options{}, err
		}
		rules = append(rules, engine.Rule{Action: action, Matcher: matcher})
	}
	final, err := engine.ParseAction(s.Routing.Final)
	if err != nil {
		return engine.Options{}, err
	}

	var listeners []engine.Listener
	if s.Mode == ModeProxy || s.Tun.ServeProxy {
		var users []engine.User
		if s.Proxy.AuthEnabled {
			for _, user := range s.Proxy.Users {
				users = append(users, engine.User{Username: user.Username, Password: user.Password})
			}
		}
		systemProxySet := false
		for _, listener := range s.Proxy.Listeners {
			if !listener.Enabled {
				continue
			}
			converted := engine.Listener{
				Type:   listener.Type,
				Listen: listener.Address,
				Port:   uint16(listener.Port),
				Users:  users,
			}
			if s.Mode == ModeProxy && s.Proxy.SystemProxy && !systemProxySet && listener.Type != "socks" {
				converted.SetSystemProxy = true
				systemProxySet = true
			}
			listeners = append(listeners, converted)
		}
	}

	var tun *engine.Tun
	if s.Mode == ModeTun {
		tun = &engine.Tun{
			MTU:                 uint32(s.Tun.MTU),
			Stack:               s.Tun.Stack,
			StrictRoute:         s.Tun.StrictRoute,
			AutoRedirect:        s.Tun.AutoRedirect,
			RouteAddress:        s.Tun.IncludeRoutes,
			RouteExcludeAddress: s.Tun.ExcludeRoutes,
		}
	} else if len(listeners) == 0 {
		return engine.Options{}, errors.New("enable at least one proxy listener")
	}

	options := engine.Options{
		Log:        engine.Log{Level: s.App.LogLevel},
		Warp:       w,
		Listeners:  listeners,
		Tun:        tun,
		Rules:      rules,
		Final:      final,
		BypassLAN:  s.Routing.BypassLAN,
		DNS:        engine.DNS{Remote: s.DNS.Remote, Direct: s.DNS.Direct, Strategy: s.DNS.Strategy},
		CacheFile:  cacheFile,
		GeoIPURL:   s.Routing.GeoIPURL,
		GeoSiteURL: s.Routing.GeoSiteURL,
	}
	if s.App.ClashAPI {
		options.ClashAPI = s.App.ClashAPIAddress
		options.APISecret = s.App.ClashAPISecret
	}
	return options, nil
}
