package core

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gholamtaksir/Harp/cloudflare"
	"github.com/gholamtaksir/Harp/config"
	"github.com/gholamtaksir/Harp/util"
	"github.com/gholamtaksir/Harp/warp"
	"github.com/gholamtaksir/Harp/wireguard"
	"github.com/spf13/viper"
)

// ErrNoAccount is returned when no WARP account is registered.
var ErrNoAccount = errors.New("no WARP account registered")

// Device is a device bound to the WARP account.
type Device struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Model     string `json:"model"`
	Type      string `json:"type"`
	Active    bool   `json:"active"`
	Current   bool   `json:"current"`
	Created   string `json:"created"`
	Activated string `json:"activated"`
}

// AccountInfo describes the WARP account.
type AccountInfo struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Role        string   `json:"role"`
	WarpPlus    bool     `json:"warpPlus"`
	PremiumData int64    `json:"premiumData"`
	Quota       int64    `json:"quota"`
	License     string   `json:"license"`
	DeviceID    string   `json:"deviceId"`
	DeviceName  string   `json:"deviceName"`
	Devices     []Device `json:"devices"`
}

// AccountManager manages the WARP account file. The account code shared
// with the CLI stores its state in viper's global instance, so all access is
// serialized here.
type AccountManager struct {
	mu   sync.Mutex
	path string
}

// NewAccountManager loads the account stored at path, if any.
func NewAccountManager(path string) (*AccountManager, error) {
	manager := &AccountManager{path: path}
	if err := manager.reload(); err != nil {
		return nil, err
	}
	return manager, nil
}

// Path is the location of the account file.
func (a *AccountManager) Path() string {
	return a.path
}

// Reload reads the account file again, e.g. after it was replaced.
func (a *AccountManager) Reload() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.reload()
}

func (a *AccountManager) reload() error {
	viper.Reset()
	viper.SetConfigFile(a.path)
	viper.SetConfigType("toml")
	for _, key := range []string{config.DeviceId, config.AccessToken, config.PrivateKey, config.LicenseKey} {
		viper.SetDefault(key, "")
	}
	if err := viper.ReadInConfig(); err != nil && !errors.Is(err, os.ErrNotExist) {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			if _, statErr := os.Stat(a.path); statErr == nil {
				return errors.Wrap(err, "read account file")
			}
		}
	}
	return nil
}

func (a *AccountManager) exists() bool {
	return viper.GetString(config.DeviceId) != "" &&
		viper.GetString(config.AccessToken) != "" &&
		viper.GetString(config.PrivateKey) != ""
}

// Exists reports whether an account is registered.
func (a *AccountManager) Exists() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.exists()
}

func (a *AccountManager) context() *config.Context {
	return &config.Context{
		DeviceId:    viper.GetString(config.DeviceId),
		AccessToken: viper.GetString(config.AccessToken),
		LicenseKey:  viper.GetString(config.LicenseKey),
	}
}

// Register creates a new WARP account. The caller must have obtained the
// user's acceptance of Cloudflare's terms of service.
func (a *AccountManager) Register(deviceName, deviceModel string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.exists() {
		return errors.New("an account is already registered; remove it first")
	}
	if deviceModel == "" {
		deviceModel = "PC"
	}
	privateKey, err := wireguard.NewPrivateKey()
	if err != nil {
		return errors.WithStack(err)
	}
	device, err := cloudflare.Register(privateKey.Public(), deviceModel)
	if err != nil {
		return errors.Wrap(err, "register with Cloudflare")
	}
	if err := os.MkdirAll(filepath.Dir(a.path), 0o700); err != nil {
		return errors.WithStack(err)
	}
	viper.Set(config.PrivateKey, privateKey.String())
	viper.Set(config.DeviceId, device.Id)
	viper.Set(config.AccessToken, device.Token)
	viper.Set(config.LicenseKey, device.Account.GetLicense())
	if err := viper.WriteConfig(); err != nil {
		return errors.WithStack(err)
	}
	if profile, err := warp.FromConfig(privateKey.String(), device.Config); err == nil {
		if err := warp.Save(profile); err != nil {
			return errors.WithStack(err)
		}
	}
	if deviceName == "" {
		deviceName = util.RandomHexString(3)
	}
	return a.renameDevice(deviceName)
}

// Remove deletes the local account file, keeping a backup next to it.
func (a *AccountManager) Remove() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, err := os.Stat(a.path); err == nil {
		if err := os.Rename(a.path, a.path+".bak"); err != nil {
			return errors.WithStack(err)
		}
	}
	return a.reload()
}

// Info fetches the account details from Cloudflare.
func (a *AccountManager) Info() (*AccountInfo, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.exists() {
		return nil, ErrNoAccount
	}
	ctx := a.context()
	account, err := cloudflare.GetAccount(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "fetch account")
	}
	boundDevices, err := cloudflare.GetBoundDevices(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "fetch devices")
	}
	info := &AccountInfo{
		ID:       account.Id,
		Type:     account.AccountType,
		WarpPlus: account.WarpPlus != nil && *account.WarpPlus,
		DeviceID: ctx.DeviceId,
		License:  ctx.LicenseKey,
		Devices:  []Device{},
	}
	if account.Role != nil {
		info.Role = *account.Role
	}
	if account.PremiumData != nil {
		info.PremiumData = int64(*account.PremiumData)
	}
	if account.Quota != nil {
		info.Quota = int64(*account.Quota)
	}
	if account.License != nil {
		info.License = *account.License
	}
	for _, device := range boundDevices {
		converted := Device{
			ID:        device.Id,
			Type:      device.Type,
			Active:    device.Active,
			Current:   device.Id == ctx.DeviceId,
			Created:   device.Created.Format(time.RFC3339),
			Activated: device.Activated.Format(time.RFC3339),
		}
		if device.Name != nil {
			converted.Name = *device.Name
		}
		if device.Model != nil {
			converted.Model = *device.Model
		}
		if converted.Current {
			info.DeviceName = converted.Name
		}
		info.Devices = append(info.Devices, converted)
	}
	return info, nil
}

// UpdateLicense binds the device to the account owning licenseKey.
func (a *AccountManager) UpdateLicense(licenseKey string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.exists() {
		return ErrNoAccount
	}
	if wireguard.IsKey(licenseKey) {
		return errors.New("this looks like a WireGuard private key, not a WARP license key")
	}
	ctx := a.context()
	ctx.LicenseKey = licenseKey
	if err := cloudflare.UpdateLicenseKey(ctx); err != nil {
		return errors.Wrap(err, "update license key")
	}
	viper.Set(config.LicenseKey, licenseKey)
	return errors.WithStack(viper.WriteConfig())
}

// RenameDevice changes the name of this device.
func (a *AccountManager) RenameDevice(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.exists() {
		return ErrNoAccount
	}
	return a.renameDevice(name)
}

func (a *AccountManager) renameDevice(name string) error {
	ctx := a.context()
	if _, err := cloudflare.UpdateSourceBoundDeviceName(ctx, ctx.DeviceId, name); err != nil && !util.IsHttp500Error(err) {
		return errors.Wrap(err, "rename device")
	}
	return nil
}

// SetDeviceActive activates or deactivates a bound device.
func (a *AccountManager) SetDeviceActive(deviceID string, active bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.exists() {
		return ErrNoAccount
	}
	_, err := cloudflare.UpdateSourceBoundDeviceActive(a.context(), deviceID, active)
	return errors.Wrap(err, "update device")
}

// RemoveDevice unbinds a device from the account.
func (a *AccountManager) RemoveDevice(deviceID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.exists() {
		return ErrNoAccount
	}
	return errors.Wrap(cloudflare.DeleteBoundDevice(a.context(), deviceID), "remove device")
}

// Profile returns the WireGuard parameters, fetching them from Cloudflare
// when they are not cached or refresh is set.
func (a *AccountManager) Profile(refresh bool) (*warp.Profile, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.exists() {
		return nil, ErrNoAccount
	}
	profile, err := warp.Load(a.context(), refresh)
	return profile, errors.WithStack(err)
}

// WireGuardConfig renders a standard WireGuard configuration file.
func (a *AccountManager) WireGuardConfig(keepalive int) (string, error) {
	profile, err := a.Profile(false)
	if err != nil {
		return "", err
	}
	rendered, err := wireguard.NewProfile(&wireguard.ProfileData{
		PrivateKey: profile.PrivateKey,
		Address1:   profile.IPv4,
		Address2:   profile.IPv6,
		PublicKey:  profile.PeerPublicKey,
		Endpoint:   profile.Endpoint,
		Keepalive:  keepalive,
	})
	if err != nil {
		return "", errors.WithStack(err)
	}
	return rendered.String(), nil
}
