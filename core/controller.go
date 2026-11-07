package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/gholamtaksir/Harp/engine"
	"github.com/gholamtaksir/Harp/privilege"
	"github.com/gofrs/uuid/v5"
	"github.com/sagernet/sing-box/option"
)

// Connection states.
const (
	StatusDisconnected  = "disconnected"
	StatusConnecting    = "connecting"
	StatusConnected     = "connected"
	StatusDisconnecting = "disconnecting"
	StatusError         = "error"
)

// Error codes the user interface can react to.
const (
	ErrorCodeElevationRequired = "elevation_required"
	ErrorCodeNoAccount         = "no_account"
	ErrorCodeInvalidSettings   = "invalid_settings"
	ErrorCodeNoEndpoint        = "no_endpoint"
	ErrorCodeFailed            = "failed"
)

// Events emitted by the controller.
const (
	EventState    = "harp:state"
	EventStats    = "harp:stats"
	EventLogs     = "harp:logs"
	EventSettings = "harp:settings"
)

const statsHistoryLength = 300

const (
	// monitorInterval is how often a running connection is checked.
	monitorInterval = 30 * time.Second
	// maxFailovers bounds automatic endpoint switches per connect.
	maxFailovers = 5
)

// ErrElevationRequired is returned when TUN mode needs administrator rights.
var ErrElevationRequired = errors.New("TUN mode requires administrator privileges")

// State describes the connection.
type State struct {
	Status      string `json:"status"`
	Mode        string `json:"mode"`
	Error       string `json:"error"`
	ErrorCode   string `json:"errorCode"`
	ConnectedAt int64  `json:"connectedAt"`
	Endpoint    string `json:"endpoint"`
	// Automatic is set when Harp chose the endpoint itself.
	Automatic bool     `json:"automatic"`
	Listeners []string `json:"listeners"`
	// Notice explains an automatic action, such as switching endpoints.
	Notice     string `json:"notice"`
	Trace      Trace  `json:"trace"`
	Checking   bool   `json:"checking"`
	Elevated   bool   `json:"elevated"`
	HasAccount bool   `json:"hasAccount"`
}

// Stats is one second of traffic.
type Stats struct {
	Time        int64 `json:"time"`
	Up          int64 `json:"up"`
	Down        int64 `json:"down"`
	UpTotal     int64 `json:"upTotal"`
	DownTotal   int64 `json:"downTotal"`
	Connections int   `json:"connections"`
}

// Connection is an active connection.
type Connection struct {
	ID          string   `json:"id"`
	Network     string   `json:"network"`
	Protocol    string   `json:"protocol"`
	Inbound     string   `json:"inbound"`
	Host        string   `json:"host"`
	Destination string   `json:"destination"`
	Source      string   `json:"source"`
	Process     string   `json:"process"`
	ProcessPath string   `json:"processPath"`
	Rule        string   `json:"rule"`
	Outbound    string   `json:"outbound"`
	Chain       []string `json:"chain"`
	Upload      int64    `json:"upload"`
	Download    int64    `json:"download"`
	Start       int64    `json:"start"`
}

// Paths are the files the controller uses.
type Paths struct {
	Settings  string
	Account   string
	CacheFile string
	// Endpoints stores scan results; empty keeps them in memory.
	Endpoints string
}

// DefaultPaths places all files in dataDir.
func DefaultPaths(dataDir string) Paths {
	return Paths{
		Settings:  filepath.Join(dataDir, "settings.json"),
		Account:   filepath.Join(dataDir, "harp-account.toml"),
		CacheFile: filepath.Join(dataDir, "cache.db"),
		Endpoints: filepath.Join(dataDir, "endpoints.json"),
	}
}

// Controller owns the connection lifecycle.
type Controller struct {
	paths     Paths
	store     *SettingsStore
	accounts  *AccountManager
	endpoints *endpoints
	emit      func(name string, data any)
	logs      *logBuffer

	// operation serializes connect, disconnect and reconnect.
	operation sync.Mutex
	// failovers counts automatic endpoint switches since the user
	// connected. Guarded by operation.
	failovers int
	// notice is shown while reconnecting after a failover. Guarded by
	// operation.
	notice string

	mu       sync.Mutex
	settings Settings
	state    State
	instance *engine.Instance
	stop     chan struct{}
	// cancelConnect aborts a connection attempt in progress.
	cancelConnect context.CancelFunc
	history       []Stats
	lastTotal     [2]int64

	shutdown chan struct{}
}

// NewController loads the settings and account. emit delivers events to the
// user interface and may be nil.
func NewController(paths Paths, emit func(name string, data any)) (*Controller, error) {
	if emit == nil {
		emit = func(string, any) {}
	}
	store := NewSettingsStore(paths.Settings)
	// A corrupt file must not lock the user out: Load falls back to defaults.
	settings, loadErr := store.Load()
	accounts, err := NewAccountManager(paths.Account)
	if err != nil {
		return nil, err
	}
	c := &Controller{
		paths:     paths,
		store:     store,
		accounts:  accounts,
		endpoints: newEndpoints(paths.Endpoints),
		emit:      emit,
		logs:      newLogBuffer(3000),
		settings:  settings,
		shutdown:  make(chan struct{}),
	}
	c.state = State{Status: StatusDisconnected, Mode: settings.Mode, Listeners: []string{}}
	if loadErr != nil {
		c.logs.add("warn", "Settings could not be read and were reset: "+loadErr.Error())
	}
	go c.flushLogs()
	return c, nil
}

// Accounts gives access to account management.
func (c *Controller) Accounts() *AccountManager {
	return c.accounts
}

// Paths returns the files used by the controller.
func (c *Controller) Paths() Paths {
	return c.paths
}

// State returns the current connection state.
func (c *Controller) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stateLocked()
}

func (c *Controller) stateLocked() State {
	state := c.state
	state.Elevated = privilege.IsElevated()
	state.HasAccount = c.accounts.Exists()
	state.Listeners = append([]string{}, state.Listeners...)
	return state
}

func (c *Controller) update(change func(state *State)) {
	c.mu.Lock()
	change(&c.state)
	state := c.stateLocked()
	c.mu.Unlock()
	c.emit(EventState, state)
}

// NotifyAccountChanged re-emits the state after the account changed.
func (c *Controller) NotifyAccountChanged() {
	c.update(func(*State) {})
}

// Settings returns a copy of the current settings.
func (c *Controller) Settings() Settings {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.settings.Clone()
}

// SaveSettings validates and stores settings. A running connection is
// restarted to apply them.
func (c *Controller) SaveSettings(settings Settings) error {
	settings = settings.Clone()
	normalize(&settings)
	if err := settings.Validate(); err != nil {
		return err
	}
	if err := c.store.Save(settings); err != nil {
		return err
	}
	c.mu.Lock()
	previous := c.settings
	c.settings = settings
	running := c.state.Status == StatusConnected
	c.mu.Unlock()
	c.emit(EventSettings, settings)
	if !running {
		c.update(func(state *State) { state.Mode = settings.Mode })
		return nil
	}
	if connectionSettingsChanged(previous, settings) {
		return c.Reconnect()
	}
	return nil
}

// SetMode switches between proxy and TUN mode.
func (c *Controller) SetMode(mode string) error {
	settings := c.Settings()
	settings.Mode = mode
	return c.SaveSettings(settings)
}

func settingsEqual(a, b Settings) bool {
	first, _ := json.Marshal(a)
	second, _ := json.Marshal(b)
	return bytes.Equal(first, second)
}

// connectionSettingsChanged ignores settings that do not affect a running
// connection.
func connectionSettingsChanged(a, b Settings) bool {
	a.App.StartMinimized, b.App.StartMinimized = false, false
	a.App.ConnectOnLaunch, b.App.ConnectOnLaunch = false, false
	a.App.CloseToTray, b.App.CloseToTray = false, false
	a.App.Theme, b.App.Theme = "", ""
	return !settingsEqual(a, b)
}

// elevationRequired reports whether TUN mode needs elevation on this OS. On
// Linux, CAP_NET_ADMIN is enough, which cannot be detected reliably.
func elevationRequired() bool {
	return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
}

// Connect starts the connection with the current settings.
func (c *Controller) Connect() error {
	c.operation.Lock()
	defer c.operation.Unlock()
	c.failovers = 0
	c.notice = ""
	return c.connect()
}

func (c *Controller) connect() error {
	c.mu.Lock()
	if c.instance != nil {
		c.mu.Unlock()
		return nil
	}
	settings := c.settings
	c.mu.Unlock()

	fail := func(code string, err error) error {
		c.logs.add("error", err.Error())
		c.update(func(state *State) {
			state.Status = StatusError
			state.Error = err.Error()
			state.ErrorCode = code
		})
		return err
	}

	if !c.accounts.Exists() {
		return fail(ErrorCodeNoAccount, ErrNoAccount)
	}
	if err := settings.Validate(); err != nil {
		return fail(ErrorCodeInvalidSettings, err)
	}
	if settings.Mode == ModeTun && elevationRequired() && !privilege.IsElevated() {
		return fail(ErrorCodeElevationRequired, ErrElevationRequired)
	}

	c.logs.setLevel(settings.App.LogLevel)
	notice := c.notice
	c.update(func(state *State) {
		*state = State{Status: StatusConnecting, Mode: settings.Mode, Listeners: []string{}, Notice: notice}
	})
	c.logs.add("info", fmt.Sprintf("Connecting in %s mode", modeName(settings.Mode)))

	ctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.cancelConnect = cancel
	c.mu.Unlock()
	defer func() {
		cancel()
		c.mu.Lock()
		c.cancelConnect = nil
		c.mu.Unlock()
	}()

	automatic := settings.Warp.Endpoint == ""
	if automatic {
		profile, err := c.accounts.Profile(false)
		if err != nil {
			return fail(ErrorCodeFailed, err)
		}
		endpoint, err := c.chooseEndpoint(ctx, profile, settings)
		if ctx.Err() != nil {
			c.logs.add("info", "Connection cancelled")
			c.update(func(state *State) {
				*state = State{Status: StatusDisconnected, Mode: settings.Mode, Listeners: []string{}}
			})
			return nil
		}
		if errors.Is(err, ErrNoEndpoint) {
			return fail(ErrorCodeNoEndpoint, err)
		}
		if err != nil {
			return fail(ErrorCodeFailed, err)
		}
		settings.Warp.Endpoint = endpoint
	}

	built, err := c.build(settings)
	if err != nil {
		return fail(ErrorCodeFailed, err)
	}
	instance, err := engine.Start(built.ctx, built.options, c.logs)
	if err != nil {
		return fail(ErrorCodeFailed, err)
	}

	stop := make(chan struct{})
	c.mu.Lock()
	c.instance = instance
	c.stop = stop
	c.history = nil
	c.lastTotal = [2]int64{}
	c.mu.Unlock()
	c.update(func(state *State) {
		state.Status = StatusConnected
		state.ConnectedAt = time.Now().UnixMilli()
		state.Endpoint = built.endpoint
		state.Automatic = automatic
		state.Listeners = built.listeners
		state.Error = ""
		state.ErrorCode = ""
	})
	c.logs.add("info", "Connected through "+built.endpoint)
	go c.collectStats(instance, stop)
	go c.verify(instance, stop, built.endpoint, automatic)
	return nil
}

func modeName(mode string) string {
	if mode == ModeTun {
		return "TUN"
	}
	return "proxy"
}

// Disconnect stops the connection.
func (c *Controller) Disconnect() error {
	// A connection attempt may be searching for an endpoint; stop it
	// instead of waiting for it.
	c.mu.Lock()
	if c.cancelConnect != nil {
		c.cancelConnect()
	}
	c.mu.Unlock()
	c.operation.Lock()
	defer c.operation.Unlock()
	c.notice = ""
	return c.disconnect()
}

func (c *Controller) disconnect() error {
	c.mu.Lock()
	instance := c.instance
	stop := c.stop
	c.instance = nil
	c.stop = nil
	c.mu.Unlock()
	if instance == nil {
		// Also clears a previous error.
		c.update(func(state *State) {
			state.Status = StatusDisconnected
			state.Error, state.ErrorCode = "", ""
		})
		return nil
	}
	close(stop)
	c.update(func(state *State) { state.Status = StatusDisconnecting })
	err := instance.Close()
	if err != nil {
		c.logs.add("warn", err.Error())
	}
	c.logs.add("info", "Disconnected")
	c.update(func(state *State) {
		mode := state.Mode
		*state = State{Status: StatusDisconnected, Mode: mode, Listeners: []string{}}
	})
	return nil
}

// Reconnect restarts a running connection.
func (c *Controller) Reconnect() error {
	c.operation.Lock()
	defer c.operation.Unlock()
	c.failovers = 0
	c.notice = ""
	if err := c.disconnect(); err != nil {
		return err
	}
	// update holds c.mu while calling the function, so read the mode first.
	mode := c.Settings().Mode
	c.update(func(state *State) { state.Mode = mode })
	return c.connect()
}

// Shutdown disconnects and stops background work.
func (c *Controller) Shutdown() {
	c.StopScan()
	_ = c.Disconnect()
	select {
	case <-c.shutdown:
	default:
		close(c.shutdown)
	}
}

type builtConfig struct {
	ctx       context.Context
	options   option.Options
	endpoint  string
	listeners []string
}

func (c *Controller) build(settings Settings) (*builtConfig, error) {
	profile, err := c.accounts.Profile(false)
	if err != nil {
		return nil, err
	}
	warpEndpoint, err := WarpEndpoint(settings.Warp, profile)
	if err != nil {
		return nil, err
	}
	options, err := EngineOptions(settings, warpEndpoint, c.paths.CacheFile)
	if err != nil {
		return nil, err
	}
	config, err := engine.Build(options)
	if err != nil {
		return nil, err
	}
	ctx := engine.NewContext()
	parsed, err := engine.ParseObject(ctx, config)
	if err != nil {
		return nil, err
	}
	built := &builtConfig{
		ctx:      ctx,
		options:  parsed,
		endpoint: net.JoinHostPort(warpEndpoint.Server, fmt.Sprint(warpEndpoint.Port)),
	}
	for _, listener := range options.Listeners {
		built.listeners = append(built.listeners, listenerURL(listener))
	}
	if built.listeners == nil {
		built.listeners = []string{}
	}
	return built, nil
}

func listenerURL(listener engine.Listener) string {
	scheme := listener.Type
	if scheme == "socks" {
		scheme = "socks5"
	}
	return scheme + "://" + net.JoinHostPort(listener.Listen, fmt.Sprint(listener.Port))
}

// ExportConfig returns the sing-box configuration for the current settings.
// In automatic mode, the endpoint that last carried traffic is used.
func (c *Controller) ExportConfig() (string, error) {
	settings := c.Settings()
	if settings.Warp.Endpoint == "" {
		settings.Warp.Endpoint = c.lastEndpoint()
	}
	built, err := c.build(settings)
	if err != nil {
		return "", err
	}
	content, err := engine.Marshal(built.ctx, built.options)
	return string(content), err
}

func (c *Controller) collectStats(instance *engine.Instance, stop chan struct{}) {
	traffic := instance.Traffic()
	if traffic == nil {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			up, down := traffic.Total()
			c.mu.Lock()
			stats := Stats{
				Time:        now.UnixMilli(),
				Up:          max(0, up-c.lastTotal[0]),
				Down:        max(0, down-c.lastTotal[1]),
				UpTotal:     up,
				DownTotal:   down,
				Connections: traffic.ConnectionsLen(),
			}
			c.lastTotal = [2]int64{up, down}
			c.history = append(c.history, stats)
			if len(c.history) > statsHistoryLength {
				c.history = c.history[len(c.history)-statsHistoryLength:]
			}
			c.mu.Unlock()
			c.emit(EventStats, stats)
		}
	}
}

// StatsHistory returns the recent per-second traffic.
func (c *Controller) StatsHistory() []Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Stats{}, c.history...)
}

// verify checks that traffic actually flows through WARP, then keeps
// checking. In automatic mode, Harp switches to another endpoint when
// traffic stops getting through.
func (c *Controller) verify(instance *engine.Instance, stop chan struct{}, endpoint string, automatic bool) {
	delays := []time.Duration{500 * time.Millisecond, 3 * time.Second, 6 * time.Second}
	if automatic {
		delays = delays[:2]
	}
	var trace Trace
	for _, delay := range delays {
		select {
		case <-stop:
			return
		case <-time.After(delay):
		}
		if trace = c.trace(instance); trace.OK {
			break
		}
	}
	c.markEndpoint(endpoint, trace)
	if trace.OK {
		c.update(func(state *State) { state.Notice = "" })
	} else {
		c.logs.add("warn", "WARP check failed: "+trace.Error)
		if automatic && c.scheduleFailover(instance, stop, endpoint) {
			return
		}
	}
	c.monitor(instance, stop, endpoint, automatic)
}

// monitor checks the connection periodically.
func (c *Controller) monitor(instance *engine.Instance, stop chan struct{}, endpoint string, automatic bool) {
	ticker := time.NewTicker(monitorInterval)
	defer ticker.Stop()
	failures := 0
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		trace := c.trace(instance)
		if trace.OK {
			if failures != 0 {
				c.logs.add("info", "WARP is responding again")
				c.markEndpoint(endpoint, trace)
			}
			failures = 0
			continue
		}
		failures++
		if failures == 1 {
			c.logs.add("warn", "WARP check failed: "+trace.Error)
		}
		// Two failures in a row: the endpoint was probably blocked.
		if failures == 2 {
			c.markEndpoint(endpoint, trace)
			if automatic && c.scheduleFailover(instance, stop, endpoint) {
				return
			}
		}
	}
}

// scheduleFailover reconnects through another endpoint in the background.
// It reports whether a switch was scheduled.
func (c *Controller) scheduleFailover(instance *engine.Instance, stop chan struct{}, endpoint string) bool {
	go func() {
		c.operation.Lock()
		defer c.operation.Unlock()
		c.mu.Lock()
		current := c.instance
		c.mu.Unlock()
		select {
		case <-stop:
			return
		default:
		}
		if current != instance {
			return
		}
		if c.failovers >= maxFailovers {
			c.logs.add("error", fmt.Sprintf("Traffic did not get through the last %d endpoints. Run a scan on the Endpoints page.", maxFailovers+1))
			c.update(func(state *State) {
				state.Notice = "No working endpoint found automatically. Run a scan on the Endpoints page."
			})
			return
		}
		c.failovers++
		c.notice = fmt.Sprintf("Endpoint %s stopped carrying traffic, switching to another one…", endpoint)
		c.logs.add("warn", c.notice)
		if err := c.disconnect(); err != nil {
			return
		}
		_ = c.connect()
	}()
	return true
}

func (c *Controller) trace(instance *engine.Instance) Trace {
	c.update(func(state *State) { state.Checking = true })
	var result Trace
	client, err := instance.HTTPClient(engine.DefaultWarpTag, 10*time.Second)
	if err != nil {
		result = Trace{Error: err.Error(), CheckedAt: time.Now().UnixMilli()}
	} else {
		result = FetchTrace(client)
	}
	c.mu.Lock()
	current := c.instance
	c.mu.Unlock()
	c.update(func(state *State) {
		state.Checking = false
		if current == instance {
			state.Trace = result
		}
	})
	return result
}

// CheckConnection runs the WARP check now.
func (c *Controller) CheckConnection() (Trace, error) {
	c.mu.Lock()
	instance := c.instance
	c.mu.Unlock()
	if instance == nil {
		return Trace{}, errors.New("not connected")
	}
	return c.trace(instance), nil
}

// Connections lists the active connections.
func (c *Controller) Connections() []Connection {
	c.mu.Lock()
	instance := c.instance
	c.mu.Unlock()
	result := []Connection{}
	if instance == nil || instance.Traffic() == nil {
		return result
	}
	for _, metadata := range instance.Traffic().Connections() {
		connection := Connection{
			ID:          metadata.ID.String(),
			Network:     metadata.Metadata.Network,
			Protocol:    metadata.Metadata.Protocol,
			Inbound:     metadata.Metadata.Inbound,
			Destination: metadata.Metadata.Destination.String(),
			Source:      metadata.Metadata.Source.String(),
			Outbound:    metadata.Outbound,
			Chain:       append([]string{}, metadata.Chain...),
			Start:       metadata.CreatedAt.UnixMilli(),
			Rule:        "final",
		}
		connection.Host = metadata.Metadata.Domain
		if connection.Host == "" {
			connection.Host = metadata.Metadata.Destination.AddrString()
		}
		if metadata.Rule != nil {
			connection.Rule = metadata.Rule.String() + " => " + metadata.Rule.Action().String()
		}
		if metadata.Upload != nil {
			connection.Upload = metadata.Upload.Load()
		}
		if metadata.Download != nil {
			connection.Download = metadata.Download.Load()
		}
		if owner := metadata.Metadata.ProcessInfo; owner != nil && owner.ProcessPath != "" {
			connection.ProcessPath = owner.ProcessPath
			connection.Process = filepath.Base(strings.ReplaceAll(owner.ProcessPath, "\\", "/"))
		}
		result = append(result, connection)
	}
	return result
}

// CloseConnection closes one connection.
func (c *Controller) CloseConnection(id string) error {
	parsed, err := uuid.FromString(id)
	if err != nil {
		return errors.WithStack(err)
	}
	c.mu.Lock()
	instance := c.instance
	c.mu.Unlock()
	if instance == nil || instance.Traffic() == nil {
		return nil
	}
	if tracker := instance.Traffic().Connection(parsed); tracker != nil {
		return errors.WithStack(tracker.Close())
	}
	return nil
}

// CloseAllConnections closes every connection.
func (c *Controller) CloseAllConnections() {
	c.mu.Lock()
	instance := c.instance
	c.mu.Unlock()
	if instance != nil && instance.Traffic() != nil {
		instance.Traffic().CloseAllConnections()
	}
}

// Logs returns the buffered log entries.
func (c *Controller) Logs() []LogEntry {
	return c.logs.snapshot()
}

// ClearLogs empties the log buffer.
func (c *Controller) ClearLogs() {
	c.logs.clear()
}

// Log adds an application message to the log.
func (c *Controller) Log(level, message string) {
	c.logs.add(level, message)
}

func (c *Controller) flushLogs() {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-c.shutdown:
			return
		case <-ticker.C:
			if pending := c.logs.takePending(); len(pending) != 0 {
				c.emit(EventLogs, pending)
			}
		}
	}
}
