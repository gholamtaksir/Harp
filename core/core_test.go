package core

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gholamtaksir/Harp/engine"
	"github.com/gholamtaksir/Harp/warp"
)

func testProfile() *warp.Profile {
	return &warp.Profile{
		PrivateKey:    "OP+0TxVB0xaYtLthaqIOxPuKdEv7FsWPuFcmk4INWHs=",
		IPv4:          "172.16.0.2",
		IPv6:          "2606:4700:110:8a36:df92:102a:9602:fa18",
		PeerPublicKey: "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=",
		Endpoint:      "162.159.192.1:2408",
		Reserved:      []byte{1, 2, 3},
	}
}

func TestDefaultSettingsAreValid(t *testing.T) {
	if err := DefaultSettings().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateReportsEveryField(t *testing.T) {
	s := DefaultSettings()
	s.Mode = "vpn"
	s.Proxy.Listeners = append(s.Proxy.Listeners, Listener{Type: "mixed", Address: "127.0.0.1", Port: 1080, Enabled: true})
	s.Proxy.AuthEnabled = true
	s.Tun.ExcludeRoutes = []string{"not-a-cidr"}
	s.Routing.Rules = []Rule{{ID: "r1", Action: "allow", Type: "ip", Value: "999.1.1.1", Enabled: true}}
	s.DNS.Direct = "tls://dns.google"
	s.Warp.Endpoint = "nope"
	err := s.Validate()
	validation, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	fields := map[string]bool{}
	for _, field := range validation.Fields {
		fields[field.Field] = true
	}
	for _, want := range []string{"mode", "proxy.listeners[1]", "proxy.users", "tun.excludeRoutes", "routing.rules.r1", "dns.direct", "warp.endpoint"} {
		if !fields[want] {
			t.Errorf("missing error for %s in %v", want, err)
		}
	}
}

func TestSettingsStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "settings.json")
	store := NewSettingsStore(path)
	loaded, err := store.Load()
	if err != nil || loaded.Mode != ModeProxy {
		t.Fatalf("missing file should load defaults: %+v %v", loaded, err)
	}
	loaded.Mode = ModeTun
	loaded.Routing.Rules = []Rule{{ID: "a", Action: "direct", Type: "geoip", Value: "ir", Enabled: true}}
	if err := store.Save(loaded); err != nil {
		t.Fatal(err)
	}
	again, err := store.Load()
	if err != nil || again.Mode != ModeTun || len(again.Routing.Rules) != 1 {
		t.Fatalf("round trip failed: %+v %v", again, err)
	}

	// Fields missing from an older file keep their defaults.
	if err := os.WriteFile(path, []byte(`{"mode":"tun"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	partial, err := store.Load()
	if err != nil || partial.Warp.MTU != 1280 || len(partial.Proxy.Listeners) != 1 {
		t.Fatalf("defaults not applied: %+v %v", partial, err)
	}
}

func TestEngineOptions(t *testing.T) {
	s := DefaultSettings()
	s.Proxy.AuthEnabled = true
	s.Proxy.Users = []User{{Username: "alice", Password: "secret"}}
	s.Proxy.SystemProxy = true
	s.Proxy.Listeners = append(s.Proxy.Listeners, Listener{Type: "socks", Address: "0.0.0.0", Port: 1081, Enabled: false})
	s.Routing.Rules = []Rule{
		{ID: "1", Action: "direct", Type: "geoip", Value: "ir", Enabled: true},
		{ID: "2", Action: "block", Type: "geosite", Value: "category-ads-all", Enabled: false},
		{ID: "3", Action: "warp", Type: "app", Value: "chrome.exe", Enabled: true},
		{ID: "4", Action: "direct", Type: "private", Enabled: true},
	}
	w, err := WarpEndpoint(s.Warp, testProfile())
	if err != nil {
		t.Fatal(err)
	}
	options, err := EngineOptions(s, w, "cache.db")
	if err != nil {
		t.Fatal(err)
	}
	if len(options.Listeners) != 1 || !options.Listeners[0].SetSystemProxy || len(options.Listeners[0].Users) != 1 {
		t.Errorf("unexpected listeners: %+v", options.Listeners)
	}
	if len(options.Rules) != 3 {
		t.Errorf("disabled rules must be skipped: %+v", options.Rules)
	}
	if options.Tun != nil {
		t.Error("proxy mode must not create a TUN")
	}
	config, err := engine.Build(options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ParseObject(engine.NewContext(), config); err != nil {
		t.Fatalf("sing-box rejected the configuration: %v", err)
	}

	s.Mode = ModeTun
	options, err = EngineOptions(s, w, "")
	if err != nil {
		t.Fatal(err)
	}
	if options.Tun == nil || len(options.Listeners) != 0 {
		t.Errorf("TUN mode without proxy: tun=%v listeners=%v", options.Tun, options.Listeners)
	}
	s.Tun.ServeProxy = true
	options, _ = EngineOptions(s, w, "")
	if len(options.Listeners) != 1 || options.Listeners[0].SetSystemProxy {
		t.Errorf("TUN mode with proxy must not set the system proxy: %+v", options.Listeners)
	}

	s.Mode = ModeProxy
	s.Proxy.Listeners = nil
	if _, err := EngineOptions(s, w, ""); err == nil {
		t.Error("proxy mode without listeners must fail")
	}
}

func TestWarpEndpointOptions(t *testing.T) {
	s := DefaultSettings().Warp
	s.Reserved = false
	s.Endpoint = "188.114.96.1:500"
	w, err := WarpEndpoint(s, testProfile())
	if err != nil {
		t.Fatal(err)
	}
	if w.Server != "188.114.96.1" || w.Port != 500 || w.Reserved != nil || len(w.Address) != 2 {
		t.Errorf("unexpected endpoint: %+v", w)
	}
}

func TestLogBuffer(t *testing.T) {
	buffer := newLogBuffer(3)
	buffer.WriteMessage(2, "\x1b[31mERROR\x1b[0m[0000] router: boom")
	for i := 0; i < 4; i++ {
		buffer.add("info", "line")
	}
	entries := buffer.snapshot()
	if len(entries) != 3 || entries[2].ID != 5 {
		t.Errorf("capacity not enforced: %+v", entries)
	}
	pending := buffer.takePending()
	if len(pending) != 3 || len(buffer.takePending()) != 0 {
		t.Errorf("pending not drained: %+v", pending)
	}
	buffer = newLogBuffer(10)
	buffer.WriteMessage(2, "\x1b[31mERROR\x1b[0m[0000] router: boom")
	if got := buffer.snapshot()[0]; got.Message != "router: boom" || got.Level != "error" {
		t.Errorf("prefix not stripped: %+v", got)
	}
	buffer.WriteMessage(5, "DEBUG[0000] dropped at the default info level")
	buffer.setLevel("debug")
	buffer.WriteMessage(5, "DEBUG[0000] kept")
	if entries := buffer.snapshot(); len(entries) != 2 || entries[1].Message != "kept" {
		t.Errorf("level filter not applied: %+v", entries)
	}
}

func TestRulesTextRoundTrip(t *testing.T) {
	rules := []Rule{
		{ID: "1", Action: "direct", Type: "geoip", Value: "ir", Enabled: true, Note: "Local sites"},
		{ID: "2", Action: "block", Type: "port", Value: "6000:7000", Enabled: true},
		{ID: "3", Action: "warp", Type: "app", Value: "chrome.exe", Enabled: false},
		{ID: "4", Action: "direct", Type: "private", Enabled: true},
		{ID: "5", Action: "direct", Type: "path", Value: `C:\Program Files\App\app.exe`, Enabled: true},
	}
	parsed, err := ParseRulesText(FormatRulesText(rules))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 4 {
		t.Fatalf("want 4 enabled rules, got %+v", parsed)
	}
	for i, want := range []Rule{rules[0], rules[1], rules[3], rules[4]} {
		got := parsed[i]
		if got.Action != want.Action || got.Type != want.Type || got.Value != want.Value || !got.Enabled || got.ID == "" {
			t.Errorf("rule %d = %+v, want %+v", i, got, want)
		}
	}
}

func TestParseTrace(t *testing.T) {
	values, err := parseTrace(strings.NewReader("fl=1\nip=1.2.3.4\nloc=DE\ncolo=FRA\nwarp=plus\n"))
	if err != nil || values["ip"] != "1.2.3.4" || values["warp"] != "plus" || values["colo"] != "FRA" {
		t.Errorf("parseTrace = %v, %v", values, err)
	}
}

const fakeAccount = `access_token = 'token'
device_id = 'device'
license_key = ''
private_key = 'OP+0TxVB0xaYtLthaqIOxPuKdEv7FsWPuFcmk4INWHs='
ipv4 = '172.16.0.2'
ipv6 = '2606:4700:110:8a36:df92:102a:9602:fa18'
peer_public_key = 'bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo='
endpoint = '127.0.0.1:9'
endpoint_ip = '127.0.0.1:9'
client_id = 'AQID'
`

func freePort(t *testing.T) int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// TestControllerLifecycle starts a real engine against a local, silent
// endpoint: the tunnel never handshakes, but the lifecycle, statistics and
// connection tracking can be exercised offline.
func TestControllerLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("starts a network service")
	}
	paths := DefaultPaths(t.TempDir())
	if err := os.WriteFile(paths.Account, []byte(fakeAccount), 0o600); err != nil {
		t.Fatal(err)
	}
	stats := make(chan Stats, 10)
	controller, err := NewController(paths, func(name string, data any) {
		if name == EventStats {
			stats <- data.(Stats)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer controller.Shutdown()

	settings := controller.Settings()
	settings.Proxy.Listeners[0].Port = freePort(t)
	settings.Routing.Rules = []Rule{{ID: "1", Action: "direct", Type: "private", Enabled: true}}
	// A fixed endpoint: automatic mode would probe the Internet.
	settings.Warp.Endpoint = "127.0.0.1:9"
	if err := controller.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := controller.Connect(); err != nil {
		t.Fatal(err)
	}
	state := controller.State()
	if state.Status != StatusConnected || len(state.Listeners) != 1 || state.Endpoint != "127.0.0.1:9" {
		t.Fatalf("unexpected state: %+v", state)
	}
	select {
	case <-stats:
	case <-time.After(3 * time.Second):
		t.Error("no traffic statistics received")
	}
	if connections := controller.Connections(); connections == nil {
		t.Error("connections must be an empty list, not nil")
	}

	// Changing a connection setting restarts the engine on a new port.
	settings.Proxy.Listeners[0].Port = freePort(t)
	if err := controller.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	if state := controller.State(); state.Status != StatusConnected ||
		!strings.HasSuffix(state.Listeners[0], ":"+strconv.Itoa(settings.Proxy.Listeners[0].Port)) {
		t.Errorf("settings not applied by reconnecting: %+v", state)
	}
	if _, err := controller.ExportConfig(); err != nil {
		t.Errorf("ExportConfig: %v", err)
	}
	if err := controller.Disconnect(); err != nil || controller.State().Status != StatusDisconnected {
		t.Errorf("Disconnect: %v %+v", err, controller.State())
	}
}

func TestControllerWithoutAccount(t *testing.T) {
	var events []string
	controller, err := NewController(DefaultPaths(t.TempDir()), func(name string, _ any) {
		events = append(events, name)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer controller.Shutdown()
	if err := controller.Connect(); err != ErrNoAccount {
		t.Fatalf("Connect without account = %v", err)
	}
	state := controller.State()
	if state.Status != StatusError || state.ErrorCode != ErrorCodeNoAccount || state.HasAccount {
		t.Errorf("unexpected state: %+v", state)
	}
	if err := controller.Disconnect(); err != nil || controller.State().Status != StatusDisconnected {
		t.Errorf("Disconnect should clear the error: %v %+v", err, controller.State())
	}
	if err := controller.SetMode(ModeTun); err != nil || controller.Settings().Mode != ModeTun || controller.State().Mode != ModeTun {
		t.Errorf("SetMode failed: %v", err)
	}
	if len(events) == 0 {
		t.Error("no events emitted")
	}
	if processes, err := ListProcesses(); err != nil || len(processes) == 0 {
		t.Errorf("ListProcesses = %d, %v", len(processes), err)
	}
}
