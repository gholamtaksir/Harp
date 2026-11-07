package tun

import (
	"net/netip"

	"github.com/cockroachdb/errors"
	. "github.com/gholamtaksir/Harp/cmd/shared"
	"github.com/gholamtaksir/Harp/engine"
	"github.com/spf13/cobra"
)

var flags EngineFlags
var tunOptions engine.Tun
var noStrictRoute bool

var shortMsg = "Tunnels the whole system through WARP"

var Cmd = &cobra.Command{
	Use:   "tun",
	Short: shortMsg,
	Long: FormatMessage(shortMsg, `
Creates a virtual network interface and routes all system traffic through it,
like the official WARP client. Rules decide which traffic goes through WARP,
which bypasses it, and which is blocked. Requires administrator/root rights.`),
	Example: `  harp tun
  harp tun --direct geoip:ir,private --direct app:Telegram.exe
  harp tun --final direct --warp app:chrome.exe,app:firefox.exe
  harp tun --exclude-route 192.168.0.0/16 -l mixed://127.0.0.1:1080`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		RunCommandFatal(run)
	},
}

func init() {
	flags.Register(Cmd, nil)
	f := Cmd.Flags()
	f.StringVar(&tunOptions.InterfaceName, "name", "", "Interface name (chosen automatically when empty)")
	f.Uint32Var(&tunOptions.MTU, "mtu", 9000, "MTU of the TUN interface")
	f.StringVar(&tunOptions.Stack, "stack", "mixed", "Network stack: system, gvisor or mixed")
	f.StringArrayVar(&tunOptions.Address, "address", nil, "TUN interface address (repeatable, default 172.19.0.1/30 and fdfe:dcba:9876::1/126)")
	f.BoolVar(&noStrictRoute, "no-strict-route", false, "Allow traffic to leave outside the tunnel (disables DNS leak protection)")
	f.BoolVar(&tunOptions.AutoRedirect, "auto-redirect", false, "Linux only: use nftables redirection for better performance")
	f.StringArrayVar(&tunOptions.RouteAddress, "include-route", nil, "Only route these CIDRs into the tunnel (repeatable)")
	f.StringArrayVar(&tunOptions.RouteExcludeAddress, "exclude-route", nil, "Never route these CIDRs into the tunnel (repeatable)")
}

func run() error {
	for _, values := range [][]string{tunOptions.Address, tunOptions.RouteAddress, tunOptions.RouteExcludeAddress} {
		for _, value := range values {
			if _, err := netip.ParsePrefix(value); err != nil {
				return NewUserError("invalid CIDR " + value)
			}
		}
	}
	switch tunOptions.Stack {
	case "system", "gvisor", "mixed":
	default:
		return NewUserError("--stack must be system, gvisor or mixed")
	}
	tunOptions.StrictRoute = !noStrictRoute

	options, err := flags.Options(&tunOptions)
	if err != nil {
		return errors.WithStack(err)
	}
	if !flags.Dump {
		WarnIfNotElevated()
	}
	return RunEngine(options, flags.Dump)
}
