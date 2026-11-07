package proxy

import (
	"github.com/cockroachdb/errors"
	. "github.com/gholamtaksir/Harp/cmd/shared"
	"github.com/spf13/cobra"
)

var flags EngineFlags

var shortMsg = "Runs WARP as a local SOCKS5/HTTP proxy"

var Cmd = &cobra.Command{
	Use:   "proxy",
	Short: shortMsg,
	Long: FormatMessage(shortMsg, `
Connects to WARP in user space (no administrator rights needed) and serves it
through one or more proxy listeners. Rules decide which traffic goes through
WARP, which bypasses it, and which is blocked.`),
	Example: `  harp proxy
  harp proxy -l socks://127.0.0.1:1080 -l http://0.0.0.0:8080 -u alice:secret
  harp proxy --direct geoip:ir,geosite:category-ir --block geosite:category-ads-all
  harp proxy --final direct --warp suffix:openai.com,suffix:google.com
  harp proxy --direct app:steam.exe,10.0.0.0/8 --system-proxy`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		RunCommandFatal(run)
	},
}

func init() {
	flags.Register(Cmd, []string{"mixed://127.0.0.1:1080"})
}

func run() error {
	options, err := flags.Options(nil)
	if err != nil {
		return errors.WithStack(err)
	}
	return RunEngine(options, flags.Dump)
}
