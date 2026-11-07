package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gholamtaksir/Harp/core"
	"github.com/gholamtaksir/Harp/privilege"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// AppInfo describes the application and its environment.
type AppInfo struct {
	Version        string `json:"version"`
	SingBoxVersion string `json:"singBoxVersion"`
	GoVersion      string `json:"goVersion"`
	OS             string `json:"os"`
	Arch           string `json:"arch"`
	Elevated       bool   `json:"elevated"`
	CanRelaunch    bool   `json:"canRelaunch"`
	ElevationHint  string `json:"elevationHint"`
	DataDir        string `json:"dataDir"`
	AccountPath    string `json:"accountPath"`
}

// SaveResult reports why settings were rejected, if they were.
type SaveResult struct {
	OK     bool              `json:"ok"`
	Fields []core.FieldError `json:"fields"`
}

// HarpService is the API exposed to the frontend.
type HarpService struct {
	controller *core.Controller
	dataDir    string
	app        *application.App
	window     *application.WebviewWindow
	quitting   atomic.Bool
}

func newHarpService(controller *core.Controller, dataDir string) *HarpService {
	return &HarpService{controller: controller, dataDir: dataDir}
}

func (s *HarpService) attach(app *application.App, window *application.WebviewWindow) {
	s.app = app
	s.window = window
}

func (s *HarpService) quit() {
	s.quitting.Store(true)
	s.app.Quit()
}

// GetAppInfo returns version and environment information.
func (s *HarpService) GetAppInfo() AppInfo {
	info := AppInfo{
		Version:        Version,
		SingBoxVersion: "unknown",
		GoVersion:      runtime.Version(),
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
		Elevated:       privilege.IsElevated(),
		CanRelaunch:    privilege.CanRelaunch,
		ElevationHint:  privilege.Hint,
		DataDir:        s.dataDir,
		AccountPath:    s.controller.Accounts().Path(),
	}
	if build, ok := debug.ReadBuildInfo(); ok {
		for _, dependency := range build.Deps {
			if dependency.Path == "github.com/sagernet/sing-box" {
				info.SingBoxVersion = strings.TrimPrefix(dependency.Version, "v")
			}
		}
	}
	return info
}

// ---- Connection ----

// GetState returns the connection state.
func (s *HarpService) GetState() core.State {
	return s.controller.State()
}

// Connect starts the connection.
func (s *HarpService) Connect() error {
	return s.controller.Connect()
}

// Disconnect stops the connection.
func (s *HarpService) Disconnect() error {
	return s.controller.Disconnect()
}

// SetMode switches between "proxy" and "tun", reconnecting if needed.
func (s *HarpService) SetMode(mode string) error {
	return s.controller.SetMode(mode)
}

// CheckConnection verifies that traffic flows through WARP.
func (s *HarpService) CheckConnection() (core.Trace, error) {
	return s.controller.CheckConnection()
}

// GetStatsHistory returns the recent per-second traffic.
func (s *HarpService) GetStatsHistory() []core.Stats {
	return s.controller.StatsHistory()
}

// GetConnections lists active connections.
func (s *HarpService) GetConnections() []core.Connection {
	return s.controller.Connections()
}

// CloseConnection closes one connection.
func (s *HarpService) CloseConnection(id string) error {
	return s.controller.CloseConnection(id)
}

// CloseAllConnections closes every connection.
func (s *HarpService) CloseAllConnections() {
	s.controller.CloseAllConnections()
}

// RelaunchAsAdmin restarts Harp with administrator rights (Windows only).
func (s *HarpService) RelaunchAsAdmin(connect bool) error {
	args := []string{"--wait-pid", strconv.Itoa(os.Getpid())}
	if connect {
		args = append(args, "--connect")
	}
	if err := privilege.RelaunchElevated(args); err != nil {
		return err
	}
	go func() {
		time.Sleep(300 * time.Millisecond)
		s.quit()
	}()
	return nil
}

// ---- Endpoints ----

// GetScan returns the current or last endpoint scan.
func (s *HarpService) GetScan() core.ScanState {
	return s.controller.ScanState()
}

// StartScan looks for WARP endpoints that work on this network. A running
// connection is stopped first.
func (s *HarpService) StartScan(request core.ScanRequest) error {
	return s.controller.StartScan(request)
}

// StopScan cancels a running scan, keeping what was found.
func (s *HarpService) StopScan() {
	s.controller.StopScan()
}

// TestEndpoint measures one endpoint and checks that traffic gets through.
func (s *HarpService) TestEndpoint(endpoint string) (core.EndpointResult, error) {
	return s.controller.TestEndpoint(endpoint)
}

// NextEndpoint moves the connection to the next endpoint and returns it.
func (s *HarpService) NextEndpoint() (string, error) {
	return s.controller.NextEndpoint()
}

// ClearEndpoints forgets all scan results.
func (s *HarpService) ClearEndpoints() error {
	return s.controller.ClearEndpoints()
}

// UseEndpoint switches to a fixed endpoint, or back to automatic selection
// when endpoint is empty. A running connection is restarted.
func (s *HarpService) UseEndpoint(endpoint string) (SaveResult, error) {
	settings := s.controller.Settings()
	settings.Warp.Endpoint = strings.TrimSpace(endpoint)
	return s.SaveSettings(settings)
}

// ---- Settings ----

// GetSettings returns the current settings.
func (s *HarpService) GetSettings() core.Settings {
	return s.controller.Settings()
}

// GetDefaultSettings returns the settings of a fresh installation.
func (s *HarpService) GetDefaultSettings() core.Settings {
	return core.DefaultSettings()
}

// SaveSettings validates and stores the settings, applying them to a running
// connection.
func (s *HarpService) SaveSettings(settings core.Settings) (SaveResult, error) {
	err := s.controller.SaveSettings(settings)
	var validation *core.ValidationError
	if errors.As(err, &validation) {
		return SaveResult{Fields: validation.Fields}, nil
	}
	if err != nil {
		return SaveResult{Fields: []core.FieldError{}}, err
	}
	return SaveResult{OK: true, Fields: []core.FieldError{}}, nil
}

// GetAutostart reports whether Harp starts at login.
func (s *HarpService) GetAutostart() (bool, error) {
	status, err := s.app.Autostart.Status()
	if err != nil {
		return false, err
	}
	return status.Enabled, nil
}

// SetAutostart enables or disables starting at login.
func (s *HarpService) SetAutostart(enabled bool) error {
	if enabled {
		return s.app.Autostart.Enable()
	}
	return s.app.Autostart.Disable()
}

// ---- Logs ----

// GetLogs returns the buffered log entries.
func (s *HarpService) GetLogs() []core.LogEntry {
	return s.controller.Logs()
}

// ClearLogs empties the log buffer.
func (s *HarpService) ClearLogs() {
	s.controller.ClearLogs()
}

// ExportLogs saves the logs to a file chosen by the user. It returns the
// path, or an empty string when cancelled.
func (s *HarpService) ExportLogs() (string, error) {
	var builder strings.Builder
	for _, entry := range s.controller.Logs() {
		builder.WriteString(time.UnixMilli(entry.Time).Format("2006-01-02 15:04:05.000"))
		builder.WriteString(" [" + strings.ToUpper(entry.Level) + "] ")
		builder.WriteString(entry.Message)
		builder.WriteByte('\n')
	}
	return s.saveFile("Export logs", "harp-"+time.Now().Format("20060102-150405")+".log",
		"Log files", "*.log;*.txt", builder.String())
}

// ---- Account ----

// GetAccount fetches the account details from Cloudflare.
func (s *HarpService) GetAccount() (*core.AccountInfo, error) {
	return s.controller.Accounts().Info()
}

// RegisterAccount creates a new WARP account. The frontend must have shown
// Cloudflare's terms of service and obtained the user's consent.
func (s *HarpService) RegisterAccount(deviceName, deviceModel string) error {
	err := s.controller.Accounts().Register(deviceName, deviceModel)
	s.controller.NotifyAccountChanged()
	if err == nil {
		s.controller.Log("info", "Registered a new WARP account")
	}
	return err
}

// ImportAccount copies an account file created by the Harp or wgcf CLI.
func (s *HarpService) ImportAccount() (bool, error) {
	if s.controller.Accounts().Exists() {
		return false, errors.New("remove the current account before importing another one")
	}
	path, err := s.app.Dialog.OpenFile().
		SetTitle("Import WARP account").
		AddFilter("Account files", "*.toml").
		AttachToWindow(s.window).
		PromptForSingleSelection()
	if err != nil || path == "" {
		return false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if !strings.Contains(string(content), "private_key") || !strings.Contains(string(content), "device_id") {
		return false, errors.New("this file does not look like a WARP account file")
	}
	if err := os.WriteFile(s.controller.Accounts().Path(), content, 0o600); err != nil {
		return false, err
	}
	if err := s.reloadAccount(); err != nil {
		return false, err
	}
	s.controller.Log("info", "Imported account from "+path)
	return true, nil
}

func (s *HarpService) reloadAccount() error {
	err := s.controller.Accounts().Reload()
	s.controller.NotifyAccountChanged()
	return err
}

// RemoveAccount disconnects and deletes the local account file. A backup is
// kept next to it.
func (s *HarpService) RemoveAccount() error {
	_ = s.controller.Disconnect()
	err := s.controller.Accounts().Remove()
	s.controller.NotifyAccountChanged()
	return err
}

// UpdateLicense binds this device to a Warp+ license key.
func (s *HarpService) UpdateLicense(licenseKey string) error {
	return s.controller.Accounts().UpdateLicense(strings.TrimSpace(licenseKey))
}

// RenameDevice changes this device's name.
func (s *HarpService) RenameDevice(name string) error {
	return s.controller.Accounts().RenameDevice(strings.TrimSpace(name))
}

// SetDeviceActive activates or deactivates a bound device.
func (s *HarpService) SetDeviceActive(deviceID string, active bool) error {
	return s.controller.Accounts().SetDeviceActive(deviceID, active)
}

// RemoveDevice unbinds a device from the account.
func (s *HarpService) RemoveDevice(deviceID string) error {
	return s.controller.Accounts().RemoveDevice(deviceID)
}

// RefreshPeerConfig downloads the WireGuard peer configuration again.
func (s *HarpService) RefreshPeerConfig() error {
	_, err := s.controller.Accounts().Profile(true)
	if err == nil {
		s.controller.Log("info", "Refreshed the WARP peer configuration")
	}
	return err
}

// ExportWireGuardProfile saves a standard WireGuard configuration file.
func (s *HarpService) ExportWireGuardProfile() (string, error) {
	content, err := s.controller.Accounts().WireGuardConfig(s.controller.Settings().Warp.Keepalive)
	if err != nil {
		return "", err
	}
	return s.saveFile("Export WireGuard profile", "harp-wireguard.conf", "WireGuard profiles", "*.conf", content)
}

// ExportSingBoxConfig saves the generated sing-box configuration.
func (s *HarpService) ExportSingBoxConfig() (string, error) {
	content, err := s.controller.ExportConfig()
	if err != nil {
		return "", err
	}
	return s.saveFile("Export sing-box configuration", "harp-config.json", "JSON files", "*.json", content)
}

// ---- Rules ----

// ListProcesses lists running programs for the application picker.
func (s *HarpService) ListProcesses() ([]core.Process, error) {
	return core.ListProcesses()
}

// BrowseApplication lets the user pick an executable and returns its path.
func (s *HarpService) BrowseApplication() (string, error) {
	dialog := s.app.Dialog.OpenFile().SetTitle("Choose an application").AttachToWindow(s.window)
	switch runtime.GOOS {
	case "windows":
		dialog.AddFilter("Applications", "*.exe")
	case "darwin":
		dialog.TreatsFilePackagesAsDirectories(false)
	}
	return dialog.PromptForSingleSelection()
}

// ImportRules reads rules from a rules file chosen by the user.
func (s *HarpService) ImportRules() ([]core.Rule, error) {
	path, err := s.app.Dialog.OpenFile().
		SetTitle("Import rules").
		AddFilter("Rule files", "*.txt;*.rules;*.list").
		AddFilter("All files", "*.*").
		AttachToWindow(s.window).
		PromptForSingleSelection()
	if err != nil || path == "" {
		return []core.Rule{}, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return core.ParseRulesText(string(content))
}

// ExportRules writes rules to a file usable with "harp proxy --rules".
func (s *HarpService) ExportRules(rules []core.Rule) (string, error) {
	return s.saveFile("Export rules", "harp-rules.txt", "Rule files", "*.txt", core.FormatRulesText(rules))
}

// ---- Desktop integration ----

// OpenURL opens a link in the default browser.
func (s *HarpService) OpenURL(url string) error {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return errors.New("only web links can be opened")
	}
	return s.app.Browser.OpenURL(url)
}

// CopyText puts text on the clipboard.
func (s *HarpService) CopyText(text string) bool {
	return s.app.Clipboard.SetText(text)
}

// Quit disconnects and exits the application.
func (s *HarpService) Quit() {
	s.quit()
}

// saveFile asks for a location and writes content there. It returns the
// path, or an empty string when the user cancels.
func (s *HarpService) saveFile(title, filename, filterName, pattern, content string) (string, error) {
	path, err := s.app.Dialog.SaveFile().
		SetMessage(title).
		SetFilename(filename).
		AddFilter(filterName, pattern).
		AttachToWindow(s.window).
		PromptForSingleSelection()
	if err != nil || path == "" {
		return "", err
	}
	if filepath.Ext(path) == "" {
		path += filepath.Ext(filename)
	}
	return path, os.WriteFile(path, []byte(content), 0o600)
}
