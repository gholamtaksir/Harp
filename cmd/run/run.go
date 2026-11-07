package run

import (
	"os"

	"github.com/cockroachdb/errors"
	. "github.com/gholamtaksir/Harp/cmd/shared"
	"github.com/gholamtaksir/Harp/engine"
	"github.com/spf13/cobra"
)

var configFiles []string
var warpFlags WarpFlags
var noInject bool
var dump bool

var shortMsg = "Runs a sing-box configuration with WARP injected"

var Cmd = &cobra.Command{
	Use:   "run",
	Short: shortMsg,
	Long: FormatMessage(shortMsg, `
Runs any sing-box configuration, giving access to every inbound protocol
(Shadowsocks, VLESS, Trojan, Hysteria2, ...), outbound and routing feature.
A WireGuard endpoint for the current WARP account is added with the tag given
by --warp-tag; reference it as an outbound in your rules. It also becomes the
final outbound when route.final is empty. A good starting point is the output
of "harp proxy --dump" or "harp tun --dump".`),
	Example: `  harp run -c config.json
  harp run -c base.json -c rules.json --endpoint 162.159.192.1:2408`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		RunCommandFatal(run)
	},
}

func init() {
	Cmd.Flags().StringArrayVarP(&configFiles, "config-file", "c", nil, "sing-box configuration file; later files are merged over earlier ones (repeatable)")
	Cmd.Flags().StringVar(&warpFlags.Tag, "warp-tag", engine.DefaultWarpTag, "Tag of the injected WARP endpoint")
	Cmd.Flags().BoolVar(&noInject, "no-inject", false, "Run the configuration as is, without adding the WARP endpoint")
	Cmd.Flags().BoolVar(&dump, "dump", false, "Print the final configuration and exit (contains your private key)")
	warpFlags.Register(Cmd)
	_ = Cmd.MarkFlagRequired("config-file")
}

func run() error {
	var contents [][]byte
	for _, path := range configFiles {
		content, err := os.ReadFile(path)
		if err != nil {
			return NewUserError(err.Error())
		}
		contents = append(contents, content)
	}
	ctx := engine.NewContext()
	options, err := engine.Merge(ctx, contents)
	if err != nil {
		return NewUserError(err.Error())
	}
	if !noInject {
		warpEndpoint, err := warpFlags.Build()
		if err != nil {
			return errors.WithStack(err)
		}
		if err := engine.InjectWarp(ctx, &options, warpEndpoint); err != nil {
			return errors.WithStack(err)
		}
	}
	return RunOptions(ctx, options, dump)
}
