package warp

import (
	"bytes"
	"testing"

	"github.com/gholamtaksir/Harp/openapi"
)

func TestReservedRoundTrip(t *testing.T) {
	reserved, err := DecodeReserved("AQID")
	if err != nil || !bytes.Equal(reserved, []byte{1, 2, 3}) {
		t.Fatalf("DecodeReserved = %v, %v", reserved, err)
	}
	if encoded := EncodeReserved(reserved); encoded != "AQID" {
		t.Errorf("EncodeReserved = %q", encoded)
	}
	if reserved, err := DecodeReserved(""); err != nil || reserved != nil {
		t.Errorf("empty client_id = %v, %v", reserved, err)
	}
	for _, bad := range []string{"!!", "AQIDBA=="} {
		if _, err := DecodeReserved(bad); err == nil {
			t.Errorf("DecodeReserved(%q) succeeded", bad)
		}
	}
}

func TestFromConfig(t *testing.T) {
	host := "engage.cloudflareclient.com:2408"
	config := &openapi.Config{
		ClientId: "AQID",
		Interface: openapi.ConfigInterface{Addresses: openapi.NetworkAddress{
			V4: "172.16.0.2",
			V6: "2606:4700:110:8a36:df92:102a:9602:fa18",
		}},
		Peers: []openapi.Peer{{
			PublicKey: "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=",
			Endpoint: openapi.Endpoint{
				Host:  &host,
				V4:    "162.159.192.7:0",
				V6:    "[2606:4700:d0::a29f:c007]:0",
				Ports: []int32{2408, 500},
			},
		}},
	}
	profile, err := FromConfig("key", config)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Endpoint != host || profile.EndpointIP != "162.159.192.7:2408" ||
		!bytes.Equal(profile.Reserved, []byte{1, 2, 3}) || profile.IPv4 != "172.16.0.2" {
		t.Errorf("unexpected profile: %+v", profile)
	}

	config.Peers[0].Endpoint.V4 = "0.0.0.0:0"
	if profile, _ := FromConfig("key", config); profile.EndpointIP != "[2606:4700:d0::a29f:c007]:2408" {
		t.Errorf("IPv6 fallback = %q", profile.EndpointIP)
	}
	if _, err := FromConfig("key", &openapi.Config{}); err == nil {
		t.Error("config without peers accepted")
	}
}
