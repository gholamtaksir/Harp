package status

import (
	"github.com/cockroachdb/errors"
	"github.com/gholamtaksir/Harp/cloudflare"
	. "github.com/gholamtaksir/Harp/cmd/shared"
	"github.com/spf13/cobra"
)

var shortMsg = "Prints the status of the current Cloudflare Warp device"

var Cmd = &cobra.Command{
	Use:   "status",
	Short: shortMsg,
	Long:  FormatMessage(shortMsg, ``),
	Run: func(cmd *cobra.Command, args []string) {
		RunCommandFatal(status)
	},
}

func init() {
}

func status() error {
	if err := EnsureConfigValidAccount(); err != nil {
		return errors.WithStack(err)
	}

	ctx := CreateContext()

	account, err := cloudflare.GetAccount(ctx)
	if err != nil {
		return errors.WithStack(err)
	}
	boundDevices, err := cloudflare.GetBoundDevices(ctx)
	if err != nil {
		return errors.WithStack(err)
	}

	PrintAccountDetails(account, boundDevices)
	return nil
}
