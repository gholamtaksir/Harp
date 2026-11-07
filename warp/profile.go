// Package warp turns a registered Cloudflare WARP account into the WireGuard
// parameters needed to bring up a tunnel.
package warp

import (
	"encoding/base64"
	"log"
	"net"
	"net/netip"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/gholamtaksir/Harp/cloudflare"
	"github.com/gholamtaksir/Harp/config"
	"github.com/gholamtaksir/Harp/openapi"
	"github.com/spf13/viper"
)

// DefaultPeerPort is the WireGuard port used when Cloudflare does not list one.
const DefaultPeerPort = 2408

// Profile holds everything needed to connect to WARP over WireGuard.
type Profile struct {
	PrivateKey    string
	IPv4          string
	IPv6          string
	PeerPublicKey string
	// Endpoint is the peer address as returned by Cloudflare, usually
	// engage.cloudflareclient.com:2408.
	Endpoint string
	// EndpointIP is a literal IP fallback for Endpoint, used when the
	// hostname cannot be resolved.
	EndpointIP string
	// Reserved holds the three WireGuard reserved bytes Cloudflare uses to
	// identify the client, decoded from client_id.
	Reserved []byte
}

// FromConfig extracts the WireGuard parameters from a device configuration,
// as returned by both the registration and the device endpoints.
func FromConfig(privateKey string, deviceConfig *openapi.Config) (*Profile, error) {
	if deviceConfig == nil || len(deviceConfig.Peers) == 0 {
		return nil, errors.New("Cloudflare response did not contain a WireGuard peer")
	}
	peer := deviceConfig.Peers[0]
	endpoint, err := cloudflare.WireGuardEndpoint(peer.Endpoint)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	reserved, err := DecodeReserved(deviceConfig.ClientId)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return &Profile{
		PrivateKey:    privateKey,
		IPv4:          deviceConfig.Interface.Addresses.V4,
		IPv6:          deviceConfig.Interface.Addresses.V6,
		PeerPublicKey: peer.PublicKey,
		Endpoint:      endpoint,
		EndpointIP:    endpointIP(peer.Endpoint),
		Reserved:      reserved,
	}, nil
}

// endpointIP returns the literal IPv4 (or IPv6) peer address with its port.
func endpointIP(endpoint openapi.Endpoint) string {
	port := DefaultPeerPort
	if len(endpoint.Ports) != 0 {
		port = int(endpoint.Ports[0])
	}
	for _, candidate := range []string{endpoint.V4, endpoint.V6} {
		host := candidate
		if parsedHost, _, err := net.SplitHostPort(candidate); err == nil {
			host = parsedHost
		}
		if addr, err := netip.ParseAddr(host); err == nil && !addr.IsUnspecified() {
			return net.JoinHostPort(addr.String(), strconv.Itoa(port))
		}
	}
	return ""
}

// DecodeReserved converts Cloudflare's base64 client_id into reserved bytes.
func DecodeReserved(clientId string) ([]byte, error) {
	if clientId == "" {
		return nil, nil
	}
	decoded, err := base64.StdEncoding.DecodeString(clientId)
	if err != nil {
		return nil, errors.Wrap(err, "decode client_id")
	}
	if len(decoded) != 3 {
		return nil, errors.Newf("client_id decodes to %d bytes; expected 3", len(decoded))
	}
	return decoded, nil
}

// EncodeReserved is the inverse of DecodeReserved.
func EncodeReserved(reserved []byte) string {
	if len(reserved) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(reserved)
}

// Load returns the WARP profile for the current account. Cached peer data is
// used when available; otherwise, or when refresh is set, it is fetched from
// the Cloudflare API and saved back to the account file.
func Load(ctx *config.Context, refresh bool) (*Profile, error) {
	if !refresh {
		if profile, ok := cached(); ok {
			return profile, nil
		}
		log.Println("Fetching WARP peer configuration from Cloudflare")
	}
	device, err := cloudflare.GetSourceDevice(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	profile, err := FromConfig(viper.GetString(config.PrivateKey), device.Config)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if err := Save(profile); err != nil {
		return nil, errors.WithStack(err)
	}
	return profile, nil
}

func cached() (*Profile, bool) {
	reserved, err := DecodeReserved(viper.GetString(config.ClientId))
	if err != nil {
		return nil, false
	}
	profile := &Profile{
		PrivateKey:    viper.GetString(config.PrivateKey),
		IPv4:          viper.GetString(config.IPv4),
		IPv6:          viper.GetString(config.IPv6),
		PeerPublicKey: viper.GetString(config.PeerPublicKey),
		Endpoint:      viper.GetString(config.Endpoint),
		EndpointIP:    viper.GetString(config.EndpointIP),
		Reserved:      reserved,
	}
	complete := profile.PrivateKey != "" && profile.IPv4 != "" &&
		profile.PeerPublicKey != "" && profile.Endpoint != ""
	return profile, complete
}

// Save writes the peer data of profile into the account file.
func Save(profile *Profile) error {
	viper.Set(config.IPv4, profile.IPv4)
	viper.Set(config.IPv6, profile.IPv6)
	viper.Set(config.PeerPublicKey, profile.PeerPublicKey)
	viper.Set(config.Endpoint, profile.Endpoint)
	viper.Set(config.EndpointIP, profile.EndpointIP)
	viper.Set(config.ClientId, EncodeReserved(profile.Reserved))
	return errors.WithStack(viper.WriteConfig())
}
