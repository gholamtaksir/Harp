package config

const (
	DeviceId    = "device_id"
	AccessToken = "access_token"
	PrivateKey  = "private_key"
	LicenseKey  = "license_key"

	// Cached WireGuard peer data. Harp stores it next to the credentials so
	// that the proxy and TUN modes can start without reaching the Cloudflare
	// API, which is frequently unreachable on restricted networks.
	IPv4          = "ipv4"
	IPv6          = "ipv6"
	PeerPublicKey = "peer_public_key"
	Endpoint      = "endpoint"
	EndpointIP    = "endpoint_ip"
	ClientId      = "client_id"
)

type Context struct {
	DeviceId    string
	AccessToken string
	PrivateKey  string
	LicenseKey  string
}
