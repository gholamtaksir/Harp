# Harp
> Harp is an unofficial, cross-platform client for [Cloudflare WARP](https://1.1.1.1/): run it as a local SOCKS5/HTTP proxy or tunnel your whole system, with per-IP, per-domain and per-app routing.

Harp started as a fork of [wgcf](https://github.com/ViRb3/wgcf) and embeds the [sing-box](https://github.com/SagerNet/sing-box) networking core, so no separate WireGuard installation is needed.

## Features
- Register WARP accounts, bind Warp+ license keys, manage devices
- **Proxy mode**: SOCKS5, HTTP or mixed (both on one port) listeners, with username/password authentication, LAN sharing and system proxy support. No administrator rights required
- **TUN mode**: system-wide tunnel like the official WARP client, with DNS leak protection
- **Routing rules** in both modes: send traffic through WARP, bypass it or block it by IP/CIDR, domain, application, port, protocol, GeoIP country or GeoSite category
- Encrypted DNS through WARP, local DNS for bypassed traffic
- **Endpoint scanner** for filtered networks: finds the WARP addresses and ports that answer and carry traffic, and switches automatically when one gets blocked
- Any [sing-box configuration](https://sing-box.sagernet.org/configuration/) can be run with WARP injected, giving access to Shadowsocks, VLESS, Trojan, VMess, Hysteria2, TUIC and more
- Clash compatible API for dashboards
- Generate standard WireGuard profiles
- Works offline from the Cloudflare API once an account is registered
- **Desktop app** for Windows, macOS and Linux, plus a full command line interface

## Desktop app
`harp-gui` puts everything above behind a modern interface with a system tray icon:

- **Dashboard**: one-click connect, proxy/TUN switch, WARP status with your WARP IP, location and data center, live traffic chart
- **Connections**: every active connection with its application, matched rule and route, closable one by one
- **Endpoints**: scan for WARP endpoints that work on your network, with ping, jitter, loss and data center; automatic selection that switches endpoints when traffic stops getting through
- **Routing rules**: add rules by domain, IP range, country, site category, port, protocol or application (picked from running programs), one-click presets, import/export compatible with `harp --rules`
- **Proxy, TUN and DNS** settings, **WARP & account** management (registration, Warp+ license, linked devices, endpoint selection, WireGuard/sing-box export), live **logs**
- Launch at login, start minimized, connect on launch, keep running in the tray, light and dark themes

Settings and the account are stored in the user configuration folder (`%AppData%\Harp`, `~/Library/Application Support/Harp` or `~/.config/Harp`), or in `HARP_DATA_DIR` when set. Existing `harp-account.toml` / `wgcf-account.toml` files can be imported.

TUN mode needs administrator rights: on Windows, Harp offers to restart itself elevated; on macOS and Linux, start it with `sudo` (on Linux, `sudo setcap cap_net_admin,cap_net_bind_service+ep harp-gui` also works).

### Building the desktop app
Requirements: Go 1.26+, Node.js 20+, and the Wails CLI:
```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
```
On Linux, also install the GTK 4 and WebKitGTK 6 development packages (e.g. `libgtk-4-dev libwebkitgtk-6.0-dev` on Debian/Ubuntu 24.04 or newer); on macOS, the Xcode command line tools.

```bash
make gui           # builds gui/bin/harp-gui(.exe)
```
```bash
make gui-package   # installer or package for the current OS
```

The app lives in [gui/](gui/): Go services in `gui/*.go`, the platform-independent logic in [core/](core/), and a React + TypeScript + Tailwind frontend in [gui/frontend/](gui/frontend/).

## Download
Pre-compiled binaries are on the [releases page](https://github.com/gholamtaksir/Harp/releases).

### Building from source
Harp needs Go 1.26 or newer and must be built with sing-box build tags:

```bash
make build
```

Without `make`:

```bash
go build -trimpath -tags with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api -ldflags "-s -w" -o harp .
```

Building without the tags fails with an `undefined: harp_must_be_built_with_tags...` error.

## Quick start
```bash
harp register          # create a WARP account, saved to harp-account.toml
harp proxy             # SOCKS5 + HTTP proxy on 127.0.0.1:1080
```
Or tunnel everything (run as Administrator on Windows, with `sudo` on Linux/macOS):
```bash
harp tun
```
Check that it works with `harp trace` (through the tunnel) or:
```bash
curl -x socks5h://127.0.0.1:1080 https://cloudflare.com/cdn-cgi/trace
```
The output should contain `warp=on` (or `warp=plus`).

Run `harp <command> --help` to see every option.

## Proxy mode
`harp proxy` connects to WARP entirely in user space and serves it through one or more listeners. A listener is written as `[type://][user:pass@]host:port`, where `type` is `mixed` (default; SOCKS4/4a/5 and HTTP on the same port), `socks` or `http`:

```bash
# SOCKS5 for this machine, HTTP for the LAN with authentication
harp proxy -l socks://127.0.0.1:1080 -l http://alice:secret@0.0.0.0:8080

# Same credentials on every listener
harp proxy -l 0.0.0.0:1080 -l 0.0.0.0:1081 --user alice:secret --user bob:hunter2

# Make the first mixed/http listener the system proxy
harp proxy --system-proxy
```

## TUN mode
`harp tun` creates a virtual interface and routes all traffic through it. DNS queries are captured as well, and strict routing prevents leaks outside the tunnel.

```bash
harp tun --exclude-route 192.168.0.0/16       # never route the LAN into the tunnel
harp tun --include-route 104.16.0.0/12        # only route these networks into the tunnel
harp tun -l mixed://127.0.0.1:1080            # also open a proxy listener
```

| Option | Description |
|---|---|
| `--stack` | `mixed` (default), `system` or `gvisor` |
| `--mtu` | TUN MTU, default 9000 |
| `--name` | Interface name, chosen automatically when empty |
| `--no-strict-route` | Allow traffic outside the tunnel (disables leak protection) |
| `--auto-redirect` | Linux only: nftables based redirection for better performance |

## Routing rules
Both modes accept the same rules:

| Option | Effect |
|---|---|
| `--direct <matchers>` | Bypass WARP |
| `--warp <matchers>` | Send through WARP |
| `--block <matchers>` | Drop the connection |
| `--final warp\|direct\|block` | What happens to everything else (default `warp`) |
| `--rules <file>` | Load rules from a file |
| `--no-bypass-lan` | Also send private/LAN addresses through WARP (bypassed by default) |

Each option can be repeated and takes a comma separated list of matchers:

| Matcher | Example | Matches |
|---|---|---|
| IP or CIDR | `1.2.3.4`, `10.0.0.0/8`, `2001:db8::/32` | Destination address |
| `suffix:` (or a bare domain) | `suffix:example.com`, `example.com` | The domain and all its subdomains |
| `domain:` | `domain:chat.openai.com` | Exactly this domain |
| `keyword:` | `keyword:google` | Domains containing the word |
| `regex:` | `regex:^cdn[0-9]+\.` | Domains matching the regular expression |
| `app:` | `app:chrome.exe`, `app:firefox` | Process name |
| `path:` / `path_regex:` | `path:/usr/bin/curl` | Process executable path |
| `port:` | `port:443`, `port:6000-7000` | Destination port or range |
| `protocol:` | `protocol:quic`, `protocol:bittorrent` | Sniffed protocol |
| `network:` | `network:udp` | `tcp` or `udp` |
| `geoip:` | `geoip:ir`, `geoip:cn` | IPs of a country ([list](https://github.com/SagerNet/sing-geoip/tree/rule-set)) |
| `geosite:` | `geosite:category-ads-all` | Domains of a category ([list](https://github.com/SagerNet/sing-geosite/tree/rule-set)) |
| `ruleset:` | `ruleset:https://example.com/list.srs`, `ruleset:./list.json` | A [sing-box rule set](https://sing-box.sagernet.org/configuration/rule-set/) |
| `private` | `private` | Private and LAN addresses |

Examples:
```bash
# Bypass WARP for local sites and a game, block ads
harp proxy --direct geoip:ir,suffix:ir,app:steam.exe --block geosite:category-ads-all

# Whitelist mode: only these go through WARP, everything else is direct
harp tun --final direct --warp app:chrome.exe,suffix:openai.com
```

Rules are evaluated in this order:
1. Private/LAN addresses (unless `--no-bypass-lan`)
2. `block`, then `warp`, then `direct` rules on domains, apps, ports and protocols
3. `block`, then `warp`, then `direct` rules on IPs, `geoip:` and `ruleset:` (domains are resolved first)
4. `--final`

A rules file has one `<action> <matchers>` entry per line:
```
# rules.txt
direct geoip:ir suffix:ir
direct app:Telegram.exe,app:steam.exe
block  geosite:category-ads-all protocol:bittorrent
warp   suffix:openai.com
```
```bash
harp proxy --rules rules.txt
```

GeoIP, GeoSite and remote rule sets are downloaded through WARP when Harp starts and cached in `harp-cache.db`. Mirrors can be set with `--geoip-url` and `--geosite-url` (the `%s` is replaced by the code).

## DNS
Domains sent through WARP are resolved with DNS over HTTPS to `1.1.1.1`, itself sent through WARP. Bypassed domains use the system resolver.

| Option | Default | Description |
|---|---|---|
| `--dns` | `https://1.1.1.1/dns-query` | Resolver used through WARP: `udp://`, `tcp://`, `tls://`, `quic://`, `https://` or `h3://` |
| `--direct-dns` | `local` | Resolver for bypassed traffic: `local` (system) or a server IP/URL |
| `--dns-strategy` | | `prefer_ipv4`, `prefer_ipv6`, `ipv4_only` or `ipv6_only` |

## WARP connection options
| Option | Description |
|---|---|
| `--endpoint` | WARP server to connect to, e.g. `188.114.98.2:1701`, or `auto` to use one that answers (see [Finding a working endpoint](#finding-a-working-endpoint)) |
| `--warp-mtu` | MTU of the WARP tunnel, default 1280 |
| `--keepalive` | WireGuard keepalive in seconds, default 25 |
| `--refresh` | Fetch fresh peer data from the Cloudflare API before connecting |
| `--no-reserved` | Do not send the client id in the WireGuard reserved bytes |

The peer configuration is saved in the account file when registering, so connecting does not require access to the Cloudflare API.

## Finding a working endpoint
WARP's WireGuard servers listen on hundreds of anycast addresses (`162.159.192.0/24`, `162.159.195.0/24`, `188.114.96.0/22`, plus IPv6) and dozens of ports. Filtered networks often block some of them, or let the handshake through but drop the traffic that follows, sometimes only after a few minutes of use. `harp scan` finds the ones that work:

```bash
harp scan                  # 600 random endpoints, then checks traffic through the best
harp scan -n 3000 --ipv6   # a larger scan, including IPv6
```
```
ENDPOINT              PING    JITTER  LOSS  TRAFFIC      DATA CENTER
188.114.97.27:3854    116 ms  0 ms    0%    ok, 650 ms   FRA Frankfurt, DE
188.114.99.244:864    116 ms  1 ms    0%    ok, 679 ms   FRA Frankfurt, DE
162.159.192.131:1180  117 ms  0 ms    0%    not checked
```

The scan sends a real WireGuard handshake initiation from your account to each endpoint, so only genuine WARP servers answer, then starts a tunnel through the fastest ones and fetches Cloudflare's trace to confirm that traffic gets through and to find the data center.

`--endpoint auto` (CLI) or the **Automatic** endpoint mode (desktop app) picks an endpoint each time Harp connects: the desktop app prefers the endpoint that last carried traffic, then the best ones from earlier scans, the account's own endpoint and a few well-known addresses, and scans when none of them answers. While connected, it checks the tunnel every 30 seconds and moves to another endpoint when traffic stops getting through.

When the endpoint's host name resolves to a private or reserved address, as filtering DNS servers do, Harp falls back to the address Cloudflare listed for the account.

## Advanced: sing-box configurations
`harp run` runs any sing-box configuration and adds a WireGuard endpoint for your WARP account, tagged `warp`. Reference it as an outbound in your rules; it also becomes the default outbound when `route.final` is empty.

```bash
harp proxy --direct geoip:ir --dump > config.json   # start from a generated configuration
harp run -c config.json
harp run -c base.json -c overrides.json             # later files are merged over earlier ones
```

The generated configuration contains your private key, keep it private.

### Monitoring
`--api 127.0.0.1:9090` (with an optional `--api-secret`) enables the Clash compatible API, usable with dashboards such as [yacd](https://github.com/haishanh/yacd) or [metacubexd](https://github.com/MetaCubeX/metacubexd).

## Account management
### Register a new account
```bash
harp register
```
The account is saved to `harp-account.toml`. Use `--config` to choose another file. Accounts created by wgcf (`wgcf-account.toml`) are picked up automatically.

### Add a license key
If you have an existing Warp+ subscription, for example on your phone, you can bind the account generated by this tool to your phone's account, sharing its Warp+ status. Please note that there is a limit of 5 maximum devices linked at a time. You can remove linked devices from the 1.1.1.1 app on your phone.

> [!CAUTION]
> Only subscriptions purchased directly from the official 1.1.1.1 app are supported. Keys obtained by any other means, including referrals, will not work and will not be supported.

First, get your Warp+ account license key. To view it on Android:
1. Open the `1.1.1.1` app
2. Click on the hamburger menu button in the top-right corner
3. Navigate to: `Account` > `Key`

> [!WARNING]
> There's a bug on Cloudflare's side that prevents existing accounts from getting Warp+ even after you bind a correct license key. If you have ever connected to Warp VPN, then your account is affected. You will need to make a new account with `harp register`, then immediately proceed with the steps below, without running any other commands. For more details, check [wgcf#85](https://github.com/ViRb3/wgcf/issues/85).

```bash
harp update --license-key "YOUR_LICENSE_KEY_GOES_HERE"
```

`harp update` can also rename the device (`--name`) and remove, activate or deactivate linked devices (`--remove`, `--activate`, `--deactivate`).

### Check device status
```bash
harp status
```

### Generate a WireGuard profile
To use WARP with another WireGuard client:
```bash
harp generate
```
The profile is saved to `harp-profile.conf`. See the [WireGuard Quick Start](https://www.wireguard.com/quickstart/) for how to use it. `--keepalive` adds `PersistentKeepalive` to the profile. The MTU is 1280, like the official Android app; you may get better speeds by raising it (see [wgcf#40](https://github.com/ViRb3/wgcf/issues/40)).

### Verify WARP works
While connected, run:
```bash
harp trace
```
The last line should say `warp=on` or `warp=plus`.

## Development
```bash
make test   # run the tests with the required build tags
make vet
```

The API client code is auto-generated from the OpenAPI spec [openapi-spec.yml](openapi-spec.yml) and stored under the [openapi/](openapi/) package. Do not touch any code under that package, instead, change the spec file and regenerate the API client code. To do this, [install openapi-generator](https://openapi-generator.tech/docs/installation), then run:

```bash
go generate
```

## License
Harp is licensed under the [GNU General Public License v3.0](LICENSE) or later, because it includes sing-box, which is licensed under GPLv3 or later.

Harp is based on [wgcf](https://github.com/ViRb3/wgcf) by ViRb3, licensed under the [MIT License](LICENSE-wgcf).

## Notice of Non-Affiliation and Disclaimer
We are not affiliated, associated, authorized, endorsed by, or in any way officially connected with Cloudflare, or any of its subsidiaries or its affiliates, nor with the wgcf or sing-box projects. The official Cloudflare website can be found at https://www.cloudflare.com/.

The names Cloudflare Warp and Cloudflare as well as related names, marks, emblems and images are registered trademarks of their respective owners.
