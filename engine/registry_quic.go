//go:build with_quic

package engine

// Registers the QUIC transport for V2Ray based protocols (VMess/VLESS/Trojan).
import _ "github.com/sagernet/sing-box/transport/v2rayquic"
