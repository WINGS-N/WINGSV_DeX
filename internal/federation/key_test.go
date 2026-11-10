package federation

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// fixedSeedKey writes a known seed so the key is deterministic across the test.
func fixedSeedKey(t *testing.T) *receiptKey {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	path := filepath.Join(t.TempDir(), "signing.key")
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(seed)), 0o600); err != nil {
		t.Fatal(err)
	}
	return newReceiptKey(path)
}

// expectedMessage rebuilds the signed byte layout independently of sign(), so the test fails
// if the field order, the length framing or the number encoding ever drifts from the server's.
func expectedMessage(r Receipt) []byte {
	var buf []byte
	buf = append(buf, []byte("wingsv-fed-receipt-v1\x00")...)
	field := func(s string) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(s)))
		buf = append(buf, n[:]...)
		buf = append(buf, s...)
	}
	num := func(v int64) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(v))
		buf = append(buf, n[:]...)
	}
	field(r.ClientID)
	field(r.NodeID)
	field(r.Transport)
	field(r.Nonce)
	num(r.WindowStartUnix)
	num(r.WindowEndUnix)
	num(r.PayloadUpBytes)
	num(r.PayloadDown)
	return buf
}

func TestReceiptSignatureLayout(t *testing.T) {
	key := fixedSeedKey(t)
	r := Receipt{
		ClientID:        "client-abc",
		NodeID:          "203.0.113.7",
		Transport:       "vktp",
		Nonce:           "0011223344556677889900aa",
		WindowStartUnix: 1_700_000_000,
		WindowEndUnix:   1_700_000_300,
		PayloadUpBytes:  123456,
		PayloadDown:     7891011,
	}
	sig, err := key.sign(r)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		t.Fatalf("signature is not base64: %v", err)
	}

	pubB64, err := key.publicKey()
	if err != nil {
		t.Fatalf("publicKey: %v", err)
	}
	pub, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil {
		t.Fatalf("public key is not base64: %v", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Fatalf("public key size = %d, want %d", len(pub), ed25519.PublicKeySize)
	}
	if !ed25519.Verify(pub, expectedMessage(r), raw) {
		t.Fatal("signature does not verify against the independently built message")
	}
}

func TestReceiptSignatureDeterministic(t *testing.T) {
	key := fixedSeedKey(t)
	r := Receipt{ClientID: "c", NodeID: "n", Transport: "xray", Nonce: "ab", PayloadUpBytes: 1, PayloadDown: 2}
	a, err := key.sign(r)
	if err != nil {
		t.Fatal(err)
	}
	b, err := key.sign(r)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("ed25519 signatures should be deterministic for the same message")
	}
}
