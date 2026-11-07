package scanner

import (
	"encoding/binary"
	"math/rand/v2"
	"net/netip"
)

// Address ranges WARP's WireGuard servers listen on. Every address is
// anycast, so the data center that answers depends on the network path.
var (
	IPv4Prefixes = mustPrefixes(
		"162.159.192.0/24",
		"162.159.195.0/24",
		"188.114.96.0/24",
		"188.114.97.0/24",
		"188.114.98.0/24",
		"188.114.99.0/24",
	)
	IPv6Prefixes = mustPrefixes(
		"2606:4700:d0::/64",
		"2606:4700:d1::/64",
	)
)

// Ports WARP's WireGuard servers accept.
var Ports = []uint16{
	2408, 500, 1701, 4500, 854, 859, 864, 878, 880, 890, 891, 894, 903, 908,
	928, 934, 939, 942, 943, 945, 946, 955, 968, 987, 988, 1002, 1010, 1014,
	1018, 1070, 1074, 1180, 1387, 1843, 2371, 2506, 3138, 3476, 3581, 3854,
	4177, 4198, 4233, 5279, 5956, 7103, 7152, 7156, 7281, 7559, 8319, 8742,
	8854, 8886,
}

// WellKnown are endpoints worth trying when nothing better is known.
var WellKnown = []netip.AddrPort{
	netip.MustParseAddrPort("162.159.192.1:2408"),
	netip.MustParseAddrPort("162.159.195.1:500"),
	netip.MustParseAddrPort("188.114.96.1:1701"),
	netip.MustParseAddrPort("188.114.97.1:4500"),
	netip.MustParseAddrPort("188.114.98.1:908"),
	netip.MustParseAddrPort("188.114.99.1:854"),
	netip.MustParseAddrPort("162.159.192.10:894"),
	netip.MustParseAddrPort("162.159.195.10:2506"),
}

func mustPrefixes(values ...string) []netip.Prefix {
	result := make([]netip.Prefix, len(values))
	for i, value := range values {
		result[i] = netip.MustParsePrefix(value)
	}
	return result
}

// randomAddress picks a host address inside prefix, avoiding the network
// and broadcast addresses of IPv4 /24s.
func randomAddress(prefix netip.Prefix, random *rand.Rand) netip.Addr {
	base := prefix.Masked().Addr().As16()
	bits := prefix.Bits()
	if prefix.Addr().Is4() {
		bits += 96
	}
	// Randomize the host part, at most the last 64 bits.
	hostBits := 128 - bits
	if hostBits > 64 {
		hostBits = 64
	}
	if hostBits <= 0 {
		return prefix.Addr()
	}
	low := binary.BigEndian.Uint64(base[8:])
	mask := uint64(1)<<hostBits - 1
	if hostBits == 64 {
		mask = ^uint64(0)
	}
	for {
		host := random.Uint64() & mask
		if prefix.Addr().Is4() && hostBits >= 2 && (host == 0 || host == mask) {
			continue
		}
		binary.BigEndian.PutUint64(base[8:], low&^mask|host)
		address := netip.AddrFrom16(base)
		if prefix.Addr().Is4() {
			address = address.Unmap()
		}
		return address
	}
}

// Candidates returns up to count endpoints to probe: extra first, then
// random addresses spread evenly over prefixes and ports.
func Candidates(count int, prefixes []netip.Prefix, ports []uint16, extra []netip.AddrPort) []netip.AddrPort {
	seen := map[netip.AddrPort]bool{}
	result := make([]netip.AddrPort, 0, count)
	add := func(candidate netip.AddrPort) {
		if candidate.IsValid() && !seen[candidate] && len(result) < count {
			seen[candidate] = true
			result = append(result, candidate)
		}
	}
	for _, candidate := range extra {
		add(candidate)
	}
	if len(prefixes) == 0 || len(ports) == 0 {
		return result
	}
	random := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	// Bounded so that small prefix/port spaces cannot loop forever.
	for attempts := 0; len(result) < count && attempts < count*20; attempts++ {
		prefix := prefixes[attempts%len(prefixes)]
		port := ports[random.IntN(len(ports))]
		add(netip.AddrPortFrom(randomAddress(prefix, random), port))
	}
	return result
}
