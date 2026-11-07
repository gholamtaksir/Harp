//go:build !with_gvisor || !with_wireguard

package main

// Harp embeds sing-box, whose WireGuard and user-space network stack are only
// compiled in with build tags. Build with `make build`, or pass
// -tags with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api
// to go build/install/run.
var _ = harp_must_be_built_with_tags_with_gvisor_and_with_wireguard__see_README
