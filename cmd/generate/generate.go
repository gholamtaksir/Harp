package generate

import (
	"log"

	"github.com/cockroachdb/errors"
	. "github.com/gholamtaksir/Harp/cmd/shared"
	"github.com/gholamtaksir/Harp/warp"
	"github.com/gholamtaksir/Harp/wireguard"
	"github.com/spf13/cobra"
)

var profileFile string
var keepalive int
var shortMsg = "Generates a WireGuard profile from the current Cloudflare Warp account"

const maxKeepalive = 65535

var Cmd = &cobra.Command{
	Use:   "generate",
	Short: shortMsg,
	Long:  FormatMessage(shortMsg, ``),
	Run: func(cmd *cobra.Command, args []string) {
		RunCommandFatal(generateProfile)
	},
}

func init() {
	Cmd.PersistentFlags().StringVarP(&profileFile, "profile", "p", "harp-profile.conf", "WireGuard profile file")
	Cmd.PersistentFlags().IntVarP(&keepalive, "keepalive", "k", 0, "Persistent keepalive interval in seconds (default 25)")
	Cmd.PersistentFlags().Lookup("keepalive").NoOptDefVal = "25"
}

func generateProfile() error {
	if err := validateKeepalive(keepalive); err != nil {
		return err
	}

	if err := EnsureConfigValidAccount(); err != nil {
		return errors.WithStack(err)
	}

	// Always fetch fresh peer data; this also refreshes the cache used by
	// the proxy and TUN modes.
	warpProfile, err := warp.Load(CreateContext(), true)
	if err != nil {
		return errors.WithStack(err)
	}

	profile, err := wireguard.NewProfile(&wireguard.ProfileData{
		PrivateKey: warpProfile.PrivateKey,
		Address1:   warpProfile.IPv4,
		Address2:   warpProfile.IPv6,
		PublicKey:  warpProfile.PeerPublicKey,
		Endpoint:   warpProfile.Endpoint,
		Keepalive:  keepalive,
	})
	if err != nil {
		return errors.WithStack(err)
	}
	if err := profile.Save(profileFile); err != nil {
		return errors.WithStack(err)
	}

	log.Println("Successfully generated WireGuard profile:", profileFile)
	return nil
}

func validateKeepalive(interval int) error {
	if interval < 0 || interval > maxKeepalive {
		return errors.Errorf("persistent keepalive interval must be between 0 and %d seconds, got %d", maxKeepalive, interval)
	}
	return nil
}
