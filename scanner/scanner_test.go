package scanner

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/sagernet/wireguard-go/conn"
	"github.com/sagernet/wireguard-go/device"
	"github.com/sagernet/wireguard-go/tun"
	"golang.org/x/crypto/curve25519"
)

// fakeTun is a TUN device that never produces packets.
type fakeTun struct {
	events chan tun.Event
	closed chan struct{}
	once   sync.Once
}

func newFakeTun() *fakeTun {
	return &fakeTun{events: make(chan tun.Event, 1), closed: make(chan struct{})}
}

func (t *fakeTun) File() *os.File { return nil }
func (t *fakeTun) Read([][]byte, []int, int) (int, error) {
	<-t.closed
	return 0, os.ErrClosed
}
func (t *fakeTun) Write(bufs [][]byte, _ int) (int, error) { return len(bufs), nil }
func (t *fakeTun) MTU() (int, error)                       { return 1420, nil }
func (t *fakeTun) Name() (string, error)                   { return "fake", nil }
func (t *fakeTun) Events() <-chan tun.Event                { return t.events }
func (t *fakeTun) BatchSize() int                          { return 1 }
func (t *fakeTun) Close() error {
	t.once.Do(func() { close(t.closed); close(t.events) })
	return nil
}

func newKeyPair(t *testing.T) (private, public []byte) {
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

func freeUDPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.LocalAddr().(*net.UDPAddr).Port
}

// startServer runs a real WireGuard device on localhost that accepts
// clientPublic, and returns its address and public key.
func startServer(t *testing.T, clientPublic []byte) (netip.AddrPort, string) {
	t.Helper()
	serverPrivate, serverPublic := newKeyPair(t)
	port := freeUDPPort(t)
	logger := device.NewLogger(device.LogLevelSilent, "")
	wg := device.NewDevice(context.Background(), newFakeTun(), conn.NewStdNetBind(nil), logger, 1)
	t.Cleanup(wg.Close)
	config := fmt.Sprintf("private_key=%s\nlisten_port=%d\npublic_key=%s\nallowed_ip=10.0.0.2/32\n",
		hex.EncodeToString(serverPrivate), port, hex.EncodeToString(clientPublic))
	if err := wg.IpcSet(config); err != nil {
		t.Fatal(err)
	}
	if err := wg.Up(); err != nil {
		t.Fatal(err)
	}
	return netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(port)), base64.StdEncoding.EncodeToString(serverPublic)
}

func TestProbeRealWireGuard(t *testing.T) {
	clientPrivate, clientPublic := newKeyPair(t)
	server, serverPublic := startServer(t, clientPublic)

	keys, err := NewKeys(base64.StdEncoding.EncodeToString(clientPrivate), serverPublic, nil)
	if err != nil {
		t.Fatal(err)
	}
	rtt, err := keys.Probe(context.Background(), server, 2*time.Second)
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	if rtt < 0 || rtt > 2*time.Second {
		t.Errorf("rtt = %v", rtt)
	}

	// An unknown client must be ignored, exactly like a WARP server would.
	strangerPrivate, _ := newKeyPair(t)
	stranger, err := NewKeys(base64.StdEncoding.EncodeToString(strangerPrivate), serverPublic, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stranger.Probe(context.Background(), server, 300*time.Millisecond); err != ErrNoResponse {
		t.Errorf("unknown client: err = %v, want ErrNoResponse", err)
	}
}

func TestReservedBytesAfterMAC(t *testing.T) {
	clientPrivate, clientPublic := newKeyPair(t)
	server, serverPublic := startServer(t, clientPublic)
	keys, err := NewKeys(base64.StdEncoding.EncodeToString(clientPrivate), serverPublic, []byte{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	sender := randomIndex()
	packet, err := keys.initiation(sender, nextTimestamp())
	if err != nil {
		t.Fatal(err)
	}
	if packet[0] != 1 || packet[1] != 1 || packet[2] != 2 || packet[3] != 3 {
		t.Fatalf("header = %v", packet[:4])
	}
	// Servers clear the reserved bytes before checking the MAC; a plain
	// WireGuard server needs them cleared to accept the message at all.
	copy(packet[1:4], []byte{0, 0, 0})
	udp, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(server))
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	_ = udp.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := udp.Write(packet); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 256)
	n, err := udp.Read(buffer)
	if err != nil {
		t.Fatalf("no response: %v", err)
	}
	if !isAnswer(buffer[:n], sender) {
		t.Errorf("unexpected answer %x", buffer[:n])
	}
}

func TestScan(t *testing.T) {
	clientPrivate, clientPublic := newKeyPair(t)
	server, serverPublic := startServer(t, clientPublic)
	keys, err := NewKeys(base64.StdEncoding.EncodeToString(clientPrivate), serverPublic, nil)
	if err != nil {
		t.Fatal(err)
	}
	dead := netip.AddrPortFrom(server.Addr(), uint16(freeUDPPort(t)))
	var mu sync.Mutex
	var last Progress
	found := 0
	results := keys.Scan(context.Background(), Options{
		Candidates: []netip.AddrPort{dead, server},
		Timeout:    500 * time.Millisecond,
		Samples:    4,
		Interval:   40 * time.Millisecond,
	}, func(progress Progress, result *Result) {
		mu.Lock()
		defer mu.Unlock()
		last = progress
		if result != nil && progress.Phase == PhaseProbing {
			found++
		}
	})
	if len(results) != 1 || results[0].Endpoint != server {
		t.Fatalf("results = %+v", results)
	}
	result := results[0]
	if result.Sent != 4 || result.Received != 4 || result.Loss() != 0 || result.RTT < 0 {
		t.Errorf("result = %+v", result)
	}
	if found != 1 || last.Phase != PhaseMeasuring || last.Done != 1 {
		t.Errorf("progress: found %d, last %+v", found, last)
	}

	// WireGuard drops initiations less than 20 ms apart from the same peer.
	time.Sleep(50 * time.Millisecond)
	quick := keys.Quick(context.Background(), []netip.AddrPort{dead, server}, 500*time.Millisecond)
	if len(quick) != 1 || quick[0].Endpoint != server {
		t.Errorf("quick = %+v", quick)
	}
}

func TestCandidates(t *testing.T) {
	extra := []netip.AddrPort{netip.MustParseAddrPort("1.2.3.4:5"), netip.MustParseAddrPort("1.2.3.4:5")}
	candidates := Candidates(500, IPv4Prefixes, Ports, extra)
	if len(candidates) != 500 || candidates[0] != extra[0] {
		t.Fatalf("got %d candidates, first %v", len(candidates), candidates[0])
	}
	seen := map[netip.AddrPort]bool{}
	for _, candidate := range candidates[1:] {
		if seen[candidate] {
			t.Fatalf("duplicate %v", candidate)
		}
		seen[candidate] = true
		inRange := false
		for _, prefix := range IPv4Prefixes {
			inRange = inRange || prefix.Contains(candidate.Addr())
		}
		last := candidate.Addr().As4()[3]
		if !inRange || last == 0 || last == 255 {
			t.Fatalf("bad candidate %v", candidate)
		}
	}
	for _, candidate := range Candidates(20, IPv6Prefixes, []uint16{2408}, nil) {
		if !candidate.Addr().Is6() || !(IPv6Prefixes[0].Contains(candidate.Addr()) || IPv6Prefixes[1].Contains(candidate.Addr())) {
			t.Fatalf("bad IPv6 candidate %v", candidate)
		}
	}
	// A tiny space must not loop forever.
	if got := Candidates(10, []netip.Prefix{netip.MustParsePrefix("10.0.0.1/32")}, []uint16{1}, nil); len(got) != 1 {
		t.Errorf("tiny space: %v", got)
	}
}

func TestStopwatch(t *testing.T) {
	watch := startStopwatch()
	time.Sleep(20 * time.Millisecond)
	if elapsed := watch.elapsed(); elapsed < 15*time.Millisecond || elapsed > 500*time.Millisecond {
		t.Errorf("elapsed = %v", elapsed)
	}
}
