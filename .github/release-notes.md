First release of Harp: a Cloudflare WARP client that runs as a local SOCKS5/HTTP proxy or tunnels the whole system, with routing rules by IP, domain, country, site category and application.

### Desktop app
| File | System |
|---|---|
| `Harp-*-windows-amd64.zip` | Windows 10/11 (64-bit). Unzip and run `harp-gui.exe`. TUN mode asks for administrator rights. |
| `Harp-*-macos-universal.zip` | macOS 12 or newer, Apple silicon and Intel. The app is not notarized: after unzipping, run `xattr -dr com.apple.quarantine Harp.app` once, or right-click the app and choose Open. |
| `Harp-*-linux-amd64.deb` / `.tar.gz` | Ubuntu 24.04, Debian 13 or newer (needs GTK 4 and WebKitGTK 6). |

### Command line
`harp_<version>_<os>_<arch>` binaries for Linux, Windows, macOS and FreeBSD, including ARM, MIPS and RISC-V routers. Start with `harp register`, then `harp proxy` or `harp tun`. `harp scan` finds WARP endpoints that work on your network.

### Highlights
- Proxy mode (SOCKS5, HTTP or mixed listeners with username/password) and TUN mode
- Routing rules for IPs, domains, countries, site categories, ports, protocols and apps
- Endpoint scanner with real WireGuard handshakes and traffic checks; automatic endpoint selection that switches endpoints when one gets blocked
- WARP+ license keys, linked devices, WireGuard and sing-box configuration export

`SHA256SUMS.txt` lists the checksums of all files.
