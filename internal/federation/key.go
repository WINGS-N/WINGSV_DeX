package federation

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"os"
	"sync"
)

// receiptDomain is the signature domain, byte for byte as the server builds it. The trailing
// NUL separates a receipt from anything else this key ever signs, so one can never be
// presented in place of another.
var receiptDomain = []byte("wingsv-fed-receipt-v1\x00")

// receiptKey is the Ed25519 key a device signs its usage receipts with. The node never holds
// it, so it cannot forge a receipt for traffic it did not serve. The seed is generated once
// and persisted; the public half goes to the panel, the private half never leaves the device.
type receiptKey struct {
	mu   sync.Mutex
	path string
	priv ed25519.PrivateKey
}

func newReceiptKey(path string) *receiptKey {
	return &receiptKey{path: path}
}

// private returns the signing key, generating and persisting a fresh seed on first use.
func (k *receiptKey) private() (ed25519.PrivateKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.priv != nil {
		return k.priv, nil
	}
	if raw, err := os.ReadFile(k.path); err == nil {
		if seed, err := base64.StdEncoding.DecodeString(string(raw)); err == nil && len(seed) == ed25519.SeedSize {
			k.priv = ed25519.NewKeyFromSeed(seed)
			return k.priv, nil
		}
	}
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}
	if err := os.WriteFile(k.path, []byte(base64.StdEncoding.EncodeToString(seed)), 0o600); err != nil {
		return nil, err
	}
	k.priv = ed25519.NewKeyFromSeed(seed)
	return k.priv, nil
}

// publicKey returns the base64 public half to hand the panel.
func (k *receiptKey) publicKey() (string, error) {
	priv, err := k.private()
	if err != nil {
		return "", err
	}
	pub := priv.Public().(ed25519.PublicKey)
	return base64.StdEncoding.EncodeToString(pub), nil
}

// forget drops the key: after signing out it is nobody's.
func (k *receiptKey) forget() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.priv = nil
	_ = os.Remove(k.path)
}

// sign builds the signed message exactly as the server does - fixed-width fields prefixed by
// their length, all numbers big-endian - and returns the base64 signature. A non-canonical
// encoder (protobuf, JSON) would emit different bytes for the same receipt and the signature
// would not verify, so the layout here is hand-rolled and must match the server's.
func (k *receiptKey) sign(r Receipt) (string, error) {
	priv, err := k.private()
	if err != nil {
		return "", err
	}
	var buf []byte
	buf = append(buf, receiptDomain...)
	buf = appendField(buf, r.ClientID)
	buf = appendField(buf, r.NodeID)
	buf = appendField(buf, r.Transport)
	buf = appendField(buf, r.Nonce)
	buf = appendNumber(buf, r.WindowStartUnix)
	buf = appendNumber(buf, r.WindowEndUnix)
	buf = appendNumber(buf, r.PayloadUpBytes)
	buf = appendNumber(buf, r.PayloadDown)
	return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, buf)), nil
}

func appendField(buf []byte, value string) []byte {
	raw := []byte(value)
	buf = appendNumber(buf, int64(len(raw)))
	return append(buf, raw...)
}

func appendNumber(buf []byte, value int64) []byte {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(value))
	return append(buf, n[:]...)
}
