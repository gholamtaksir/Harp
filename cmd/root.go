package cmd

import (
	"errors"
	"log"
	"os"

	"github.com/gholamtaksir/Harp/cmd/generate"
	"github.com/gholamtaksir/Harp/cmd/proxy"
	"github.com/gholamtaksir/Harp/cmd/register"
	"github.com/gholamtaksir/Harp/cmd/run"
	"github.com/gholamtaksir/Harp/cmd/scan"
	. "github.com/gholamtaksir/Harp/cmd/shared"
	"github.com/gholamtaksir/Harp/cmd/status"
	"github.com/gholamtaksir/Harp/cmd/trace"
	"github.com/gholamtaksir/Harp/cmd/tun"
	"github.com/gholamtaksir/Harp/cmd/update"
	"github.com/gholamtaksir/Harp/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	defaultConfigFile = "harp-account.toml"
	// Accounts created by upstream wgcf are picked up automatically.
	legacyConfigFile = "wgcf-account.toml"
)

var cfgFile string

// Version is set at build time with -ldflags "-X github.com/gholamtaksir/Harp/cmd.Version=...".
var Version = "dev"

var RootCmd = &cobra.Command{
	Use:   "harp",
	Short: "Cloudflare WARP client with proxy, TUN and routing support",
	Long: FormatMessage("", `
Harp is a Cloudflare WARP utility that creates and manages accounts, assigns
license keys, generates WireGuard profiles, and runs WARP as a local
SOCKS5/HTTP proxy or a system-wide TUN tunnel with routing rules.
Project website: https://github.com/gholamtaksir/Harp`),
	Run: func(cmd *cobra.Command, args []string) {
		if err := cmd.Help(); err != nil {
			log.Fatalf("%+v\n", err)
		}
	},
}

func Execute() error {
	return RootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	RootCmd.Version = Version
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", defaultConfigFile, "Account file")
	RootCmd.AddCommand(register.Cmd)
	RootCmd.AddCommand(update.Cmd)
	RootCmd.AddCommand(generate.Cmd)
	RootCmd.AddCommand(status.Cmd)
	RootCmd.AddCommand(trace.Cmd)
	RootCmd.AddCommand(proxy.Cmd)
	RootCmd.AddCommand(tun.Cmd)
	RootCmd.AddCommand(run.Cmd)
	RootCmd.AddCommand(scan.Cmd)
}

var unsupportedConfigError viper.UnsupportedConfigError

func initConfig() {
	initConfigDefaults()
	viper.SetConfigFile(resolveConfigFile())
	viper.SetEnvPrefix("HARP")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); errors.As(err, &unsupportedConfigError) {
		log.Fatal(err)
	} else {
		log.Println("Using account file:", viper.ConfigFileUsed())
	}
}

func resolveConfigFile() string {
	if RootCmd.PersistentFlags().Changed("config") || fileExists(cfgFile) || !fileExists(legacyConfigFile) {
		return cfgFile
	}
	return legacyConfigFile
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func initConfigDefaults() {
	viper.SetDefault(config.DeviceId, "")
	viper.SetDefault(config.AccessToken, "")
	viper.SetDefault(config.PrivateKey, "")
	viper.SetDefault(config.LicenseKey, "")
}
