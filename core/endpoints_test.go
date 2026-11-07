package core

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/wireguard-go/conn"
	"github.com/sagernet/wireguard-go/device"
	"github.com/sagernet/wireguard-go/tun"
	"golang.org/x/crypto/curve25519"
)

type silentTun struct {
	events chan tun.Event
	closed chan struct{}
	once   sync.Once
}

func (t *silentTun) File() *os.File { return nil }
func (t *silentTun) Read([][]byte, []int, int) (int, error) {
	<-t.closed
	return 0, os.ErrClosed
}
func (t *silentTun) Write(bufs [][]byte, _ int) (int, error) { return len(bufs), nil }
func (t *silentTun) MTU() (int, error)                       { return 1420, nil }
func (t *silentTun) Name() (string, error)                   { return "silent", nil }
func (t *silentTun) Events() <-chan tun.Event                { return t.events }
func (t *silentTun) BatchSize() int                          { return 1 }
func (t *silentTun) Close() error {
	t.once.Do(func() { close(t.closed); close(t.events) })
	return nil
}

func keyPair(t *testing.T) (private, public []byte) {
	t.Helper()
	private = make([]byte, 32)
	if _, err := rand.Read(private); err != nil {
		t.Fatal(err)
	}
	private[0] &= 248
	private[31] = private[31]&127 | 64
	public, err := curve25519.X25519(private, curve25519.Basepoint)
	if err != nil {
		t.Fatal(err)
	}
	return private, public
}

func unusedUDPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.LocalAddr().(*net.UDPAddr).Port
}

// fakeWarp runs a WireGuard server on localhost that knows one client and
// returns an account file for that client plus the server address.
func fakeWarp(t *testing.T) (account string, server string) {
	t.Helper()
	clientPrivate, clientPublic := keyPair(t)
	serverPrivate, serverPublic := keyPair(t)
	port := unusedUDPPort(t)
	wg := device.NewDevice(context.Background(), &silentTun{events: make(chan tun.Event, 1), closed: make(chan struct{})},
		conn.NewStdNetBind(nil), device.NewLogger(device.LogLevelSilent, ""), 1)
	t.Cleanup(wg.Close)
	if err := wg.IpcSet(fmt.Sprintf("private_key=%s\nlisten_port=%d\npublic_key=%s\nallowed_ip=172.16.0.2/32\n",
		hex.EncodeToString(serverPrivate), port, hex.EncodeToString(clientPublic))); err != nil {
		t.Fatal(err)
	}
	if err := wg.Up(); err != nil {
		t.Fatal(err)
	}
	dead := fmt.Sprintf("127.0.0.1:%d", unusedUDPPort(t))
	account = strings.NewReplacer(
		"OP+0TxVB0xaYtLthaqIOxPuKdEv7FsWPuFcmk4INWHs=", base64.StdEncoding.EncodeToString(clientPrivate),
		"bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=", base64.StdEncoding.EncodeToString(serverPublic),
		"'127.0.0.1:9'", "'"+dead+"'",
		"client_id = 'AQID'", "client_id = ''",
	).Replace(fakeAccount)
	return account, fmt.Sprintf("127.0.0.1:%d", port)
}

func newTestController(t *testing.T, dir, account string) *Controller {
	t.Helper()
	paths := DefaultPaths(dir)
	if err := os.WriteFile(paths.Account, []byte(account), 0o600); err != nil {
		t.Fatal(err)
	}
	controller, err := NewController(paths, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Never probe the Internet from tests.
	controller.endpoints.wellKnown = nil
	controller.endpoints.prefixesV4 = nil
	controller.endpoints.prefixesV6 = nil
	t.Cleanup(controller.Shutdown)
	return controller
}

func TestChooseEndpointAndMarks(t *testing.T) {
	account, server := fakeWarp(t)
	dir := t.TempDir()
	controller := newTestController(t, dir, account)
	profile, err := controller.accounts.Profile(false)
	if err != nil {
		t.Fatal(err)
	}

	// Nothing known answers yet (the account endpoint is a dead port), and a
	// scan without prefixes finds nothing either.
	if _, err := controller.chooseEndpoint(context.Background(), profile, controller.Settings()); err != ErrNoEndpoint {
		t.Fatalf("chooseEndpoint with no working endpoint = %v", err)
	}

	// A dead "last" endpoint is skipped in favour of a known one that answers.
	dead := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(unusedUDPPort(t))).String()
	controller.endpoints.mu.Lock()
	controller.endpoints.db.Last = dead
	controller.endpoints.db.Results = []EndpointResult{{Endpoint: server, Status: EndpointAnswered, Ping: 1}}
	controller.endpoints.mu.Unlock()
	chosen, err := controller.chooseEndpoint(context.Background(), profile, controller.Settings())
	if err != nil || chosen != server {
		t.Fatalf("chooseEndpoint = %q, %v; want %s", chosen, err, server)
	}

	controller.markEndpoint(server, Trace{OK: true, Colo: "FRA", Location: "DE", LatencyMs: 42})
	state := controller.ScanState()
	if state.Last != server || len(state.Results) == 0 || state.Results[0].Status != EndpointWorking ||
		state.Results[0].City != "Frankfurt" || state.Results[0].Country != "DE" || state.Results[0].Latency != 42 {
		t.Fatalf("after success: %+v", state)
	}

	// The database survives a restart.
	reloaded := newTestController(t, dir, account)
	if reloaded.ScanState().Last != server {
		t.Errorf("last endpoint not persisted: %+v", reloaded.ScanState())
	}

	// A failed endpoint is skipped for a while.
	controller.markEndpoint(server, Trace{Error: "timed out"})
	state = controller.ScanState()
	if state.Last != "" || state.Results[0].Status != EndpointBlocked {
		t.Errorf("after failure: %+v", state)
	}
	for _, candidate := range controller.knownCandidates(profile, true) {
		if candidate.String() == server {
			t.Error("failed endpoint still offered")
		}
	}
}

func TestScanFindsServer(t *testing.T) {
	account, server := fakeWarp(t)
	controller := newTestController(t, t.TempDir(), account)
	controller.endpoints.wellKnown = []netip.AddrPort{netip.MustParseAddrPort(server)}
	var mu sync.Mutex
	var events []ScanState
	controller.emit = func(name string, data any) {
		if name == EventScan {
			mu.Lock()
			events = append(events, data.(ScanState))
			mu.Unlock()
		}
	}
	if err := controller.StartScan(ScanRequest{Size: "quick"}); err != nil {
		t.Fatal(err)
	}
	if err := controller.StartScan(ScanRequest{}); err == nil {
		t.Error("a second scan must be refused while one runs")
	}
	deadline := time.Now().Add(10 * time.Second)
	for controller.ScanState().Running && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	state := controller.ScanState()
	if state.Running || state.Phase != ScanDone || state.Found != 1 || len(state.Results) != 1 ||
		state.Results[0].Endpoint != server || state.Results[0].Status != EndpointAnswered || state.Results[0].Loss != 0 {
		t.Fatalf("scan state: %+v", state)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(events) < 2 || !events[0].Running || events[len(events)-1].Running {
		t.Errorf("scan events: %d", len(events))
	}
}

func TestResultOrdering(t *testing.T) {
	results := []EndpointResult{
		{Endpoint: "blocked", Status: EndpointBlocked, Ping: 10},
		{Endpoint: "slow", Status: EndpointAnswered, Ping: 300},
		{Endpoint: "lossy", Status: EndpointAnswered, Ping: 50, Loss: 40},
		{Endpoint: "working", Status: EndpointWorking, Ping: 400},
		{Endpoint: "fast", Status: EndpointAnswered, Ping: 100},
	}
	sortResults(results)
	var order []string
	for _, result := range results {
		order = append(order, result.Endpoint)
	}
	if got := strings.Join(order, ","); got != "working,fast,slow,lossy,blocked" {
		t.Errorf("order = %s", got)
	}
}

func TestNextEndpointFixed(t *testing.T) {
	account, server := fakeWarp(t)
	_, other := fakeWarp(t)
	controller := newTestController(t, t.TempDir(), account)
	// Only the first server knows this client; the second never answers.
	controller.endpoints.wellKnown = []netip.AddrPort{netip.MustParseAddrPort(other), netip.MustParseAddrPort(server)}
	if _, err := controller.NextEndpoint(); err == nil {
		t.Fatal("NextEndpoint must fail while disconnected")
	}
	settings := controller.Settings()
	settings.Proxy.Listeners[0].Port = freePort(t)
	settings.Warp.Endpoint = "127.0.0.1:9"
	if err := controller.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err := controller.Connect(); err != nil {
		t.Fatal(err)
	}
	next, err := controller.NextEndpoint()
	if err != nil || next != server {
		t.Fatalf("NextEndpoint = %q, %v; want %s", next, err, server)
	}
	if got := controller.Settings().Warp.Endpoint; got != server {
		t.Errorf("fixed endpoint = %s", got)
	}
	if state := controller.State(); state.Status != StatusConnected || state.Endpoint != server {
		t.Errorf("state after switch: %+v", state)
	}
}
