// Package scanner finds WARP endpoints that answer on the current network by
// sending real WireGuard handshake initiations and timing the responses.
package scanner

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"hash"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"golang.org/x/crypto/blake2s"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
)

// WireGuard message layout (see the WireGuard whitepaper, section 5.4).
const (
	initiationSize  = 148
	responseSize    = 92
	cookieReplySize = 64

	typeInitiation  = 1
	typeResponse    = 2
	typeCookieReply = 3
)

var (
	construction = []byte("Noise_IKpsk2_25519_ChaChaPoly_BLAKE2s")
	identifier   = []byte("WireGuard v1 zx2c4 Jason@zx2c4.com")
	labelMAC1    = []byte("mac1----")
	zeroNonce    [chacha20poly1305.NonceSize]byte
)

// ErrNoResponse means the endpoint did not answer in time.
var ErrNoResponse = errors.New("no response")

// Keys identify the client to the WARP server. Only a peer that knows the
// client's public key answers, so a response proves a real WARP server.
type Keys struct {
	private  [32]byte
	public   [32]byte
	peer     [32]byte
	mac1Key  [32]byte
	reserved [3]byte
}

// NewKeys parses base64 WireGuard keys. reserved holds the client id bytes
// Cloudflare expects in every packet and may be empty.
func NewKeys(privateKey, peerPublicKey string, reserved []byte) (*Keys, error) {
	keys := &Keys{}
	if err := decodeKey(privateKey, &keys.private); err != nil {
		return nil, errors.Wrap(err, "private key")
	}
	if err := decodeKey(peerPublicKey, &keys.peer); err != nil {
		return nil, errors.Wrap(err, "peer public key")
	}
	public, err := curve25519.X25519(keys.private[:], curve25519.Basepoint)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	copy(keys.public[:], public)
	keys.mac1Key = blake2s.Sum256(append(append([]byte{}, labelMAC1...), keys.peer[:]...))
	if len(reserved) != 0 && len(reserved) != 3 {
		return nil, errors.Newf("reserved must be 3 bytes, got %d", len(reserved))
	}
	copy(keys.reserved[:], reserved)
	return keys, nil
}

func decodeKey(value string, key *[32]byte) error {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return errors.WithStack(err)
	}
	if len(decoded) != 32 {
		return errors.Newf("expected 32 bytes, got %d", len(decoded))
	}
	copy(key[:], decoded)
	return nil
}

// initiation builds a handshake initiation message (Noise IK, first message).
func (k *Keys) initiation(sender uint32, timestamp [12]byte) ([]byte, error) {
	message := make([]byte, initiationSize)
	message[0] = typeInitiation
	binary.LittleEndian.PutUint32(message[4:8], sender)

	chainKey := blake2s.Sum256(construction)
	hashState := mixHash(chainKey, identifier)
	hashState = mixHash(hashState, k.peer[:])

	var ephemeralPrivate [32]byte
	if _, err := rand.Read(ephemeralPrivate[:]); err != nil {
		return nil, errors.WithStack(err)
	}
	ephemeralPublic, err := curve25519.X25519(ephemeralPrivate[:], curve25519.Basepoint)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	copy(message[8:40], ephemeralPublic)
	chainKey = kdf1(chainKey[:], ephemeralPublic)
	hashState = mixHash(hashState, ephemeralPublic)

	// Encrypted static key.
	shared, err := curve25519.X25519(ephemeralPrivate[:], k.peer[:])
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var key [32]byte
	chainKey, key = kdf2(chainKey[:], shared)
	aead, err := chacha20poly1305.New(key[:])
	if err != nil {
		return nil, errors.WithStack(err)
	}
	aead.Seal(message[40:40], zeroNonce[:], k.public[:], hashState[:])
	hashState = mixHash(hashState, message[40:88])

	// Encrypted timestamp.
	shared, err = curve25519.X25519(k.private[:], k.peer[:])
	if err != nil {
		return nil, errors.WithStack(err)
	}
	_, key = kdf2(chainKey[:], shared)
	aead, err = chacha20poly1305.New(key[:])
	if err != nil {
		return nil, errors.WithStack(err)
	}
	aead.Seal(message[88:88], zeroNonce[:], timestamp[:], hashState[:])

	// MAC1 over everything before it; MAC2 stays zero (no cookie).
	mac, err := blake2s.New128(k.mac1Key[:])
	if err != nil {
		return nil, errors.WithStack(err)
	}
	mac.Write(message[:116])
	mac.Sum(message[116:116])

	// Like the WARP clients, the reserved bytes are set after the MACs.
	copy(message[1:4], k.reserved[:])
	return message, nil
}

// isAnswer reports whether packet answers the initiation with sender index.
// A cookie reply also proves the server is there (it is under load).
func isAnswer(packet []byte, sender uint32) bool {
	switch {
	case len(packet) == responseSize && packet[0] == typeResponse:
		return binary.LittleEndian.Uint32(packet[8:12]) == sender
	case len(packet) == cookieReplySize && packet[0] == typeCookieReply:
		return binary.LittleEndian.Uint32(packet[4:8]) == sender
	}
	return false
}

func newBlake2s() hash.Hash {
	h, _ := blake2s.New256(nil)
	return h
}

func hmacSum(key []byte, inputs ...[]byte) [32]byte {
	mac := hmac.New(newBlake2s, key)
	for _, input := range inputs {
		mac.Write(input)
	}
	var sum [32]byte
	mac.Sum(sum[:0])
	return sum
}

func mixHash(h [32]byte, data []byte) [32]byte {
	return blake2s.Sum256(append(h[:], data...))
}

func kdf1(key, input []byte) [32]byte {
	prk := hmacSum(key, input)
	return hmacSum(prk[:], []byte{1})
}

func kdf2(key, input []byte) ([32]byte, [32]byte) {
	prk := hmacSum(key, input)
	first := hmacSum(prk[:], []byte{1})
	second := hmacSum(prk[:], first[:], []byte{2})
	return first, second
}

// TAI64N timestamps must increase for every initiation the server sees from
// this key, so they are handed out strictly increasing.
var (
	timestampMutex sync.Mutex
	lastTimestamp  time.Time
)

func nextTimestamp() [12]byte {
	timestampMutex.Lock()
	now := time.Now()
	if !now.After(lastTimestamp) {
		now = lastTimestamp.Add(time.Microsecond)
	}
	lastTimestamp = now
	timestampMutex.Unlock()

	var stamp [12]byte
	binary.BigEndian.PutUint64(stamp[:8], 0x400000000000000a+uint64(now.Unix()))
	binary.BigEndian.PutUint32(stamp[8:], uint32(now.Nanosecond()))
	return stamp
}

func randomIndex() uint32 {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return binary.LittleEndian.Uint32(b[:])
}

// Probe sends one handshake initiation to address and returns the time until
// the server answered.
func (k *Keys) Probe(ctx context.Context, address netip.AddrPort, timeout time.Duration) (time.Duration, error) {
	conn, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(address))
	if err != nil {
		return 0, errors.WithStack(err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()) })
	defer stop()

	sender := randomIndex()
	packet, err := k.initiation(sender, nextTimestamp())
	if err != nil {
		return 0, err
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return 0, errors.WithStack(err)
	}
	stopwatch := startStopwatch()
	if _, err := conn.Write(packet); err != nil {
		return 0, errors.WithStack(err)
	}
	buffer := make([]byte, 256)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if ctx.Err() != nil {
				return 0, ctx.Err()
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				return 0, ErrNoResponse
			}
			return 0, errors.WithStack(err)
		}
		if isAnswer(buffer[:n], sender) {
			return stopwatch.elapsed(), nil
		}
	}
}
