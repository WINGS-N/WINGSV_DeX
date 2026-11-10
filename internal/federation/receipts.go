package federation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// receiptWindow is how often a window closes. More often is pointless: the deltas pile
	// up on their own.
	receiptWindow = 5 * time.Minute
	// receiptFloor is the smallest window worth signing. Below it a receipt is noise.
	receiptFloor = 64 * 1024
	// receiptsOffWindow is how long a 410 from a node suppresses signing. An hour later the
	// client tries once more in case the operator turned payouts back on; one rejected
	// request an hour is nothing, and the flag never sticks forever.
	receiptsOffWindow = time.Hour
	// receiptQueueMax caps the stored queue; past it the oldest are dropped.
	receiptQueueMax = 1024
	// receiptQueueMaxAge is how long an unsent receipt is kept. The head will not take one
	// older than this.
	receiptQueueMaxAge = 7 * 24 * time.Hour
)

// ReceiptAccount is the current signed-in account the engine signs and sends receipts for.
// The engine reads it afresh each window so a sign-out stops receipts at once.
type ReceiptAccount struct {
	Client   *Client
	Token    string
	ClientID string // the account's federation subject id
}

// ReceiptEngine collects, signs, queues and delivers federation usage receipts. It is
// transport-agnostic: the data plane feeds it already-read counters through Collect, and
// supplies the account and the node-door routing through the callbacks.
type ReceiptEngine struct {
	dir     string
	key     *receiptKey
	account func() (ReceiptAccount, bool)
	door    NodeDoorRoute
	keySent bool

	mu      sync.Mutex
	flushMu sync.Mutex
	log     func(string)
	lastS   string
}

// NewReceiptEngine builds an engine persisting its state (signing key, window mark, queue,
// off flag) under dir. door routes the node-door fallback; logf records one line per state
// change (may be nil). Wire the account provider with SetAccount before the engine can sign.
func NewReceiptEngine(dir string, door NodeDoorRoute, logf func(string)) *ReceiptEngine {
	_ = os.MkdirAll(dir, 0o700)
	return &ReceiptEngine{
		dir:  dir,
		key:  newReceiptKey(filepath.Join(dir, "signing.key")),
		door: door,
		log:  logf,
	}
}

// SetAccount wires the current-account provider: it returns the signed-in account the engine
// signs and sends for, or false when signed out. Called afresh each window so a sign-out
// stops receipts at once.
func (e *ReceiptEngine) SetAccount(account func() (ReceiptAccount, bool)) {
	e.mu.Lock()
	e.account = account
	e.mu.Unlock()
}

// currentAccount reads the provider, tolerating an unset one.
func (e *ReceiptEngine) currentAccount() (ReceiptAccount, bool) {
	if e.account == nil {
		return ReceiptAccount{}, false
	}
	return e.account()
}

func (e *ReceiptEngine) note(reason string) {
	if reason == e.lastS {
		return
	}
	e.lastS = reason
	if reason != "" && e.log != nil {
		e.log(reason)
	}
}

// Collect closes a window against already-read total counters and, when there is enough to
// sign, queues a signed receipt and flushes the queue. nodeID is who carried the traffic
// (the server address) and transport is "xray" or "vktp". It is safe to call several times a
// second: a window that has not elapsed returns at once.
func (e *ReceiptEngine) Collect(nodeID, transport string, uplinkTotal, downlinkTotal int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isOff() {
		e.note("receipts: node runs without payouts, receipts not needed")
		return
	}
	acc, ok := e.currentAccount()
	if !ok || acc.ClientID == "" || nodeID == "" {
		e.note("receipts: no ids, nothing to sign")
		return
	}
	mark := e.readMark()
	now := time.Now()
	if mark.At > 0 && now.Sub(time.UnixMilli(mark.At)) < receiptWindow {
		return // window still open
	}
	// A counter the kernel does not have yet comes back negative, not zero. Taking that for
	// a reset would re-sign the whole history.
	if uplinkTotal < 0 || downlinkTotal < 0 {
		e.note("receipts: counters unavailable")
		return
	}
	// First valid read of this session only seeds the baseline: without a prior mark the
	// delta would be the whole cumulative total over a zero-length window, which is both an
	// over-claim and a malformed receipt.
	if mark.At == 0 {
		e.writeMark(receiptMark{Up: uplinkTotal, Down: downlinkTotal, At: now.UnixMilli()})
		return
	}
	upDelta := uplinkTotal - mark.Up
	downDelta := downlinkTotal - mark.Down
	// The engine restarted and the counters went back to zero: the delta means nothing, so
	// rebase and wait for the next window.
	if upDelta < 0 || downDelta < 0 {
		e.writeMark(receiptMark{Up: uplinkTotal, Down: downlinkTotal, At: now.UnixMilli()})
		e.note("receipts: counters restarted, rebased")
		return
	}
	start := now
	if mark.At > 0 {
		start = time.UnixMilli(mark.At)
	}
	e.writeMark(receiptMark{Up: uplinkTotal, Down: downlinkTotal, At: now.UnixMilli()})
	if upDelta+downDelta < receiptFloor {
		return // below the floor
	}

	receipt := Receipt{
		ClientID:        acc.ClientID,
		NodeID:          nodeID,
		Transport:       transport,
		WindowStartUnix: start.Unix(),
		WindowEndUnix:   now.Unix(),
		PayloadUpBytes:  upDelta,
		PayloadDown:     downDelta,
		Nonce:           newNonce(),
	}
	sig, err := e.key.sign(receipt)
	if err != nil {
		e.note("receipts: no signing key, cannot sign")
		return
	}
	receipt.Signature = sig
	e.note("")
	if e.log != nil {
		e.log(fmt.Sprintf("receipt signed for %s, up %d B, down %d B", nodeID, upDelta, downDelta))
	}
	e.enqueue(receipt)
	go e.Flush()
}

// Reset forgets the window mark, the queue and the signing key: after a sign-out they are
// nobody's. The off flag is cleared too so the next account starts clean.
func (e *ReceiptEngine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.key.forget()
	e.keySent = false
	_ = os.Remove(filepath.Join(e.dir, "mark.json"))
	_ = os.Remove(filepath.Join(e.dir, "queue.json"))
	_ = os.Remove(filepath.Join(e.dir, "off"))
}

// ---- window mark ----

type receiptMark struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
	At   int64 `json:"at"` // unix millis
}

func (e *ReceiptEngine) readMark() receiptMark {
	var m receiptMark
	if raw, err := os.ReadFile(filepath.Join(e.dir, "mark.json")); err == nil {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

func (e *ReceiptEngine) writeMark(m receiptMark) {
	if raw, err := json.Marshal(m); err == nil {
		_ = os.WriteFile(filepath.Join(e.dir, "mark.json"), raw, 0o600)
	}
}

// ---- receipts-off flag ----

func (e *ReceiptEngine) markOff() {
	_ = os.WriteFile(filepath.Join(e.dir, "off"), []byte(fmt.Sprintf("%d", time.Now().UnixMilli())), 0o600)
}

func (e *ReceiptEngine) isOff() bool {
	raw, err := os.ReadFile(filepath.Join(e.dir, "off"))
	if err != nil {
		return false
	}
	var at int64
	if _, err := fmt.Sscanf(string(raw), "%d", &at); err != nil {
		return false
	}
	return at > 0 && time.Since(time.UnixMilli(at)) < receiptsOffWindow
}

func newNonce() string {
	raw := make([]byte, 12)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}

// errPanelUnreachable marks a panel send that failed outright, so Flush falls back to the
// node door rather than treating the queue as delivered.
var errPanelUnreachable = errors.New("panel unreachable")

// sendViaPanel pushes the queue to the panel, registering the signing key once first.
func (e *ReceiptEngine) sendViaPanel(acc ReceiptAccount, queue []Receipt) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if !e.keySent {
		pub, err := e.key.publicKey()
		if err != nil {
			return 0, err
		}
		if err := acc.Client.SendKey(ctx, acc.Token, pub); err != nil {
			return 0, errPanelUnreachable
		}
		e.keySent = true
	}
	accepted, err := acc.Client.SendReceipts(ctx, acc.Token, queue, "")
	if err != nil {
		return 0, errPanelUnreachable
	}
	return accepted, nil
}
